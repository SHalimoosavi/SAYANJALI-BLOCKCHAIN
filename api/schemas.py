"""
Pydantic schemas for SAYANJALI BLOCKCHAIN's REST API.

Separating schemas from the blockchain's internal dataclasses (Block,
Transaction) keeps the API's public contract stable even if internal
representations change, and gives FastAPI automatic request validation
and OpenAPI documentation.
"""

from __future__ import annotations

from decimal import Decimal
from typing import Optional

from pydantic import BaseModel, Field, field_validator


class TransactionOut(BaseModel):
    """Transaction as returned by the API."""

    sender: str
    receiver: str
    amount: str
    amount_base_units: int
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
    difficulty: int
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
    balance: str
    balance_base_units: int


class TransactionCreateRequest(BaseModel):
    """
    Payload to construct an unsigned transaction envelope.

    This endpoint never receives or handles private key material -- it
    exists only to hand back a canonical, timestamped transaction body
    for the client to sign entirely locally (e.g. via
    `blockchain.wallet.Wallet.sign` / `blockchain.transaction.Transaction.sign`,
    the same functions the CLI uses) before submitting the signed result
    to `/transaction/submit`.
    """

    sender: str
    receiver: str
    amount: Decimal = Field(gt=0)

    @field_validator("sender", "receiver")
    @classmethod
    def not_empty(cls, value: str) -> str:
        if not value or not value.strip():
            raise ValueError("Address must not be empty.")
        return value


class TransactionSubmitRequest(BaseModel):
    """Payload to submit a fully-signed transaction to the mempool."""

    sender: str
    receiver: str
    amount: Decimal = Field(gt=0)
    amount_base_units: Optional[int] = Field(default=None, gt=0)
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


# --------------------------------------------------------------------- #
# Phase 2: P2P networking schemas
# --------------------------------------------------------------------- #


class PeerOut(BaseModel):
    """A single known peer, as returned by the API."""

    address: str
    node_id: Optional[str] = None
    status: str
    last_seen: Optional[float] = None
    registered_at: Optional[float] = None
    trusted: bool = False
    failure_count: int = 0
    backoff_until: Optional[float] = None
    capabilities: list[str] = Field(default_factory=list)


class NetworkStatusResponse(BaseModel):
    """Observable, non-sensitive node/network status."""

    node_id: str
    self_address: str
    public_key: str
    peer_count: int
    healthy_peer_count: int = 0
    chain_length: int
    chain_work: int
    network_name: str
    chain_id: int = 1
    lifecycle: str = "STARTING"
    sync_state: str = "STARTING"
    mempool_size: int = 0
    current_difficulty: int = 0
    total_supply_base_units: int = 0
    propagation: dict = Field(default_factory=dict)


class PeerListResponse(BaseModel):
    """List of every peer this node currently knows about."""

    count: int
    peers: list[PeerOut]


class PeerRegisterRequest(BaseModel):
    """Payload a peer sends to announce itself to this node."""

    node_id: str
    address: str


class PeerRegisterResponse(BaseModel):
    """
    Response to a peer registration request.

    Doubles as a light peer-discovery mechanism: alongside confirming
    whether the registration was accepted, this node also returns its own
    identity and known peer addresses, so the registering peer can learn
    about the wider network from a single request.
    """

    accepted: bool
    reason: Optional[str] = None
    self_node_id: str
    self_address: str
    known_peers: list[str]


class NetworkChainResponse(BaseModel):
    """
    Full chain response used for peer-to-peer synchronization.

    Deliberately separate from `ChainOut` (the human-facing `/chain`
    endpoint): this response is what `sync.py` parses back into `Block`
    objects, so its `chain` field is a list of raw block dicts (via
    `Block.to_dict()`) rather than the `BlockOut` schema, guaranteeing
    every field consensus validation needs -- including `difficulty` --
    round-trips exactly.
    """

    length: int
    work: int
    chain: list[dict]


class PeerChallengeRequest(BaseModel):
    """Payload requesting a fresh authentication challenge."""

    node_id: str


class PeerChallengeResponse(BaseModel):
    """
    A freshly issued, single-use challenge for `node_id` to sign as proof
    of private-key possession. Expires after a short, server-configured
    window if not used.
    """

    challenge: str
    expires_in_seconds: float


class PeerAuthenticateRequest(BaseModel):
    """
    Payload completing the challenge-response handshake.

    `auth` is the signed handshake envelope (see
    `blockchain.network.handshake.build_handshake_envelope`) -- a dict
    rather than a strictly-typed nested model, since its exact shape is
    owned and validated by the handshake module itself, not duplicated
    here.
    """

    auth: dict


class PeerAuthenticateResponse(BaseModel):
    """Response to a completed (or failed) authentication handshake."""

    authenticated: bool
    reason: Optional[str] = None
    self_node_id: Optional[str] = None
    self_public_key: Optional[str] = None


class BlockReceiveRequest(BaseModel):
    """Payload for a peer propagating a mined block to this node."""

    block: dict
    from_peer: Optional[str] = None
    auth: Optional[dict] = None


class TransactionReceiveRequest(BaseModel):
    """Payload for a peer propagating a mempool transaction to this node."""

    transaction: dict
    from_peer: Optional[str] = None
    auth: Optional[dict] = None


class SyncRequest(BaseModel):
    """Payload requesting this node synchronize from a peer."""

    peer_address: Optional[str] = None
    auth: Optional[dict] = None


class ReceiveResponse(BaseModel):
    """Response to a block or transaction propagation request."""

    accepted: bool
    reason: Optional[str] = None
    rebroadcast_to: int = 0


class SyncResultOut(BaseModel):
    """Outcome of a sync attempt against a single peer."""

    peer_address: str
    accepted: bool
    reason: str
    local_length_before: int
    local_length_after: int


class SyncResponse(BaseModel):
    """Response after attempting synchronization with known peers."""

    results: list[SyncResultOut]
