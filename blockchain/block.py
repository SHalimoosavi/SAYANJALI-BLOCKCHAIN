"""
Block model for SAYANJALI BLOCKCHAIN.

A Block bundles a list of transactions together with metadata linking it
to the previous block in the chain. The block's hash is computed over its
header fields (index, previous_hash, timestamp, nonce, merkle_root) so
that mining only needs to re-hash a small, fixed-size header rather than
the full transaction list on every nonce attempt.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

from blockchain.transaction import Transaction
from blockchain.utils import (
    current_timestamp,
    deterministic_json,
    merkle_root,
    sha256,
)


@dataclass
class Block:
    """
    Represents a single block in the SAYANJALI BLOCKCHAIN.

    Attributes:
        index: Position of this block in the chain (genesis = 0).
        previous_hash: Hash of the preceding block.
        timestamp: UNIX timestamp of block creation.
        transactions: List of Transaction objects included in this block.
        nonce: Proof-of-Work nonce found during mining.
        merkle_root: Merkle root of all transaction hashes in this block.
        hash: This block's own hash, computed over its header.
    """

    index: int
    previous_hash: str
    transactions: list[Transaction] = field(default_factory=list)
    timestamp: float = field(default_factory=current_timestamp)
    nonce: int = 0
    difficulty: int = 0
    merkle_root: str = field(default="", init=True)
    hash: str = field(default="", init=True)

    def __post_init__(self) -> None:
        if not self.merkle_root:
            self.merkle_root = self.compute_merkle_root()
        if not self.hash:
            self.hash = self.compute_hash()

    def compute_merkle_root(self) -> str:
        """Compute the Merkle root over this block's transaction hashes."""
        return merkle_root(tx.tx_hash for tx in self.transactions)

    def header_payload(self) -> dict[str, Any]:
        """
        Return the fields covered by the block hash.

        Only header fields are hashed (not the full transaction bodies)
        so that mining -- which repeatedly re-hashes with a changing
        nonce -- stays cheap regardless of how many transactions a block
        contains. Transaction integrity is still guaranteed because the
        merkle_root field is part of the header and changes if any
        transaction changes.
        """
        return {
            "index": self.index,
            "previous_hash": self.previous_hash,
            "timestamp": self.timestamp,
            "nonce": self.nonce,
            "difficulty": self.difficulty,
            "merkle_root": self.merkle_root,
        }

    def compute_hash(self) -> str:
        """Compute this block's hash from its header payload."""
        return sha256(deterministic_json(self.header_payload()))

    def recompute(self) -> None:
        """
        Recompute merkle_root and hash from current field values.

        Call this after mutating `transactions` or `nonce` (for example
        during mining) so that `hash` stays consistent with the block's
        actual content.
        """
        self.merkle_root = self.compute_merkle_root()
        self.hash = self.compute_hash()

    def meets_difficulty(self, difficulty: int) -> bool:
        """Return True if this block's hash has `difficulty` leading zeros."""
        return self.hash.startswith("0" * difficulty)

    def to_dict(self) -> dict[str, Any]:
        """Serialize this block, including its transactions, to a plain dict."""
        return {
            "index": self.index,
            "previous_hash": self.previous_hash,
            "timestamp": self.timestamp,
            "nonce": self.nonce,
            "difficulty": self.difficulty,
            "merkle_root": self.merkle_root,
            "hash": self.hash,
            "transactions": [tx.to_dict() for tx in self.transactions],
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "Block":
        """Reconstruct a Block from a plain dict (e.g. from storage/API)."""
        transactions = [
            Transaction.from_dict(tx) for tx in data.get("transactions", [])
        ]
        return cls(
            index=data["index"],
            previous_hash=data["previous_hash"],
            transactions=transactions,
            timestamp=float(data["timestamp"]),
            nonce=int(data.get("nonce", 0)),
            difficulty=int(data.get("difficulty", 0)),
            merkle_root=data.get("merkle_root", ""),
            hash=data.get("hash", ""),
        )

    @classmethod
    def genesis(
        cls,
        previous_hash: str,
        timestamp: float,
        nonce: int,
        message: str,
    ) -> "Block":
        """
        Construct the deterministic genesis block.

        The genesis block contains a single informational coinbase-style
        transaction carrying `message`, so every node that builds the
        chain from config.settings.GenesisConfig produces byte-identical
        genesis blocks.
        """
        genesis_tx = Transaction(
            sender="SYJ-GENESIS-0000000000000000000000000000",
            receiver="SYJ-GENESIS-0000000000000000000000000000",
            amount=0.0,
            timestamp=timestamp,
        )
        # Encode the message into the transaction hash deterministically by
        # overriding tx_hash with a hash of the message itself.
        genesis_tx.tx_hash = sha256(message)

        block = cls(
            index=0,
            previous_hash=previous_hash,
            transactions=[genesis_tx],
            timestamp=timestamp,
            nonce=nonce,
        )
        return block
