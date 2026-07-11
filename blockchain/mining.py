"""
Mining orchestration for SAYANJALI BLOCKCHAIN.

Bridges the Mempool, Consensus engine, and Block model: selects pending
transactions, prepends a coinbase reward transaction, mines the block via
the configured consensus engine, and returns the finished block ready to
be appended to the chain.
"""

from __future__ import annotations

from blockchain.block import Block
from blockchain.consensus import ConsensusEngine
from blockchain.mempool import Mempool
from blockchain.transaction import Transaction
from blockchain.utils import get_logger

logger = get_logger("blockchain.mining")

MAX_TRANSACTIONS_PER_BLOCK = 500


class Miner:
    """Coordinates mining a new block from pending mempool transactions."""

    def __init__(self, consensus: ConsensusEngine, block_reward: float) -> None:
        self.consensus = consensus
        self.block_reward = block_reward

    def mine_block(
        self,
        index: int,
        previous_hash: str,
        mempool: Mempool,
        miner_address: str,
        difficulty: int,
        max_transactions: int = MAX_TRANSACTIONS_PER_BLOCK,
    ) -> tuple[Block, list[str]]:
        """
        Assemble and mine a new block.

        Args:
            index: Index the new block will occupy.
            previous_hash: Hash of the current chain tip.
            mempool: Source of pending transactions.
            miner_address: Address to receive the coinbase reward.
            difficulty: Target PoW difficulty.
            max_transactions: Cap on non-coinbase transactions included.

        Returns:
            (mined_block, included_tx_hashes) -- included_tx_hashes excludes
            the coinbase transaction and should be removed from the mempool
            by the caller once the block is accepted onto the chain.
        """
        selected = mempool.get_pending(limit=max_transactions)

        coinbase_tx = Transaction.new_coinbase(miner_address, self.block_reward)
        block_transactions: list[Transaction] = [coinbase_tx, *selected]

        block = Block(
            index=index,
            previous_hash=previous_hash,
            transactions=block_transactions,
        )

        logger.info(
            "Mining block %s with %s transactions (difficulty=%s)",
            index,
            len(block_transactions),
            difficulty,
        )
        mined_block = self.consensus.mine(block, difficulty)
        included_hashes = [tx.tx_hash for tx in selected]
        return mined_block, included_hashes
