"""
NetworkNode: this node's P2P identity and coordination point.

A `NetworkNode` wraps a `Blockchain` instance with everything Phase 2
needs: a persistent node identity, a persisted peer registry, an HTTP
client for talking to peers, and bounded caches of recently seen block
and transaction hashes used to prevent propagation loops. It does not
implement sync or propagation logic itself -- see `sync.py` and
`propagation.py` -- it only owns the state those modules operate on.
"""

from __future__ import annotations

from typing import Optional
import asyncio

import httpx

from blockchain.blockchain import Blockchain
from blockchain.network.handshake import AuthContext, ReplayCache
from blockchain.network.identity import P2PIdentity
from blockchain.network.peer import PeerRegistry
from blockchain.network.lifecycle import NodeLifecycle
from blockchain.network.protocol import PeerClient
from blockchain.utils import BoundedSet, get_logger
from config.settings import Settings

logger = get_logger("blockchain.network.node")

SEEN_CACHE_SIZE = 5000


class NetworkNode:
    """
    Owns a single node's networking state: identity, peers, the HTTP
    client used to reach them, and (Phase 6.5) the cryptographic identity
    and replay-protection state needed to authenticate with peers.
    Constructed once per running process (or once per CLI invocation)
    around an existing `Blockchain` instance.
    """

    def __init__(
        self,
        blockchain: Blockchain,
        settings: Optional[Settings] = None,
        transport: Optional[httpx.BaseTransport] = None,
    ) -> None:
        self.blockchain = blockchain
        self.settings = settings or blockchain.settings

        self.node_id = self.settings.p2p.node_id_override or blockchain.storage.get_or_create_node_id()
        self.self_address = self.settings.resolved_advertised_address

        # Phase 6.5: this node's own P2P identity keypair, cryptographically
        # bound to node_id via Storage, kept entirely separate from any
        # wallet key. Loaded (or generated once) here so every consumer of
        # NetworkNode shares the same identity instance.
        private_key_hex, public_key_hex = blockchain.storage.get_or_create_p2p_identity(
            self.node_id
        )
        self.identity = P2PIdentity(
            node_id=self.node_id,
            private_key_hex=private_key_hex,
            public_key_hex=public_key_hex,
        )
        self.auth_context = AuthContext(
            identity=self.identity,
            network_name=self.settings.network_name,
            genesis_hash=blockchain.genesis_block.hash,
            advertised_address=self.self_address,
        )
        self.replay_cache = ReplayCache(max_size=self.settings.p2p.replay_cache_size)

        self.peers = PeerRegistry(
            storage=blockchain.storage,
            self_address=self.self_address,
            self_node_id=self.node_id,
            max_peers=self.settings.p2p.max_peers,
        )
        self.client = PeerClient(
            timeout=self.settings.p2p.propagation_timeout_seconds,
            transport=transport,
            allow_private_addresses=self.settings.p2p.allow_private_peer_addresses,
        )
        self.sync_client = PeerClient(
            timeout=self.settings.p2p.sync_timeout_seconds,
            transport=transport,
            allow_private_addresses=self.settings.p2p.allow_private_peer_addresses,
        )

        self.seen_blocks = BoundedSet(SEEN_CACHE_SIZE)
        self.seen_transactions = BoundedSet(SEEN_CACHE_SIZE)
        self.lifecycle = NodeLifecycle.STARTING
        self._background_tasks: set[asyncio.Task] = set()
        self._stop_event: asyncio.Event | None = None
        self._propagation_stats = {"blocks_sent": 0, "transactions_sent": 0, "blocks_received": 0, "transactions_received": 0, "sync_attempts": 0}

        for bootstrap_address in self.settings.p2p.bootstrap_peers:
            accepted, reason = self.peers.register(bootstrap_address)
            if not accepted:
                logger.warning(
                    "Bootstrap peer %s not registered: %s", bootstrap_address, reason
                )

    async def _discover_bootstrap_peers(self) -> None:
        """Register and authenticate configured bootstrap peers without scanning."""
        from blockchain.network.handshake import build_handshake_envelope
        for address in self.settings.p2p.bootstrap_peers:
            try:
                registration = self.client.register_with(address, self.node_id, self.self_address)
                if registration is None:
                    self.peers.mark_failure(address)
                    continue
                self.peers.register(address, registration.get("self_node_id"))
                challenge = self.client.request_challenge(address, self.node_id)
                if not challenge or not challenge.get("challenge"):
                    self.peers.mark_failure(address)
                    continue
                envelope = build_handshake_envelope(self.auth_context, challenge["challenge"])
                result = self.client.authenticate_with(address, envelope)
                if result and result.get("authenticated"):
                    self.peers.mark_seen(address, "online", result.get("self_node_id"))
                else:
                    self.peers.mark_failure(address)
            except Exception as exc:
                logger.warning("Bootstrap discovery/authentication failed for %s: %s", address, exc)
                self.peers.mark_failure(address)

    async def start(self) -> None:
        """Start background network maintenance exactly once."""
        if self.lifecycle is NodeLifecycle.RUNNING:
            return
        self.lifecycle = NodeLifecycle.STARTING
        self._stop_event = asyncio.Event()
        await self._discover_bootstrap_peers()
        self.lifecycle = NodeLifecycle.RUNNING
        task = asyncio.create_task(self._maintenance_loop(), name=f"syj-network-{self.node_id[:8]}")
        self._background_tasks.add(task)
        task.add_done_callback(self._background_tasks.discard)

    async def stop(self) -> None:
        """Stop background work and close peer HTTP clients cleanly."""
        if self.lifecycle is NodeLifecycle.STOPPED:
            return
        self.lifecycle = NodeLifecycle.STOPPING
        if self._stop_event:
            self._stop_event.set()
        tasks = list(self._background_tasks)
        for task in tasks:
            task.cancel()
        if tasks:
            await asyncio.gather(*tasks, return_exceptions=True)
        self._background_tasks.clear()
        self.lifecycle = NodeLifecycle.STOPPED

    async def _maintenance_loop(self) -> None:
        """Periodically discover peers and synchronize healthy peers."""
        while self._stop_event and not self._stop_event.is_set():
            try:
                await asyncio.to_thread(self.discover_peers)
                from blockchain.network.sync import sync_with_all_peers
                self.lifecycle = NodeLifecycle.SYNCING
                results = await asyncio.to_thread(sync_with_all_peers, self)
                if results and all(not r.accepted and "rejected" in r.reason.lower() for r in results):
                    self.lifecycle = NodeLifecycle.RUNNING
                else:
                    self.lifecycle = NodeLifecycle.RUNNING
            except Exception as exc:  # maintenance must not kill the node
                logger.warning("Network maintenance degraded: %s", exc)
                self.lifecycle = NodeLifecycle.DEGRADED
            try:
                await asyncio.wait_for(self._stop_event.wait(), timeout=self.settings.p2p.maintenance_interval_seconds)
            except asyncio.TimeoutError:
                pass

    def discover_peers(self) -> int:
        """Exchange known peer addresses with configured bootstrap/known peers."""
        added = 0
        for peer in list(self.peers.addresses()):
            if not self.peers.is_eligible(peer):
                continue
            payload = self.client.get_peers(peer)
            if payload is None:
                self.peers.mark_failure(peer)
                continue
            self.peers.mark_seen(peer, "online")
            for peer_info in payload.get("peers", []):
                address = peer_info.get("address") if isinstance(peer_info, dict) else peer_info
                if not address:
                    continue
                accepted, _ = self.peers.register(address, peer_info.get("node_id") if isinstance(peer_info, dict) else None)
                if accepted:
                    if isinstance(peer_info, dict):
                        self.peers.set_capabilities(address, peer_info.get("capabilities", ()))
                    added += 1
        return added

    def record_propagation(self, kind: str, sent: int = 0, received: int = 0) -> None:
        self._propagation_stats[f"{kind}_sent"] += sent
        self._propagation_stats[f"{kind}_received"] += received

    def is_trusted_peer(self, address: str) -> bool:
        """Return True if `address` has successfully authenticated before."""
        return self.blockchain.storage.is_peer_trusted(address)

    def status(self) -> dict:
        """Return a summary dict describing this node's networking state."""
        return {
            "node_id": self.node_id,
            "self_address": self.self_address,
            "public_key": self.identity.public_key_hex,
            "peer_count": self.peers.count(),
            "chain_length": self.blockchain.length,
            "chain_work": self.blockchain.total_work(),
            "network_name": self.settings.network_name,
            "chain_id": self.settings.network.chain_id,
            "lifecycle": self.lifecycle.value,
            "healthy_peer_count": self.peers.healthy_count(),
            "sync_state": self.lifecycle.value,
            "mempool_size": self.blockchain.mempool.size(),
            "current_difficulty": self.blockchain.current_difficulty(),
            "total_supply_base_units": self.blockchain.total_supply_base_units(),
            "propagation": dict(self._propagation_stats),
        }
