"""
Blockchain orchestrator for SAYANJALI BLOCKCHAIN.

The `Blockchain` class is the single entry point application code (API,
CLI) should use. It owns the in-memory chain tip cache, delegates
persistence to `Storage`, delegates proof rules to a `ConsensusEngine`,
and delegates mining to `Miner`. This keeps every other module free of
cross-cutting orchestration logic.
"""

from __future__ import annotations

from typing import Optional

from blockchain.block import Block
from blockchain.consensus import ConsensusEngine, get_consensus_engine
from blockchain.mempool import Mempool
from blockchain.mining import Miner
from blockchain.storage import Storage
from blockchain.transaction import Transaction
from blockchain.utils import ValidationError, get_logger
from blockchain.validators import (
    validate_block_against_chain,
    validate_chain,
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
        Replace the current chain with `candidate_chain` if it is both
        valid and longer (the standard longest-valid-chain rule).

        This is the hook future P2P networking code will call when it
        receives a competing chain from a peer.
        """
        if len(candidate_chain) <= len(self.chain):
            return False, "Candidate chain is not longer than current chain."

        is_valid, reason = validate_chain(candidate_chain, 1, self.consensus)
        if not is_valid:
            return False, f"Candidate chain invalid: {reason}"

        for block in candidate_chain:
            if self.storage.get_block(block.index) is None:
                self.storage.save_block(block)
                self.storage.update_balances_for_block(block)

        self.chain = candidate_chain
        logger.info("Chain replaced with longer valid candidate (length=%s)", len(candidate_chain))
        return True, ""

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
