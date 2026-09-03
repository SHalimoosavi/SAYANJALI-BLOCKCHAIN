"""
Peer identity and registry for SAYANJALI BLOCKCHAIN's P2P layer.

Peers are persisted (via the existing `Storage` abstraction, not a
separate database) so that a node's known-peer set survives process
restarts and is visible to both the running API server and one-off CLI
invocations against the same database file.
"""

from __future__ import annotations

import time
import threading
from dataclasses import dataclass
from typing import Optional
from urllib.parse import urlparse

from blockchain.storage import Storage
from blockchain.utils import get_logger

logger = get_logger("blockchain.network.peer")

VALID_SCHEMES = {"http", "https"}


@dataclass
class Peer:
    """A single known peer node."""

    address: str
    node_id: Optional[str] = None
    status: str = "unknown"  # "online" | "offline" | "unknown"
    last_seen: Optional[float] = None
    registered_at: Optional[float] = None
    failure_count: int = 0
    backoff_until: Optional[float] = None
    capabilities: tuple[str, ...] = ()

    @classmethod
    def from_row(cls, row: dict) -> "Peer":
        """Construct a Peer from a storage row dict."""
        return cls(
            address=row["address"],
            node_id=row.get("node_id"),
            status=row.get("status", "unknown"),
            last_seen=row.get("last_seen"),
            registered_at=row.get("registered_at"),
            failure_count=int(row.get("failure_count", 0) or 0),
            backoff_until=row.get("backoff_until"),
            capabilities=tuple(row.get("capabilities", ()) or ()),
        )


def normalize_address(address: str) -> str:
    """
    Normalize a peer address to a canonical `scheme://host:port` form
    with no trailing slash, so equivalent addresses written differently
    (trailing slash, mixed case scheme) are recognized as the same peer.
    """
    return address.strip().rstrip("/")


def is_valid_peer_address(address: str) -> bool:
    """
    Validate that `address` is a structurally sound HTTP(S) peer address.

    This is a syntactic check only -- it does not confirm the address is
    reachable. Reachability is established separately (see
    `PeerClient.get_status` and `NetworkNode` health checks).
    """
    if not address or not isinstance(address, str):
        return False
    if len(address) > 256:
        return False
    try:
        parsed = urlparse(address)
    except ValueError:
        return False
    if parsed.scheme.lower() not in VALID_SCHEMES:
        return False
    if not parsed.hostname:
        return False
    if parsed.path not in ("", "/"):
        return False
    return True


class PeerRegistry:
    """
    Persisted set of known peers for a single node.

    Registration is symmetric: `register()` is used both when this node
    learns about a peer (e.g. via CLI `add-peer` or a bootstrap list) and
    when a remote peer announces itself to this node via
    `POST /network/peers/register`.
    """

    def __init__(
        self,
        storage: Storage,
        self_address: str,
        self_node_id: str,
        max_peers: int,
    ) -> None:
        self._storage = storage
        self._self_address = normalize_address(self_address)
        self._self_node_id = self_node_id
        self._max_peers = max_peers
        self._lock = threading.RLock()
        self._health: dict[str, dict[str, object]] = {}

    def register(
        self, address: str, node_id: Optional[str] = None
    ) -> tuple[bool, str]:
        """
        Register a peer by address, rejecting malformed or self-referential
        entries and enforcing the configured peer cap.

        Returns:
            (accepted, reason) -- reason explains rejection when accepted
            is False.
        """
        if not is_valid_peer_address(address):
            return False, "Malformed peer address."

        normalized = normalize_address(address)

        if normalized == self._self_address:
            return False, "Refusing to register self as a peer."
        if node_id is not None and node_id == self._self_node_id:
            return False, "Refusing to register self (matching node_id) as a peer."

        existing = self._storage.get_peer(normalized)
        if existing is None and self._storage.peer_count() >= self._max_peers:
            return False, f"Peer limit reached ({self._max_peers})."

        with self._lock:
            self._health.setdefault(normalized, {"failure_count": 0, "backoff_until": None, "capabilities": ()})
        self._storage.upsert_peer(
            address=normalized,
            node_id=node_id,
            status=existing["status"] if existing else "unknown",
            last_seen=existing.get("last_seen") if existing else None,
        )
        logger.info("Registered peer %s (node_id=%s)", normalized, node_id)
        return True, ""

    def mark_seen(self, address: str, status: str, node_id: Optional[str] = None) -> None:
        """Record successful/failed liveness while retaining bounded retry state."""
        normalized = normalize_address(address)
        now = time.time()
        with self._lock:
            state = self._health.setdefault(normalized, {"failure_count": 0, "backoff_until": None, "capabilities": ()})
            if status == "online":
                state["failure_count"] = 0
                state["backoff_until"] = None
            self._storage.upsert_peer(address=normalized, node_id=node_id, status=status, last_seen=now)

    def mark_failure(self, address: str) -> int:
        """Record a failed operation and return the consecutive failure count."""
        normalized = normalize_address(address)
        with self._lock:
            state = self._health.setdefault(normalized, {"failure_count": 0, "backoff_until": None, "capabilities": ()})
            failures = int(state["failure_count"]) + 1
            delay = min(300.0, 2.0 ** min(failures - 1, 8))
            state["failure_count"] = failures
            state["backoff_until"] = time.time() + delay
            self._storage.upsert_peer(address=normalized, status="offline", last_seen=time.time())
            return failures

    def is_eligible(self, address: str) -> bool:
        normalized = normalize_address(address)
        with self._lock:
            state = self._health.get(normalized)
            return state is None or float(state.get("backoff_until") or 0) <= time.time()

    def failure_count(self, address: str) -> int:
        normalized = normalize_address(address)
        with self._lock:
            return int(self._health.get(normalized, {}).get("failure_count", 0))

    def backoff_until(self, address: str) -> Optional[float]:
        normalized = normalize_address(address)
        with self._lock:
            return self._health.get(normalized, {}).get("backoff_until")

    def set_capabilities(self, address: str, capabilities: list[str] | tuple[str, ...]) -> None:
        normalized = normalize_address(address)
        with self._lock:
            state = self._health.setdefault(normalized, {"failure_count": 0, "backoff_until": None, "capabilities": ()})
            state["capabilities"] = tuple(sorted(set(str(v) for v in capabilities)))

    def health_snapshot(self) -> dict[str, dict[str, object]]:
        with self._lock:
            return {k: dict(v) for k, v in self._health.items()}

    def healthy_count(self) -> int:
        return sum(1 for p in self.list_peers() if p.status == "online" and self.is_eligible(p.address))

    def remove(self, address: str) -> None:
        """Remove a peer from the registry."""
        self._storage.remove_peer(normalize_address(address))

    def list_peers(self) -> list[Peer]:
        """Return every known peer, enriched with runtime health/capability state."""
        peers = []
        with self._lock:
            for row in self._storage.list_peers():
                peer = Peer.from_row(row)
                state = self._health.get(peer.address, {})
                peer.failure_count = int(state.get("failure_count", 0))
                peer.backoff_until = state.get("backoff_until")
                peer.capabilities = tuple(state.get("capabilities", ()))
                peers.append(peer)
        return peers

    def get(self, address: str) -> Optional[Peer]:
        """Look up a single known peer by address."""
        normalized = normalize_address(address)
        row = self._storage.get_peer(normalized)
        if row is None:
            return None
        peer = Peer.from_row(row)
        with self._lock:
            state = self._health.get(normalized, {})
            peer.failure_count = int(state.get("failure_count", 0))
            peer.backoff_until = state.get("backoff_until")
            peer.capabilities = tuple(state.get("capabilities", ()))
        return peer

    def count(self) -> int:
        """Return the number of known peers."""
        return self._storage.peer_count()

    def addresses(self) -> list[str]:
        """Return every known peer's address."""
        return [peer.address for peer in self.list_peers()]
