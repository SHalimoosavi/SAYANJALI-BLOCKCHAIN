"""Tests for blockchain/network/peer.py."""

from __future__ import annotations

from blockchain.network.peer import PeerRegistry, is_valid_peer_address, normalize_address


def test_valid_peer_addresses_accepted():
    assert is_valid_peer_address("http://127.0.0.1:8000")
    assert is_valid_peer_address("https://node.example.com:9000")


def test_invalid_peer_addresses_rejected():
    assert not is_valid_peer_address("not-a-url")
    assert not is_valid_peer_address("ftp://127.0.0.1:8000")
    assert not is_valid_peer_address("")
    assert not is_valid_peer_address(None)
    assert not is_valid_peer_address("http://")
    assert not is_valid_peer_address("http://127.0.0.1:8000/some/path")
    assert not is_valid_peer_address("x" * 300)


def test_normalize_address_strips_trailing_slash():
    assert normalize_address("http://127.0.0.1:8000/") == "http://127.0.0.1:8000"
    assert normalize_address("  http://127.0.0.1:8000  ") == "http://127.0.0.1:8000"


def test_register_peer(blockchain):
    registry = PeerRegistry(
        blockchain.storage, "http://127.0.0.1:8000", "self-node-id", max_peers=10
    )
    accepted, reason = registry.register("http://127.0.0.1:8001", node_id="peer-1")
    assert accepted, reason
    assert registry.count() == 1
    peer = registry.get("http://127.0.0.1:8001")
    assert peer is not None
    assert peer.node_id == "peer-1"


def test_register_rejects_malformed_address(blockchain):
    registry = PeerRegistry(
        blockchain.storage, "http://127.0.0.1:8000", "self-node-id", max_peers=10
    )
    accepted, reason = registry.register("not-a-url")
    assert not accepted
    assert registry.count() == 0


def test_register_rejects_self_by_address(blockchain):
    registry = PeerRegistry(
        blockchain.storage, "http://127.0.0.1:8000", "self-node-id", max_peers=10
    )
    accepted, reason = registry.register("http://127.0.0.1:8000")
    assert not accepted
    assert "self" in reason.lower()


def test_register_rejects_self_by_node_id(blockchain):
    registry = PeerRegistry(
        blockchain.storage, "http://127.0.0.1:8000", "self-node-id", max_peers=10
    )
    accepted, reason = registry.register(
        "http://127.0.0.1:9999", node_id="self-node-id"
    )
    assert not accepted


def test_register_enforces_max_peers(blockchain):
    registry = PeerRegistry(
        blockchain.storage, "http://127.0.0.1:8000", "self-node-id", max_peers=2
    )
    assert registry.register("http://127.0.0.1:8001")[0]
    assert registry.register("http://127.0.0.1:8002")[0]
    accepted, reason = registry.register("http://127.0.0.1:8003")
    assert not accepted
    assert "limit" in reason.lower()


def test_register_same_peer_twice_is_idempotent(blockchain):
    registry = PeerRegistry(
        blockchain.storage, "http://127.0.0.1:8000", "self-node-id", max_peers=2
    )
    registry.register("http://127.0.0.1:8001")
    accepted, reason = registry.register("http://127.0.0.1:8001")
    assert accepted, reason
    assert registry.count() == 1


def test_peer_registry_persists_across_instances(blockchain):
    registry1 = PeerRegistry(
        blockchain.storage, "http://127.0.0.1:8000", "self-node-id", max_peers=10
    )
    registry1.register("http://127.0.0.1:8001", node_id="peer-1")

    registry2 = PeerRegistry(
        blockchain.storage, "http://127.0.0.1:8000", "self-node-id", max_peers=10
    )
    assert registry2.count() == 1
    assert registry2.get("http://127.0.0.1:8001").node_id == "peer-1"


def test_mark_seen_updates_status(blockchain):
    registry = PeerRegistry(
        blockchain.storage, "http://127.0.0.1:8000", "self-node-id", max_peers=10
    )
    registry.register("http://127.0.0.1:8001")
    registry.mark_seen("http://127.0.0.1:8001", "online")
    peer = registry.get("http://127.0.0.1:8001")
    assert peer.status == "online"
    assert peer.last_seen is not None


def test_remove_peer(blockchain):
    registry = PeerRegistry(
        blockchain.storage, "http://127.0.0.1:8000", "self-node-id", max_peers=10
    )
    registry.register("http://127.0.0.1:8001")
    registry.remove("http://127.0.0.1:8001")
    assert registry.count() == 0
