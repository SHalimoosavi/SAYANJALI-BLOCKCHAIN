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

import httpx

from blockchain.blockchain import Blockchain
from blockchain.network.peer import PeerRegistry
from blockchain.network.protocol import PeerClient
from blockchain.utils import BoundedSet, get_logger
from config.settings import Settings

logger = get_logger("blockchain.network.node")

SEEN_CACHE_SIZE = 5000


class NetworkNode:
    """
    Owns a single node's networking state: identity, peers, and the HTTP
    client used to reach them. Constructed once per running process (or
    once per CLI invocation) around an existing `Blockchain` instance.
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

        self.peers = PeerRegistry(
            storage=blockchain.storage,
            self_address=self.self_address,
            self_node_id=self.node_id,
            max_peers=self.settings.p2p.max_peers,
        )
        self.client = PeerClient(
            timeout=self.settings.p2p.propagation_timeout_seconds,
            transport=transport,
        )
        self.sync_client = PeerClient(
            timeout=self.settings.p2p.sync_timeout_seconds,
            transport=transport,
        )

        self.seen_blocks = BoundedSet(SEEN_CACHE_SIZE)
        self.seen_transactions = BoundedSet(SEEN_CACHE_SIZE)

        for bootstrap_address in self.settings.p2p.bootstrap_peers:
            accepted, reason = self.peers.register(bootstrap_address)
            if not accepted:
                logger.warning(
                    "Bootstrap peer %s not registered: %s", bootstrap_address, reason
                )

    def status(self) -> dict:
        """Return a summary dict describing this node's networking state."""
        return {
            "node_id": self.node_id,
            "self_address": self.self_address,
            "peer_count": self.peers.count(),
            "chain_length": self.blockchain.length,
            "chain_work": self.blockchain.total_work(),
            "network_name": self.settings.network_name,
        }
