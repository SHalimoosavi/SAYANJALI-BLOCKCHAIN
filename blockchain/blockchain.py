"""
Blockchain orchestrator for SAYANJALI BLOCKCHAIN.

The `Blockchain` class is the single entry point application code (API,
CLI) should use. It owns the in-memory chain tip cache, delegates
persistence to `Storage`, delegates proof rules to a `ConsensusEngine`,
and delegates mining to `Miner`. This keeps every other module free of
cross-cutting orchestration logic.
"""

from __future__ import annotations

import threading
from typing import Optional

from blockchain.block import Block
from blockchain.consensus import ConsensusEngine, get_consensus_engine
from blockchain.mempool import Mempool
from blockchain.mining import Miner
from blockchain.storage import Storage
from blockchain.transaction import Transaction
from blockchain.utils import ValidationError, get_logger
from blockchain.validators import (
    chain_work,
    validate_block_against_chain,
    validate_chain,
    validate_genesis_identity,
    validate_transaction,
)
from config.settings import Settings, get_settings

logger = get_logger("blockchain.blockchain")


class Blockchain:
    """
    Top-level facade coordinating chain state, consensus, storage, and
    the mempool for a single SAYANJALI BLOCKCHAIN node.
    """

    def __init__(self, settings: Optional[Settings] = None) -> None:
        self.settings = settings or get_settings()
        self.storage = Storage(self.settings.database)
        self.consensus: ConsensusEngine = get_consensus_engine(
            self.settings.consensus.algorithm
        )
        self.mempool = Mempool()
        self.miner = Miner(self.consensus, self.settings.mining.block_reward)

        # Phase 6.5: guards every section that mutates `self.chain` and its
        # corresponding storage/mempool state. FastAPI runs synchronous
        # route handlers (including the P2P propagation endpoints) in a
        # threadpool, so concurrent requests can genuinely execute against
        # this same Blockchain instance on separate threads. A plain
        # `threading.Lock` (not `asyncio.Lock`) is the correct primitive
        # here since these are real OS threads, not just async tasks --
        # and it is the smallest change that closes the race without any
        # broader asyncio redesign.
        self.mutation_lock = threading.Lock()

        self.chain: list[Block] = self.storage.load_chain()
        if not self.chain:
            self._create_and_persist_genesis_block()

    # ------------------------------------------------------------------ #
    # Initialization
    # ------------------------------------------------------------------ #

    def _create_and_persist_genesis_block(self) -> None:
        """Create the genesis block from config and persist it."""
        genesis_cfg = self.settings.genesis
        genesis_block = Block.genesis(
            previous_hash=genesis_cfg.previous_hash,
            timestamp=genesis_cfg.timestamp,
            nonce=genesis_cfg.nonce,
            message=genesis_cfg.message,
        )
        self.storage.save_block(genesis_block)
        self.storage.update_balances_for_block(genesis_block)
        self.chain = [genesis_block]
        logger.info("Genesis block created: %s", genesis_block.hash)

    # ------------------------------------------------------------------ #
    # Chain accessors
    # ------------------------------------------------------------------ #

    @property
    def latest_block(self) -> Block:
        """Return the current chain tip."""
        return self.chain[-1]

    @property
    def length(self) -> int:
        """Return the number of blocks in the chain, including genesis."""
        return len(self.chain)

    @property
    def genesis_block(self) -> Block:
        """Return this node's genesis block."""
        return self.chain[0]

    def total_work(self) -> int:
        """Return the accumulated proof-of-work of the local chain."""
        return chain_work(self.chain)

    def get_block(self, index: int) -> Optional[Block]:
        """Return the block at `index`, or None if out of range."""
        if 0 <= index < len(self.chain):
            return self.chain[index]
        return None

    def current_difficulty(self) -> int:
        """
        Compute the difficulty the next block must satisfy, using the
        consensus engine's retargeting logic over the recent block window.
        """
        window = self.settings.consensus.difficulty_adjustment_interval
        recent = self.chain[-window:] if len(self.chain) >= window else self.chain
        return self.consensus.next_difficulty(
            recent,
            self.settings.consensus.difficulty,
            self.settings.consensus.target_block_time_seconds,
        )

    # ------------------------------------------------------------------ #
    # Transactions
    # ------------------------------------------------------------------ #

    def submit_transaction(self, transaction: Transaction) -> tuple[bool, str]:
        """
        Validate and add a transaction to the mempool.

        Returns:
            (accepted, reason) -- reason explains rejection when accepted
            is False.
        """
        is_valid, reason = validate_transaction(transaction)
        if not is_valid:
            return False, reason

        balance = self.get_balance(transaction.sender)
        if not transaction.is_coinbase() and balance < transaction.amount:
            return False, (
                f"Insufficient balance: address {transaction.sender} has "
                f"{balance}, needs {transaction.amount}."
            )

        added = self.mempool.add_transaction(transaction)
        if not added:
            return False, "Transaction rejected by mempool (duplicate or invalid)."
        return True, ""

    def get_balance(self, address: str) -> float:
        """Return the current confirmed balance for `address`."""
        return self.storage.get_balance(address)

    def get_pending_transactions(self) -> list[Transaction]:
        """Return all transactions currently waiting in the mempool."""
        return self.mempool.get_pending()

    # ------------------------------------------------------------------ #
    # Mining
    # ------------------------------------------------------------------ #

    def mine_pending_transactions(self, miner_address: str) -> Block:
        """
        Mine a new block containing pending mempool transactions plus a
        coinbase reward, append it to the chain, and persist it.

        Returns:
            The newly mined and appended Block.
        """
        difficulty = self.current_difficulty()
        new_block, included_hashes = self.miner.mine_block(
            index=self.latest_block.index + 1,
            previous_hash=self.latest_block.hash,
            mempool=self.mempool,
            miner_address=miner_address,
            difficulty=difficulty,
        )

        # Proof-of-work search happens outside the lock (it can be slow
        # and doesn't touch shared state); the actual chain mutation is
        # guarded, with validation re-run under the lock since the chain
        # tip this block was mined against may have moved while mining
        # was in progress (e.g. a concurrently-received propagated block).
        with self.mutation_lock:
            is_valid, reason = validate_block_against_chain(
                new_block, self.latest_block, 1, self.consensus
            )
            if not is_valid:
                raise ValidationError(f"Newly mined block failed validation: {reason}")

            self.chain.append(new_block)
            self.storage.save_block(new_block)
            self.storage.update_balances_for_block(new_block)
            self.mempool.remove_transactions(included_hashes)

        logger.info(
            "Block %s appended to chain (hash=%s, tx_count=%s)",
            new_block.index,
            new_block.hash,
            len(new_block.transactions),
        )
        return new_block

    # ------------------------------------------------------------------ #
    # Validation
    # ------------------------------------------------------------------ #

    def is_chain_valid(self) -> tuple[bool, str]:
        """
        Validate the entire in-memory chain from genesis to tip.

        Uses a difficulty floor of 1 (any real proof-of-work) rather than
        the live configured difficulty, since historical blocks may have
        been mined at a lower difficulty before a retarget increased it.
        """
        return validate_chain(self.chain, 1, self.consensus)

    def replace_chain(self, candidate_chain: list[Block]) -> tuple[bool, str]:
        """
        Adopt `candidate_chain` in place of the local chain if it is valid,
        belongs to this network, and represents strictly more accumulated
        proof-of-work than the local chain.

        Accumulated work -- not raw block count -- is the correct
        criterion for a PoW chain: a shorter chain mined at higher
        difficulty can represent more real computational effort than a
        longer one mined at lower difficulty. Using length alone would let
        a chain of many trivially-easy blocks outrank a chain that was
        genuinely harder to produce.

        This is the hook Phase 2's P2P synchronization calls when it
        receives a competing chain from a peer. It safely handles both the
        simple case (candidate extends the local tip) and a true reorg
        (candidate diverges below the local tip), persisting the result
        atomically via `Storage.reorganize_from`.

        The entire operation runs under `mutation_lock`: unlike mining
        (where only the PoW search is safely lockless), every step here
        reads and then acts on `self.chain`/`self.total_work()`, so
        holding the lock for the full call is what keeps the
        read-then-mutate sequence atomic against concurrent propagation
        or mining on other threads. Chain replacement is comparatively
        rare, so the throughput cost of the coarser lock here is
        acceptable in exchange for that simplicity and correctness.
        """
        with self.mutation_lock:
            genesis_ok, reason = validate_genesis_identity(candidate_chain, self.genesis_block)
            if not genesis_ok:
                return False, reason

            candidate_work = chain_work(candidate_chain)
            local_work = self.total_work()
            if candidate_work <= local_work:
                return False, (
                    f"Candidate chain work ({candidate_work}) does not exceed "
                    f"local chain work ({local_work})."
                )

            is_valid, reason = validate_chain(candidate_chain, 1, self.consensus)
            if not is_valid:
                return False, f"Candidate chain invalid: {reason}"

            fork_index = self._find_fork_index(candidate_chain)
            new_blocks = candidate_chain[fork_index:]

            self.storage.reorganize_from(fork_index, new_blocks)
            self.chain = candidate_chain

            # Any transaction now confirmed on the adopted chain should no
            # longer sit in the local mempool as pending.
            confirmed_hashes = [
                tx.tx_hash for block in new_blocks for tx in block.transactions
            ]
            self.mempool.remove_transactions(confirmed_hashes)

            logger.info(
                "Chain replaced: fork_index=%s, new_length=%s, local_work=%s -> %s",
                fork_index,
                len(candidate_chain),
                local_work,
                candidate_work,
            )
        return True, ""

    def _find_fork_index(self, candidate_chain: list[Block]) -> int:
        """
        Return the first index at which `candidate_chain` diverges from
        the local chain (i.e. the first index that must be rewritten).

        If the candidate simply extends the local chain with no shared
        divergence, this returns `len(self.chain)` -- nothing already
        persisted needs to change, only new blocks are appended.
        """
        shared_length = min(len(self.chain), len(candidate_chain))
        for i in range(shared_length):
            if self.chain[i].hash != candidate_chain[i].hash:
                return i
        return shared_length

    def status(self) -> dict:
        """Return a summary dict describing current node/chain status."""
        return {
            "network_name": self.settings.network_name,
            "chain_length": self.length,
            "latest_block_hash": self.latest_block.hash,
            "difficulty": self.current_difficulty(),
            "pending_transactions": self.mempool.size(),
            "consensus_algorithm": self.settings.consensus.algorithm,
        }
