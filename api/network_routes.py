"""
P2P networking API routes for SAYANJALI BLOCKCHAIN (Phase 2).

Every endpoint here is a thin HTTP wrapper around
`blockchain.network.*` -- no chain, consensus, or validation logic lives
in this file. Endpoints:

    GET  /network/status              This node's networking summary
    GET  /network/peers                Known peers
    POST /network/peers/register       A peer announces itself
    POST /network/sync                 Sync against known (or one) peer(s)
    GET  /network/chain                Full chain, for peer synchronization
    POST /network/blocks/receive       A peer propagates a mined block
    POST /network/transactions/receive A peer propagates a mempool transaction

These endpoints are served on the same FastAPI application and port as
the Phase 1 REST API, under the `/network` prefix, rather than a separate
listener -- see `config.settings.P2PConfig` for the rationale.
"""

from __future__ import annotations

from typing import Optional

from fastapi import APIRouter, Body, HTTPException, Request

from api.routes import get_blockchain
from api.schemas import (
    BlockReceiveRequest,
    NetworkChainResponse,
    NetworkStatusResponse,
    PeerListResponse,
    PeerOut,
    PeerRegisterRequest,
    PeerRegisterResponse,
    ReceiveResponse,
    SyncResponse,
    SyncResultOut,
    TransactionReceiveRequest,
)
from blockchain.network import propagation, sync
from blockchain.network.node import NetworkNode
from blockchain.network.ratelimit import PeerRateLimiter
from blockchain.utils import get_logger
from config.settings import get_settings

logger = get_logger("api.network_routes")
router = APIRouter(prefix="/network", tags=["network"])

_network_node: Optional[NetworkNode] = None
_rate_limiter: Optional[PeerRateLimiter] = None


def get_network_node() -> NetworkNode:
    """
    Return the module-level NetworkNode instance, constructing it around
    the shared `Blockchain` singleton (from `api.routes.get_blockchain`)
    if it doesn't exist yet. Kept as a single instance per process, the
    same pattern Phase 1 uses for the `Blockchain` singleton.
    """
    global _network_node
    if _network_node is None:
        _network_node = NetworkNode(get_blockchain())
    return _network_node


def _get_rate_limiter() -> PeerRateLimiter:
    global _rate_limiter
    if _rate_limiter is None:
        settings = get_settings()
        _rate_limiter = PeerRateLimiter(
            max_requests=settings.p2p.rate_limit_requests,
            window_seconds=settings.p2p.rate_limit_window_seconds,
        )
    return _rate_limiter


def _enforce_rate_limit(request: Request, from_peer: Optional[str]) -> None:
    """
    Apply a basic per-peer rate limit to propagation endpoints.

    Keyed by the claimed `from_peer` address when present, falling back
    to the raw connecting client host. This is an MVP-level protection
    against an obviously abusive peer, not a substitute for real network
    security -- see the README's Security section.
    """
    key = from_peer or (request.client.host if request.client else "unknown")
    if not _get_rate_limiter().allow(key):
        raise HTTPException(status_code=429, detail="Rate limit exceeded for this peer.")


def _enforce_max_body_size(request: Request, max_bytes: int) -> None:
    """
    Reject requests whose declared Content-Length exceeds `max_bytes`.

    A practical, cheap guard against obviously oversized payloads. It is
    not a substitute for a streaming size limit enforced at the ASGI
    server level, but is sufficient to reject grossly oversized requests
    for this MVP.
    """
    content_length = request.headers.get("content-length")
    if content_length is not None:
        try:
            if int(content_length) > max_bytes:
                raise HTTPException(
                    status_code=413,
                    detail=f"Request body exceeds the {max_bytes}-byte limit for this endpoint.",
                )
        except ValueError:
            pass  # Malformed Content-Length header; let the body parser handle it.


@router.get("/status", response_model=NetworkStatusResponse)
def network_status() -> NetworkStatusResponse:
    """Return this node's networking identity and summary status."""
    node = get_network_node()
    return NetworkStatusResponse(**node.status())


@router.get("/peers", response_model=PeerListResponse)
def list_peers() -> PeerListResponse:
    """Return every peer this node currently knows about."""
    node = get_network_node()
    peers = node.peers.list_peers()
    return PeerListResponse(
        count=len(peers),
        peers=[
            PeerOut(
                address=p.address,
                node_id=p.node_id,
                status=p.status,
                last_seen=p.last_seen,
                registered_at=p.registered_at,
            )
            for p in peers
        ],
    )


@router.post("/peers/register", response_model=PeerRegisterResponse)
def register_peer(payload: PeerRegisterRequest, request: Request) -> PeerRegisterResponse:
    """
    Accept a peer's self-announcement.

    Also returns this node's own identity and known peer addresses, so a
    single registration call gives the caller a starting point for wider
    peer discovery rather than only confirming the one-way registration.
    """
    _enforce_rate_limit(request, payload.address)

    node = get_network_node()
    accepted, reason = node.peers.register(payload.address, payload.node_id)
    if accepted:
        node.peers.mark_seen(payload.address, "online", payload.node_id)

    return PeerRegisterResponse(
        accepted=accepted,
        reason=reason or None,
        self_node_id=node.node_id,
        self_address=node.self_address,
        known_peers=node.peers.addresses(),
    )


@router.get("/chain", response_model=NetworkChainResponse)
def network_chain() -> NetworkChainResponse:
    """
    Return the full chain in peer-synchronization form.

    Distinct from the human-facing `GET /chain`: this always includes
    every field consensus validation needs (notably `difficulty`) as raw
    dicts, and includes the chain's accumulated work so a peer can
    cheaply decide whether a full sync is even worth attempting before
    parsing the whole payload.
    """
    blockchain = get_blockchain()
    return NetworkChainResponse(
        length=blockchain.length,
        work=blockchain.total_work(),
        chain=[block.to_dict() for block in blockchain.chain],
    )


@router.post("/sync", response_model=SyncResponse)
def trigger_sync(peer_address: Optional[str] = Body(default=None, embed=True)) -> SyncResponse:
    """
    Trigger synchronization against a specific peer, or every known peer
    if `peer_address` is omitted.
    """
    node = get_network_node()
    if peer_address:
        results = [sync.sync_with_peer(node, peer_address)]
    else:
        results = sync.sync_with_all_peers(node)

    return SyncResponse(
        results=[
            SyncResultOut(
                peer_address=r.peer_address,
                accepted=r.accepted,
                reason=r.reason,
                local_length_before=r.local_length_before,
                local_length_after=r.local_length_after,
            )
            for r in results
        ]
    )


@router.post("/blocks/receive", response_model=ReceiveResponse)
def receive_block(payload: BlockReceiveRequest, request: Request) -> ReceiveResponse:
    """Accept a block propagated by a peer."""
    _enforce_rate_limit(request, payload.from_peer)
    settings = get_settings()
    _enforce_max_body_size(request, settings.p2p.max_block_payload_bytes)

    node = get_network_node()
    accepted, reason, should_rebroadcast = propagation.receive_block(
        node, payload.block, payload.from_peer
    )

    rebroadcast_count = 0
    if accepted and should_rebroadcast:
        from blockchain.block import Block

        block = Block.from_dict(payload.block)
        rebroadcast_count = len(
            propagation.broadcast_block(node, block, exclude_address=payload.from_peer)
        )

    return ReceiveResponse(
        accepted=accepted, reason=reason or None, rebroadcast_to=rebroadcast_count
    )


@router.post("/transactions/receive", response_model=ReceiveResponse)
def receive_transaction(
    payload: TransactionReceiveRequest, request: Request
) -> ReceiveResponse:
    """Accept a transaction propagated by a peer."""
    _enforce_rate_limit(request, payload.from_peer)
    settings = get_settings()
    _enforce_max_body_size(request, settings.p2p.max_transaction_payload_bytes)

    node = get_network_node()
    accepted, reason, should_rebroadcast = propagation.receive_transaction(
        node, payload.transaction, payload.from_peer
    )

    rebroadcast_count = 0
    if accepted and should_rebroadcast:
        from blockchain.transaction import Transaction

        transaction = Transaction.from_dict(payload.transaction)
        rebroadcast_count = len(
            propagation.broadcast_transaction(
                node, transaction, exclude_address=payload.from_peer
            )
        )

    return ReceiveResponse(
        accepted=accepted, reason=reason or None, rebroadcast_to=rebroadcast_count
    )
