"""Native SYJ transaction model using integer base units."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Optional

from blockchain.native_asset import (
    AmountInput,
    from_base_units,
    to_base_units,
    validate_base_units,
)
from blockchain.utils import ValidationError, current_timestamp, deterministic_json, sha256
from blockchain.wallet import derive_address, is_valid_address, verify_signature

COINBASE_SENDER = "SYJ-COINBASE-0000000000000000000000000000"


@dataclass
class Transaction:
    """A signed SYJ transfer or protocol coinbase issuance transaction.

    Monetary state is stored exclusively in ``amount_base_units``. The
    constructor accepts a human-readable amount for compatibility with the
    existing API/CLI surface, but normalizes it immediately to an integer.
    """

    sender: str
    receiver: str
    amount_base_units: int
    timestamp: float = field(default_factory=current_timestamp)
    sender_public_key: Optional[str] = None
    signature: Optional[str] = None
    tx_hash: str = field(default="", init=True)

    def __init__(
        self,
        sender: str,
        receiver: str,
        amount: AmountInput | None = None,
        timestamp: float | None = None,
        sender_public_key: Optional[str] = None,
        signature: Optional[str] = None,
        tx_hash: str = "",
        *,
        amount_base_units: int | None = None,
    ) -> None:
        if amount_base_units is not None and amount is not None:
            raise ValidationError("Provide either amount or amount_base_units, not both.")
        if amount_base_units is None:
            if amount is None:
                raise ValidationError("Transaction amount is required.")
            try:
                amount_base_units = to_base_units(amount)
            except ValueError as exc:
                raise ValidationError(str(exc)) from exc
        elif isinstance(amount_base_units, bool) or not isinstance(amount_base_units, int):
            raise ValidationError("SYJ amount_base_units must be an integer.")

        self.sender = sender
        self.receiver = receiver
        self.amount_base_units = amount_base_units
        self.timestamp = current_timestamp() if timestamp is None else timestamp
        self.sender_public_key = sender_public_key
        self.signature = signature
        self.tx_hash = tx_hash or self.compute_hash()

    @property
    def amount(self) -> int:
        """Return the exact protocol amount in indivisible base units."""
        return self.amount_base_units

    @amount.setter
    def amount(self, value: AmountInput) -> None:
        try:
            self.amount_base_units = to_base_units(value)
        except ValueError as exc:
            raise ValidationError(str(exc)) from exc

    @property
    def amount_syj(self):
        """Return the human-readable SYJ amount as Decimal."""
        return from_base_units(self.amount_base_units)

    def _signing_payload(self) -> dict[str, Any]:
        return {
            "sender": self.sender,
            "receiver": self.receiver,
            "amount_base_units": self.amount_base_units,
            "timestamp": self.timestamp,
        }

    def compute_hash(self) -> str:
        payload = self._signing_payload()
        payload["sender_public_key"] = self.sender_public_key
        payload["signature"] = self.signature
        return sha256(deterministic_json(payload))

    def signing_message(self) -> str:
        return deterministic_json(self._signing_payload())

    def sign(self, wallet: "object") -> None:
        if self.sender == COINBASE_SENDER:
            raise ValidationError("Coinbase transactions cannot be signed.")
        if wallet.address != self.sender:  # type: ignore[attr-defined]
            raise ValidationError("Wallet address does not match transaction sender.")
        self.sender_public_key = wallet.public_key_hex  # type: ignore[attr-defined]
        self.signature = wallet.sign(self.signing_message())  # type: ignore[attr-defined]
        self.tx_hash = self.compute_hash()

    def is_coinbase(self) -> bool:
        return self.sender == COINBASE_SENDER

    def verify(self) -> bool:
        if self.amount_base_units <= 0:
            return False
        if not validate_base_units(self.amount_base_units)[0]:
            return False
        if not is_valid_address(self.receiver):
            return False
        if self.tx_hash != self.compute_hash():
            return False
        if self.is_coinbase():
            return self.sender_public_key is None and self.signature is None
        if not is_valid_address(self.sender):
            return False
        if not self.sender_public_key or not self.signature:
            return False
        if derive_address(self.sender_public_key) != self.sender:
            return False
        return verify_signature(self.sender_public_key, self.signing_message(), self.signature)

    def to_dict(self) -> dict[str, Any]:
        return {
            "sender": self.sender,
            "receiver": self.receiver,
            "amount": self.amount_syj.__str__(),
            "amount_base_units": self.amount_base_units,
            "timestamp": self.timestamp,
            "sender_public_key": self.sender_public_key,
            "signature": self.signature,
            "tx_hash": self.tx_hash,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "Transaction":
        if "amount_base_units" in data:
            amount_base_units = data["amount_base_units"]
            if isinstance(amount_base_units, bool) or not isinstance(amount_base_units, int):
                raise ValueError("amount_base_units must be an integer")
            return cls(
                sender=data["sender"], receiver=data["receiver"],
                amount_base_units=amount_base_units,
                timestamp=float(data["timestamp"]),
                sender_public_key=data.get("sender_public_key"),
                signature=data.get("signature"), tx_hash=data.get("tx_hash", ""),
            )
        return cls(
            sender=data["sender"], receiver=data["receiver"],
            amount=data["amount"], timestamp=float(data["timestamp"]),
            sender_public_key=data.get("sender_public_key"),
            signature=data.get("signature"), tx_hash=data.get("tx_hash", ""),
        )

    @classmethod
    def new_coinbase(cls, receiver: str, amount: AmountInput) -> "Transaction":
        """Construct coinbase using a human-readable SYJ amount."""
        return cls(sender=COINBASE_SENDER, receiver=receiver, amount=amount)

    @classmethod
    def new_coinbase_base_units(cls, receiver: str, amount_base_units: int) -> "Transaction":
        """Construct coinbase directly from protocol base units."""
        return cls(
            sender=COINBASE_SENDER, receiver=receiver,
            amount_base_units=amount_base_units,
        )
