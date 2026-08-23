"""
P2P networking API routes for SAYANJALI BLOCKCHAIN (Phase 6.5).

Every endpoint here is a thin HTTP wrapper around
`blockchain.network.*` -- no chain, consensus, validation, or
authentication logic lives in this file; it is all delegated to
`blockchain.network.handshake`, `sync`, and `propagation`.

--------------------------------------------------------------------
Trust boundary (Phase 6.5)
--------------------------------------------------------------------

PUBLIC / READ-ONLY (no authentication required):
    GET  /network/status               This node's networking summary
    GET  /network/peers                 Known peers (discovery info only)
    POST /network/peers/register        A peer announces itself for
                                         discovery -- this NEVER grants
                                         trust by itself.
    POST /network/peers/challenge       Issue a fresh authentication
                                         challenge for a claimed node_id.
    GET  /network/chain                 Full chain (size-limited) -- read
                                         only, does not mutate state.

AUTHENTICATED (a valid, freshly-signed auth envelope is required; see
`blockchain.network.handshake`):
    POST /network/peers/authenticate    Complete the challenge-response
                                         handshake, becoming a trusted peer.
    POST /network/blocks/receive        Propagate a mined block.
    POST /network/transactions/receive  Propagate a mempool transaction.
    POST /network/sync                  Trigger this node to sync from a
                                         peer. The target, if specified,
                                         must already be a known peer in
                                         this node's own registry -- an
                                         authenticated caller cannot direct
                                         this node to make outbound
                                         requests to an arbitrary
                                         third-party address (closing the
                                         SSRF-via-authenticated-caller
                                         vector on top of the address
                                         allowlist itself).

These endpoints are served on the same FastAPI application and port as
the rest of the REST API, under the `/network` prefix, rather than a
separate listener -- see `config.settings.P2PConfig` for the rationale.

Request body size limits are enforced globally by
`api.middleware.MaxBodySizeMiddleware` (wired in `api/main.py`), not
per-route here -- see that module for why a route-level check alone is
insufficient.
"""

from __future__ import annotations

from typing import Optional

from fastapi import APIRouter, HTTPException, Request

from api.routes import get_blockchain
from api.schemas import (
    BlockReceiveRequest,
    NetworkChainResponse,
    NetworkStatusResponse,
    PeerAuthenticateRequest,
    PeerAuthenticateResponse,
    PeerChallengeRequest,
    PeerChallengeResponse,
    PeerListResponse,
    PeerOut,
    PeerRegisterRequest,
    PeerRegisterResponse,
    ReceiveResponse,
    SyncRequest,
    SyncResponse,
    SyncResultOut,
    TransactionReceiveRequest,
)
from blockchain.network import propagation, sync
from blockchain.network.address_security import validate_peer_address
from blockchain.network.handshake import (
    ChallengeStore,
    build_handshake_envelope,
    verify_handshake_envelope,
)
from blockchain.network.node import NetworkNode
from blockchain.network.ratelimit import PeerRateLimiter
from blockchain.utils import get_logger
from config.settings import get_settings

logger = get_logger("api.network_routes")
router = APIRouter(prefix="/network", tags=["network"])

_network_node: Optional[NetworkNode] = None
_challenge_store: Optional[ChallengeStore] = None
_rate_limiters: dict[str, PeerRateLimiter] = {}

# Rate-limit tiers, expressed as multipliers of the single configured base
# rate (SYJ_PEER_RATE_LIMIT_REQUESTS / SYJ_PEER_RATE_LIMIT_WINDOW) rather
# than as separate environment variables each -- keeping the configuration
# surface small (per Phase 6.5 decision #17) while still giving cheap
# reads a materially more generous budget than expensive operations.
_TIER_MULTIPLIERS = {
    "cheap": 4.0,      # GET /status, GET /peers
    "moderate": 2.0,   # register, challenge, authenticate
    "expensive": 1.0,  # GET /chain, /sync, /blocks/receive, /transactions/receive
}


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


def _get_challenge_store() -> ChallengeStore:
    global _challenge_store
    if _challenge_store is None:
        settings = get_settings()
        _challenge_store = ChallengeStore(
            max_size=settings.p2p.replay_cache_size, ttl_seconds=60.0
        )
    return _challenge_store


def _get_rate_limiter(tier: str) -> PeerRateLimiter:
    if tier not in _rate_limiters:
        settings = get_settings()
        multiplier = _TIER_MULTIPLIERS[tier]
        _rate_limiters[tier] = PeerRateLimiter(
            max_requests=max(1, int(settings.p2p.rate_limit_requests * multiplier)),
            window_seconds=settings.p2p.rate_limit_window_seconds,
            max_keys=settings.p2p.rate_limit_max_keys,
        )
    return _rate_limiters[tier]


def _enforce_rate_limit(request: Request, tier: str, key_hint: Optional[str]) -> None:
    """
    Apply this endpoint's tiered per-peer rate limit.

    Keyed by the claimed identifier when present (e.g. `from_peer`,
    `node_id`), falling back to the raw connecting client host. This is
    an MVP-level, process-local protection against an obviously abusive
    peer, not a substitute for real network security or a distributed
    rate limiter -- see the README's Security section.
    """
    key = key_hint or (request.client.host if request.client else "unknown")
    if not _get_rate_limiter(tier).allow(key):
        raise HTTPException(status_code=429, detail="Rate limit exceeded for this peer.")


def _reset_module_state_for_tests() -> None:
    """
    Reset every module-level singleton. Test-only helper -- production
    code never calls this; each process is expected to hold exactly one
    NetworkNode/ChallengeStore/rate-limiter set for its lifetime.
    """
    global _network_node, _challenge_store, _rate_limiters
    _network_node = None
    _challenge_store = None
    _rate_limiters = {}


@router.get("/status", response_model=NetworkStatusResponse)
def network_status(request: Request) -> NetworkStatusResponse:
    """Return this node's networking identity and summary status. Public."""
    _enforce_rate_limit(request, "cheap", None)
    node = get_network_node()
    return NetworkStatusResponse(**node.status())


@router.get("/peers", response_model=PeerListResponse)
def list_peers(request: Request) -> PeerListResponse:
    """
    Return every peer this node currently knows about. Public discovery
    information only -- no credential or authentication material is
    exposed here.
    """
    _enforce_rate_limit(request, "cheap", None)
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
                trusted=node.is_trusted_peer(p.address),
            )
            for p in peers
        ],
    )


@router.post("/peers/register", response_model=PeerRegisterResponse)
def register_peer(payload: PeerRegisterRequest, request: Request) -> PeerRegisterResponse:
    """
    Accept a peer's self-announcement for discovery.

    This is intentionally unauthenticated (bootstrap needs to work
    without a prior trust relationship) and intentionally does NOT grant
    trust -- a registered peer cannot propagate blocks/transactions or be
    used as a sync target until it separately completes the
    challenge-response handshake via /network/peers/challenge and
    /network/peers/authenticate. Also returns this node's own identity
    and known peer addresses, so a single registration call gives the
    caller a starting point for wider peer discovery.
    """
    _enforce_rate_limit(request, "moderate", payload.address)

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


@router.post("/peers/challenge", response_model=PeerChallengeResponse)
def issue_challenge(payload: PeerChallengeRequest, request: Request) -> PeerChallengeResponse:
    """
    Issue a fresh, single-use authentication challenge for `node_id`.

    The caller must sign this exact value (via
    `blockchain.network.handshake.build_handshake_envelope`) and submit
    it to /network/peers/authenticate within the challenge's validity
    window to complete the handshake.
    """
    _enforce_rate_limit(request, "moderate", payload.node_id)
    store = _get_challenge_store()
    challenge = store.issue(payload.node_id)
    return PeerChallengeResponse(challenge=challenge, expires_in_seconds=60.0)


@router.post("/peers/authenticate", response_model=PeerAuthenticateResponse)
def authenticate_peer(
    payload: PeerAuthenticateRequest, request: Request
) -> PeerAuthenticateResponse:
    """
    Complete the challenge-response handshake, establishing this peer as
    trusted on success.

    Identity-change detection: if this peer's claimed address previously
    authenticated under a different node_id/public_key, that is logged as
    a security-relevant event and the new identity still must
    independently prove key possession via the challenge -- it is never
    silently trusted merely because the address matches.
    """
    envelope = payload.auth
    node_id_hint = envelope.get("node_id") if isinstance(envelope, dict) else None
    _enforce_rate_limit(request, "moderate", node_id_hint)

    node = get_network_node()
    store = _get_challenge_store()

    is_valid, reason = verify_handshake_envelope(envelope, node.auth_context, store)
    if not is_valid:
        return PeerAuthenticateResponse(authenticated=False, reason=reason)

    address = envelope["advertised_address"]
    address_ok, address_reason = validate_peer_address(
        address, node.settings.p2p.allow_private_peer_addresses
    )
    if not address_ok:
        return PeerAuthenticateResponse(
            authenticated=False, reason=f"Invalid advertised address: {address_reason}"
        )

    existing_credential = node.blockchain.storage.get_peer_credential(address)
    if existing_credential is not None and (
        existing_credential["node_id"] != envelope["node_id"]
        or existing_credential["public_key_hex"] != envelope["public_key"]
    ):
        logger.warning(
            "Peer identity change during authentication for address %s: "
            "stored node_id=%s -> claimed node_id=%s. Proceeding since the "
            "new identity independently proved key possession via the "
            "challenge; prior credential is superseded.",
            address,
            existing_credential["node_id"],
            envelope["node_id"],
        )

    node.blockchain.storage.upsert_peer_credential(
        address=address,
        node_id=envelope["node_id"],
        public_key_hex=envelope["public_key"],
        trusted=True,
    )
    # A successfully authenticated peer is also, trivially, a known
    # (discovery) peer -- register it if it wasn't already.
    node.peers.register(address, envelope["node_id"])
    node.peers.mark_seen(address, "online", envelope["node_id"])

    return PeerAuthenticateResponse(
        authenticated=True,
        self_node_id=node.node_id,
        self_public_key=node.identity.public_key_hex,
    )


@router.get("/chain", response_model=NetworkChainResponse)
def network_chain(request: Request) -> NetworkChainResponse:
    """
    Return the full chain in peer-synchronization form. Public and
    read-only -- does not mutate any state.

    Distinct from the human-facing `GET /chain`: this always includes
    every field consensus validation needs (notably `difficulty`) as raw
    dicts, and includes the chain's accumulated work so a peer can
    cheaply decide whether a full sync is even worth attempting before
    parsing the whole payload.

    Phase 6.5: enforces `SYJ_MAX_SYNC_BYTES` on the outbound response.
    This is a known, documented limitation for very large chains -- a
    chain whose serialized size exceeds the configured cap cannot be
    fetched via this endpoint at all in the current MVP; a paginated or
    incremental sync protocol is future work, not something this
    response silently truncates or corrupts.
    """
    _enforce_rate_limit(request, "expensive", None)
    blockchain = get_blockchain()
    settings = get_settings()

    chain_dicts = [block.to_dict() for block in blockchain.chain]
    response = NetworkChainResponse(
        length=blockchain.length,
        work=blockchain.total_work(),
        chain=chain_dicts,
    )

    estimated_size = len(response.model_dump_json())
    if estimated_size > settings.p2p.max_chain_sync_bytes:
        raise HTTPException(
            status_code=413,
            detail=(
                f"Chain size ({estimated_size} bytes) exceeds the configured "
                f"sync limit of {settings.p2p.max_chain_sync_bytes} bytes. "
                "Full-chain retrieval via this endpoint is not available for "
                "chains this large in the current release."
            ),
        )
    return response


@router.post("/sync", response_model=SyncResponse)
def trigger_sync(payload: SyncRequest, request: Request) -> SyncResponse:
    """
    Trigger synchronization against a specific known peer, or every known
    peer if `peer_address` is omitted. Authenticated: the caller must
    present a valid auth envelope proving it is a peer this node already
    trusts.

    If `peer_address` is supplied, it must already be a peer in this
    node's own registry -- an authenticated caller cannot direct this
    node to make outbound requests to an address of the caller's
    choosing that this node's own operator has not already registered.
    This closes the SSRF-via-authenticated-caller vector on top of the
    address allowlist enforced inside `sync.py` itself.
    """
    node = get_network_node()

    node_id_hint = payload.auth.get("node_id") if isinstance(payload.auth, dict) else None
    claimed_address = (
        payload.auth.get("advertised_address") if isinstance(payload.auth, dict) else None
    )
    _enforce_rate_limit(request, "expensive", node_id_hint)

    is_authenticated, auth_reason = propagation.authenticate_request(
        node, payload.auth, {"peer_address": payload.peer_address}, claimed_address
    )
    if not is_authenticated:
        raise HTTPException(status_code=401, detail=f"Authentication failed: {auth_reason}")

    if payload.peer_address:
        if node.peers.get(payload.peer_address) is None:
            raise HTTPException(
                status_code=403,
                detail="peer_address must already be a known peer of this node.",
            )
        results = [sync.sync_with_peer(node, payload.peer_address)]
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
    """Accept a block propagated by a peer. Authenticated."""
    node_id_hint = payload.auth.get("node_id") if isinstance(payload.auth, dict) else None
    _enforce_rate_limit(request, "expensive", node_id_hint or payload.from_peer)

    node = get_network_node()
    accepted, reason, should_rebroadcast = propagation.receive_block(
        node, payload.block, payload.from_peer, payload.auth
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
    """Accept a transaction propagated by a peer. Authenticated."""
    node_id_hint = payload.auth.get("node_id") if isinstance(payload.auth, dict) else None
    _enforce_rate_limit(request, "expensive", node_id_hint or payload.from_peer)

    node = get_network_node()
    accepted, reason, should_rebroadcast = propagation.receive_transaction(
        node, payload.transaction, payload.from_peer, payload.auth
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
