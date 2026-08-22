"""
Peer-to-peer networking package for SAYANJALI BLOCKCHAIN (Phase 2).

This package adds multi-node capability on top of the Phase 1 single-node
core without modifying that core's business rules: every validation
decision here delegates to `blockchain.validators`, `blockchain.consensus`,
and `blockchain.blockchain.Blockchain`. Nothing in this package re-derives
or duplicates chain, transaction, or consensus logic.

Modules:
    peer.py         Peer identity + persisted PeerRegistry
    protocol.py     HTTP wire client for talking to peers (httpx-based)
    node.py         NetworkNode: this node's identity + coordination point
    sync.py         Chain fetch / validate / compare-work / adopt
    propagation.py  Block and transaction broadcast + receive handling
    ratelimit.py    Basic per-peer request rate limiting

This is an HTTP-based P2P prototype, not a production peer-to-peer
protocol. See the module docstrings and README's Security section for the
specific limitations that implies.
"""

from blockchain.network.node import NetworkNode
from blockchain.network.peer import Peer, PeerRegistry

__all__ = ["NetworkNode", "Peer", "PeerRegistry"]
