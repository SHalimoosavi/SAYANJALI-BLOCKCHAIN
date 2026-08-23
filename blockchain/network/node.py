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
from blockchain.network.handshake import AuthContext, ReplayCache
from blockchain.network.identity import P2PIdentity
from blockchain.network.peer import PeerRegistry
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

        for bootstrap_address in self.settings.p2p.bootstrap_peers:
            accepted, reason = self.peers.register(bootstrap_address)
            if not accepted:
                logger.warning(
                    "Bootstrap peer %s not registered: %s", bootstrap_address, reason
                )

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
        }
