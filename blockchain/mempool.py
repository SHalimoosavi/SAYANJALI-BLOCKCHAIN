"""
Mempool (pending transaction pool) for SAYANJALI BLOCKCHAIN.

Holds transactions that have been submitted and validated but not yet
mined into a block. Provides duplicate prevention (by tx_hash), basic
validation delegation, and selection logic used by the mining process.
"""

from __future__ import annotations

from typing import Optional

from blockchain.transaction import Transaction
from blockchain.utils import get_logger

logger = get_logger("blockchain.mempool")


class Mempool:
    """In-memory pool of pending, unconfirmed transactions."""

    def __init__(self) -> None:
        self._transactions: dict[str, Transaction] = {}

    def add_transaction(self, transaction: Transaction) -> bool:
        """
        Validate and add a transaction to the pool.

        Returns:
            True if the transaction was added, False if it was rejected
            (invalid signature/structure or already present).
        """
        if transaction.tx_hash in self._transactions:
            logger.info("Rejected duplicate transaction %s", transaction.tx_hash)
            return False

        if not transaction.verify():
            logger.warning(
                "Rejected invalid transaction %s from %s",
                transaction.tx_hash,
                transaction.sender,
            )
            return False

        self._transactions[transaction.tx_hash] = transaction
        logger.info(
            "Added transaction %s (%s -> %s, amount=%s) to mempool",
            transaction.tx_hash,
            transaction.sender,
            transaction.receiver,
            transaction.amount,
        )
        return True

    def remove_transactions(self, tx_hashes: list[str]) -> None:
        """Remove transactions (typically after they've been mined) by hash."""
        for tx_hash in tx_hashes:
            self._transactions.pop(tx_hash, None)

    def get_transaction(self, tx_hash: str) -> Optional[Transaction]:
        """Look up a single pending transaction by its hash."""
        return self._transactions.get(tx_hash)

    def get_pending(self, limit: Optional[int] = None) -> list[Transaction]:
        """
        Return pending transactions ordered by timestamp (oldest first).

        Args:
            limit: If provided, return at most this many transactions.
                Used by the mining process to cap block size.
        """
        ordered = sorted(self._transactions.values(), key=lambda tx: tx.timestamp)
        return ordered[:limit] if limit is not None else ordered

    def size(self) -> int:
        """Return the number of pending transactions currently held."""
        return len(self._transactions)

    def clear(self) -> None:
        """Remove all pending transactions. Primarily useful for tests."""
        self._transactions.clear()

    def contains(self, tx_hash: str) -> bool:
        """Return True if a transaction with this hash is already pending."""
        return tx_hash in self._transactions
