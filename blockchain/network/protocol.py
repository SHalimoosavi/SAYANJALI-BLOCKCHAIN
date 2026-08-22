"""
Wire-level HTTP client for peer-to-peer communication.

Uses `httpx` (already a project dependency via FastAPI's test tooling) so
no new dependency is introduced for Phase 2. Every method here is
intentionally "dumb": it only sends/receives JSON and never interprets
its meaning -- parsing into `Block`/`Transaction` objects and deciding
what to do with them belongs to `sync.py` and `propagation.py`.

`transport` is exposed specifically so tests can point a PeerClient
directly at another node's in-process FastAPI application via
`httpx.ASGITransport`, giving true two-node integration tests without
binding real network sockets.
"""

from __future__ import annotations

from typing import Any, Optional

import httpx

from blockchain.utils import get_logger

logger = get_logger("blockchain.network.protocol")


class PeerClient:
    """Thin HTTP client for calling another node's `/network/*` endpoints."""

    def __init__(
        self,
        timeout: float = 5.0,
        transport: Optional[httpx.BaseTransport] = None,
    ) -> None:
        self._timeout = timeout
        self._transport = transport

    def _client(self, base_url: str) -> httpx.Client:
        return httpx.Client(
            base_url=base_url, timeout=self._timeout, transport=self._transport
        )

    def _get(self, base_url: str, path: str) -> Optional[dict[str, Any]]:
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
        """Announce this node to `peer_address` via `/network/peers/register`."""
        return self._post(
            peer_address,
            "/network/peers/register",
            {"node_id": node_id, "address": self_address},
        )

    def send_block(
        self, peer_address: str, block: dict[str, Any], from_peer: str
    ) -> Optional[dict[str, Any]]:
        """Propagate a mined block to `peer_address`."""
        return self._post(
            peer_address,
            "/network/blocks/receive",
            {"block": block, "from_peer": from_peer},
        )

    def send_transaction(
        self, peer_address: str, transaction: dict[str, Any], from_peer: str
    ) -> Optional[dict[str, Any]]:
        """Propagate a mempool transaction to `peer_address`."""
        return self._post(
            peer_address,
            "/network/transactions/receive",
            {"transaction": transaction, "from_peer": from_peer},
        )
