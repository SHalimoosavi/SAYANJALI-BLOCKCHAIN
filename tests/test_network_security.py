"""
Tests for Phase 2's basic peer-abuse protections: oversized payload
rejection and per-peer rate limiting on the network API endpoints.
"""

from __future__ import annotations

import pytest
from fastapi.testclient import TestClient


@pytest.fixture
def client(monkeypatch):
    import api.network_routes as network_routes_module
    import api.routes as routes_module
    from api.main import app

    routes_module._blockchain = None
    network_routes_module._network_node = None
    network_routes_module._rate_limiter = None
    return TestClient(app)


def test_oversized_transaction_payload_rejected(client: TestClient):
    oversized_payload = {
        "transaction": {
            "sender": "x" * 20_000,
            "receiver": "y",
            "amount": 1,
            "timestamp": 1,
            "tx_hash": "z",
        },
        "from_peer": "http://127.0.0.1:9000",
    }
    response = client.post("/network/transactions/receive", json=oversized_payload)
    assert response.status_code == 413


def test_oversized_block_payload_rejected(client: TestClient):
    oversized_payload = {
        "block": {"index": 1, "previous_hash": "a" * 1_000_000},
        "from_peer": "http://127.0.0.1:9000",
    }
    response = client.post("/network/blocks/receive", json=oversized_payload)
    assert response.status_code == 413


def test_rate_limit_blocks_excessive_requests_from_one_peer(monkeypatch):
    monkeypatch.setenv("SYJ_PEER_RATE_LIMIT_REQUESTS", "5")
    monkeypatch.setenv("SYJ_PEER_RATE_LIMIT_WINDOW", "60")

    from config import settings as settings_module

    settings_module.get_settings.cache_clear()

    import api.network_routes as network_routes_module
    import api.routes as routes_module
    from api.main import app

    routes_module._blockchain = None
    network_routes_module._network_node = None
    network_routes_module._rate_limiter = None

    client = TestClient(app)
    statuses = []
    for _ in range(8):
        response = client.post(
            "/network/peers/register",
            json={"node_id": "same-peer", "address": "http://127.0.0.1:7000"},
        )
        statuses.append(response.status_code)

    assert statuses[:5] == [200] * 5
    assert statuses[5:] == [429] * 3

    settings_module.get_settings.cache_clear()


def test_rate_limit_is_independent_per_peer(monkeypatch):
    monkeypatch.setenv("SYJ_PEER_RATE_LIMIT_REQUESTS", "2")
    monkeypatch.setenv("SYJ_PEER_RATE_LIMIT_WINDOW", "60")

    from config import settings as settings_module

    settings_module.get_settings.cache_clear()

    import api.network_routes as network_routes_module
    import api.routes as routes_module
    from api.main import app

    routes_module._blockchain = None
    network_routes_module._network_node = None
    network_routes_module._rate_limiter = None

    client = TestClient(app)

    for _ in range(2):
        r = client.post(
            "/network/peers/register",
            json={"node_id": "peer-a", "address": "http://127.0.0.1:7001"},
        )
        assert r.status_code == 200

    # A different peer is not affected by peer-a's usage of its own quota.
    r = client.post(
        "/network/peers/register",
        json={"node_id": "peer-b", "address": "http://127.0.0.1:7002"},
    )
    assert r.status_code == 200

    settings_module.get_settings.cache_clear()
