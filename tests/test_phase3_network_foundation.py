"""Phase 3 network lifecycle, health/backoff, protocol and configuration tests."""
from __future__ import annotations

import time

from blockchain.network.peer import PeerRegistry
from blockchain.network.protocol import MessageType


class FakeStorage:
    def __init__(self):
        self.rows = {}
    def get_peer(self, address): return self.rows.get(address)
    def peer_count(self): return len(self.rows)
    def upsert_peer(self, address, node_id=None, status="unknown", last_seen=None):
        row = self.rows.setdefault(address, {"address": address, "registered_at": time.time()})
        row.update({"node_id": node_id or row.get("node_id"), "status": status, "last_seen": last_seen})
    def list_peers(self): return list(self.rows.values())
    def remove_peer(self, address): self.rows.pop(address, None)


def test_peer_failure_backoff_and_recovery():
    storage = FakeStorage()
    registry = PeerRegistry(storage, "http://127.0.0.1:9000", "self", 4)
    assert registry.register("http://127.0.0.1:9001")[0]
    assert registry.is_eligible("http://127.0.0.1:9001")
    failures = registry.mark_failure("http://127.0.0.1:9001")
    assert failures == 1
    assert not registry.is_eligible("http://127.0.0.1:9001")
    assert registry.failure_count("http://127.0.0.1:9001") == 1
    registry.mark_seen("http://127.0.0.1:9001", "online")
    assert registry.is_eligible("http://127.0.0.1:9001")
    assert registry.failure_count("http://127.0.0.1:9001") == 0


def test_peer_capabilities_are_normalized():
    storage = FakeStorage()
    registry = PeerRegistry(storage, "http://127.0.0.1:9000", "self", 4)
    registry.register("http://127.0.0.1:9001")
    registry.set_capabilities("http://127.0.0.1:9001", ["sync", "blocks", "sync"])
    assert registry.get("http://127.0.0.1:9001").capabilities == ("blocks", "sync")


def test_protocol_message_types_are_explicit_and_stable():
    assert MessageType.HELLO.value == "HELLO"
    assert MessageType.NEW_BLOCK.value == "NEW_BLOCK"
    assert MessageType.NEW_TRANSACTION.value == "NEW_TRANSACTION"
    assert MessageType.SYNC_REQUEST.value == "SYNC_REQUEST"


def test_lifecycle_enum_covers_shutdown_states():
    from blockchain.network.lifecycle import NodeLifecycle
    assert [s.value for s in NodeLifecycle] == ["STARTING", "RUNNING", "SYNCING", "DEGRADED", "STOPPING", "STOPPED"]


def test_network_status_exposes_non_sensitive_observability():
    from fastapi.testclient import TestClient
    from api.main import app
    response = TestClient(app).get("/network/status")
    assert response.status_code == 200
    body = response.json()
    assert body["chain_id"] == 1
    assert "total_supply_base_units" in body
    assert "lifecycle" in body
    assert "public_key" in body
    assert "private_key" not in body
