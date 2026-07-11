"""
Transaction model for SAYANJALI BLOCKCHAIN.

A transaction moves value from a sender address to a receiver address.
Transactions must be signed by the sender's private key before they are
accepted into the mempool or a block (the sole exception being coinbase
transactions, which are minted by the mining process and carry no
signature since they have no sender).
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Optional

from blockchain.utils import (
    ValidationError,
    current_timestamp,
    deterministic_json,
    sha256,
)
from blockchain.wallet import derive_address, is_valid_address, verify_signature

COINBASE_SENDER = "SYJ-COINBASE-0000000000000000000000000000"


@dataclass
class Transaction:
    """
    Represents a single value transfer on SAYANJALI BLOCKCHAIN.

    Attributes:
        sender: Sender wallet address, or COINBASE_SENDER for mining rewards.
        receiver: Receiver wallet address.
        amount: Amount of SYJ token to transfer. Must be positive.
        timestamp: UNIX timestamp of transaction creation.
        sender_public_key: Hex-encoded public key of the sender, required
            to verify the signature. None for coinbase transactions.
        signature: Hex-encoded ECDSA signature over the transaction's
            signing payload. None until signed; None for coinbase.
        tx_hash: SHA-256 hash uniquely identifying this transaction.
    """

    sender: str
    receiver: str
    amount: float
    timestamp: float = field(default_factory=current_timestamp)
    sender_public_key: Optional[str] = None
    signature: Optional[str] = None
    tx_hash: str = field(default="", init=True)

    def __post_init__(self) -> None:
        if not self.tx_hash:
            self.tx_hash = self.compute_hash()

    def _signing_payload(self) -> dict[str, Any]:
        """Fields covered by the digital signature (excludes signature/hash)."""
        return {
            "sender": self.sender,
            "receiver": self.receiver,
            "amount": self.amount,
            "timestamp": self.timestamp,
        }

    def compute_hash(self) -> str:
        """Compute this transaction's content hash (includes signature)."""
        payload = self._signing_payload()
        payload["sender_public_key"] = self.sender_public_key
        payload["signature"] = self.signature
        return sha256(deterministic_json(payload))

    def signing_message(self) -> str:
        """Return the exact deterministic string that must be signed."""
        return deterministic_json(self._signing_payload())

    def sign(self, wallet: "object") -> None:
        """
        Sign this transaction using the given Wallet instance.

        Imported lazily via duck typing (wallet.sign / wallet.public_key_hex)
        to avoid a circular import between transaction.py and wallet.py.
        """
        if self.sender == COINBASE_SENDER:
            raise ValidationError("Coinbase transactions cannot be signed.")
        if wallet.address != self.sender:  # type: ignore[attr-defined]
            raise ValidationError(
                "Wallet address does not match transaction sender."
            )
        self.sender_public_key = wallet.public_key_hex  # type: ignore[attr-defined]
        self.signature = wallet.sign(self.signing_message())  # type: ignore[attr-defined]
        self.tx_hash = self.compute_hash()

    def is_coinbase(self) -> bool:
        """Return True if this is a mining-reward (coinbase) transaction."""
        return self.sender == COINBASE_SENDER

    def verify(self) -> bool:
        """
        Verify this transaction's structural validity and signature.

        Returns:
            True if the transaction is well-formed and (for non-coinbase
            transactions) correctly signed by the claimed sender.
        """
        if self.amount <= 0:
            return False
        if not is_valid_address(self.receiver):
            return False

        # Detect tampering with any field after the hash was originally
        # computed (e.g. an amount changed in-place post-signing). This
        # matters especially for coinbase transactions, which carry no
        # signature and would otherwise have no integrity check at all.
        if self.tx_hash != self.compute_hash():
            return False

        if self.is_coinbase():
            # Coinbase transactions have no signature to check beyond the
            # hash-integrity check above.
            return True

        if not is_valid_address(self.sender):
            return False
        if not self.sender_public_key or not self.signature:
            return False
        if derive_address(self.sender_public_key) != self.sender:
            return False

        return verify_signature(
            self.sender_public_key, self.signing_message(), self.signature
        )

    def to_dict(self) -> dict[str, Any]:
        """Serialize this transaction to a plain dict."""
        return {
            "sender": self.sender,
            "receiver": self.receiver,
            "amount": self.amount,
            "timestamp": self.timestamp,
            "sender_public_key": self.sender_public_key,
            "signature": self.signature,
            "tx_hash": self.tx_hash,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "Transaction":
        """Reconstruct a Transaction from a plain dict (e.g. from storage/API)."""
        return cls(
            sender=data["sender"],
            receiver=data["receiver"],
            amount=float(data["amount"]),
            timestamp=float(data["timestamp"]),
            sender_public_key=data.get("sender_public_key"),
            signature=data.get("signature"),
            tx_hash=data.get("tx_hash", ""),
        )

    @classmethod
    def new_coinbase(cls, receiver: str, amount: float) -> "Transaction":
        """Construct a coinbase (mining reward) transaction."""
        return cls(sender=COINBASE_SENDER, receiver=receiver, amount=amount)
