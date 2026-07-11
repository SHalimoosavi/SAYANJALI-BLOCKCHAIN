"""
Pydantic schemas for SAYANJALI BLOCKCHAIN's REST API.

Separating schemas from the blockchain's internal dataclasses (Block,
Transaction) keeps the API's public contract stable even if internal
representations change, and gives FastAPI automatic request validation
and OpenAPI documentation.
"""

from __future__ import annotations

from typing import Optional

from pydantic import BaseModel, Field, field_validator


class TransactionOut(BaseModel):
    """Transaction as returned by the API."""

    sender: str
    receiver: str
    amount: float
    timestamp: float
    sender_public_key: Optional[str] = None
    signature: Optional[str] = None
    tx_hash: str


class BlockOut(BaseModel):
    """Block as returned by the API."""

    index: int
    previous_hash: str
    timestamp: float
    nonce: int
    merkle_root: str
    hash: str
    transactions: list[TransactionOut]


class ChainOut(BaseModel):
    """Full chain response."""

    length: int
    chain: list[BlockOut]


class WalletCreateResponse(BaseModel):
    """Response returned after generating a new wallet."""

    address: str
    public_key: str
    private_key: str
    warning: str = (
        "Store your private_key securely. It will never be shown again "
        "and cannot be recovered if lost."
    )


class WalletBalanceResponse(BaseModel):
    """Response for a wallet balance lookup."""

    address: str
    balance: float


class TransactionCreateRequest(BaseModel):
    """Payload to construct an unsigned transaction."""

    sender: str
    receiver: str
    amount: float = Field(gt=0)

    @field_validator("sender", "receiver")
    @classmethod
    def not_empty(cls, value: str) -> str:
        if not value or not value.strip():
            raise ValueError("Address must not be empty.")
        return value


class TransactionSignRequest(BaseModel):
    """Payload to sign a previously created transaction."""

    sender: str
    receiver: str
    amount: float = Field(gt=0)
    timestamp: float
    private_key: str


class TransactionSubmitRequest(BaseModel):
    """Payload to submit a fully-signed transaction to the mempool."""

    sender: str
    receiver: str
    amount: float = Field(gt=0)
    timestamp: float
    sender_public_key: str
    signature: str


class TransactionSubmitResponse(BaseModel):
    """Response after submitting a transaction to the mempool."""

    accepted: bool
    tx_hash: Optional[str] = None
    reason: Optional[str] = None


class MineRequest(BaseModel):
    """Payload requesting a new block be mined."""

    miner_address: str


class MineResponse(BaseModel):
    """Response describing the newly mined block."""

    block: BlockOut
    message: str = "Block mined and appended to the chain."


class StatusResponse(BaseModel):
    """Node status summary."""

    network_name: str
    chain_length: int
    latest_block_hash: str
    difficulty: int
    pending_transactions: int
    consensus_algorithm: str


class HealthResponse(BaseModel):
    """Liveness probe response."""

    status: str = "ok"


class ErrorResponse(BaseModel):
    """Standard error envelope."""

    detail: str
