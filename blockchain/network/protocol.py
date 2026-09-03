"""
Wire-level HTTP client for peer-to-peer communication.

Uses `httpx` (already a project dependency via FastAPI's test tooling) so
no new dependency is introduced. Every method here is intentionally
"dumb": it only sends/receives JSON and never interprets its meaning --
parsing into `Block`/`Transaction` objects and deciding what to do with
them belongs to `sync.py` and `propagation.py`.

`transport` is exposed specifically so tests can point a PeerClient
directly at another node's in-process FastAPI application via
`httpx.ASGITransport`, giving true two-node integration tests without
binding real network sockets.

Phase 6.5: every outbound call is gated through
`address_security.validate_peer_address()` before any request is made --
this is the client-side half of the centralized SSRF defense (the
server-side half validates addresses before they're ever persisted as
peers or accepted as sync targets). Applying the check here too means
even a future call site that forgets to validate upstream still cannot
cause this client to dial a disallowed address.
"""

from __future__ import annotations

from typing import Any, Optional

import httpx

from blockchain.network.address_security import validate_peer_address
from blockchain.utils import get_logger
from enum import Enum

logger = get_logger("blockchain.network.protocol")

class MessageType(str, Enum):
    HELLO = "HELLO"
    PEER_LIST = "PEER_LIST"
    PING = "PING"
    PONG = "PONG"
    GET_CHAIN = "GET_CHAIN"
    GET_BLOCK = "GET_BLOCK"
    GET_BLOCKS = "GET_BLOCKS"
    NEW_TRANSACTION = "NEW_TRANSACTION"
    NEW_BLOCK = "NEW_BLOCK"
    SYNC_REQUEST = "SYNC_REQUEST"
    SYNC_RESPONSE = "SYNC_RESPONSE"


class PeerClient:
    """Thin HTTP client for calling another node's `/network/*` endpoints."""

    def __init__(
        self,
        timeout: float = 5.0,
        transport: Optional[httpx.BaseTransport] = None,
        allow_private_addresses: bool = True,
    ) -> None:
        self._timeout = timeout
        self._transport = transport
        self._allow_private_addresses = allow_private_addresses

    def _validate_target(self, base_url: str) -> Optional[str]:
        """Return an error string if `base_url` fails address validation, else None."""
        ok, reason = validate_peer_address(base_url, self._allow_private_addresses)
        if not ok:
            logger.warning("Refusing outbound request to %s: %s", base_url, reason)
            return reason
        return None

    def _client(self, base_url: str) -> httpx.Client:
        return httpx.Client(
            base_url=base_url, timeout=self._timeout, transport=self._transport
        )

    def _get(self, base_url: str, path: str) -> Optional[dict[str, Any]]:
        if self._validate_target(base_url) is not None:
            return None
        try:
            with self._client(base_url) as client:
                response = client.get(path)
            if response.status_code != 200:
                logger.warning(
                    "GET %s%s returned %s", base_url, path, response.status_code
                )
                return None
            return response.json()
        except httpx.HTTPError as exc:
            logger.warning("GET %s%s failed: %s", base_url, path, exc)
            return None

    def _post(
        self, base_url: str, path: str, payload: dict[str, Any]
    ) -> Optional[dict[str, Any]]:
        if self._validate_target(base_url) is not None:
            return None
        try:
            with self._client(base_url) as client:
                response = client.post(path, json=payload)
            if response.status_code not in (200, 201):
                logger.info(
                    "POST %s%s returned %s: %s",
                    base_url,
                    path,
                    response.status_code,
                    response.text[:200],
                )
                return None
            return response.json()
        except httpx.HTTPError as exc:
            logger.warning("POST %s%s failed: %s", base_url, path, exc)
            return None

    def get_status(self, peer_address: str) -> Optional[dict[str, Any]]:
        """Fetch a peer's `/network/status`. Returns None if unreachable."""
        return self._get(peer_address, "/network/status")

    def get_chain(self, peer_address: str) -> Optional[dict[str, Any]]:
        """Fetch a peer's full chain via `/network/chain`."""
        return self._get(peer_address, "/network/chain")

    def get_peers(self, peer_address: str) -> Optional[dict[str, Any]]:
        """Fetch a peer's known-peers list via `/network/peers`."""
        return self._get(peer_address, "/network/peers")

    def register_with(
        self, peer_address: str, node_id: str, self_address: str
    ) -> Optional[dict[str, Any]]:
        """
        Announce this node to `peer_address` via `/network/peers/register`.

        This is unauthenticated *discovery* only -- it does not establish
        trust. See `authenticate_with` for the cryptographic handshake
        that a peer must complete before it is treated as trusted.
        """
        return self._post(
            peer_address,
            "/network/peers/register",
            {"node_id": node_id, "address": self_address},
        )

    def request_challenge(self, peer_address: str, node_id: str) -> Optional[dict[str, Any]]:
        """Request a fresh authentication challenge from `peer_address`."""
        return self._post(peer_address, "/network/peers/challenge", {"node_id": node_id})

    def authenticate_with(
        self, peer_address: str, auth_envelope: dict[str, Any]
    ) -> Optional[dict[str, Any]]:
        """Complete the cryptographic handshake with `peer_address`."""
        return self._post(
            peer_address, "/network/peers/authenticate", {"auth": auth_envelope}
        )

    def send_block(
        self,
        peer_address: str,
        block: dict[str, Any],
        from_peer: str,
        auth_envelope: dict[str, Any],
    ) -> Optional[dict[str, Any]]:
        """Propagate a mined block to `peer_address`, authenticated."""
        return self._post(
            peer_address,
            "/network/blocks/receive",
            {"message_type": MessageType.NEW_BLOCK.value, "block": block, "from_peer": from_peer, "auth": auth_envelope},
        )

    def send_transaction(
        self,
        peer_address: str,
        transaction: dict[str, Any],
        from_peer: str,
        auth_envelope: dict[str, Any],
    ) -> Optional[dict[str, Any]]:
        """Propagate a mempool transaction to `peer_address`, authenticated."""
        return self._post(
            peer_address,
            "/network/transactions/receive",
            {"message_type": MessageType.NEW_TRANSACTION.value, "transaction": transaction, "from_peer": from_peer, "auth": auth_envelope},
        )

    def request_sync(
        self,
        peer_address: str,
        auth_envelope: dict[str, Any],
        target_peer_address: Optional[str] = None,
    ) -> Optional[dict[str, Any]]:
        """Ask `peer_address` to run a synchronization pass, authenticated."""
        return self._post(
            peer_address,
            "/network/sync",
            {"message_type": MessageType.SYNC_REQUEST.value, "auth": auth_envelope, "peer_address": target_peer_address},
        )
