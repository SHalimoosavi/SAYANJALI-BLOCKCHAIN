"""
Tests for Phase 6.5's peer-abuse protections: oversized payload
rejection (via the global body-size middleware) and bounded, tiered
per-peer rate limiting on the network API endpoints.
"""

from __future__ import annotations

import pytest
from fastapi.testclient import TestClient


@pytest.fixture
def client():
    import api.network_routes as network_routes_module
    import api.routes as routes_module
    from api.main import app

    routes_module._blockchain = None
    network_routes_module._reset_module_state_for_tests()
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


def test_oversized_handshake_payload_rejected(client: TestClient):
    """
    /network/peers/register carries a small, tight max_handshake_payload_bytes
    cap (8KB by default) -- much smaller than the generic default -- since
    a legitimate registration body is tiny.
    """
    response = client.post(
        "/network/peers/register",
        json={"node_id": "x" * 50_000, "address": "http://127.0.0.1:9000"},
    )
    assert response.status_code == 413


def _reload_app_with_env(monkeypatch, **env_vars) -> TestClient:
    """Set env vars, clear the settings cache, and return a fresh TestClient."""
    for key, value in env_vars.items():
        monkeypatch.setenv(key, str(value))

    from config import settings as settings_module

    settings_module.get_settings.cache_clear()

    import api.network_routes as network_routes_module
    import api.routes as routes_module
    from api.main import app

    routes_module._blockchain = None
    network_routes_module._reset_module_state_for_tests()

    return TestClient(app)


def test_rate_limit_blocks_excessive_requests_on_expensive_tier(monkeypatch):
    """
    GET /network/chain sits in the "expensive" tier (1.0x the configured
    base rate), so setting the base rate to 5 gives an exact, predictable
    boundary to test against.
    """
    client = _reload_app_with_env(
        monkeypatch, SYJ_PEER_RATE_LIMIT_REQUESTS=5, SYJ_PEER_RATE_LIMIT_WINDOW=60
    )

    statuses = [client.get("/network/chain").status_code for _ in range(8)]
    assert statuses[:5] == [200] * 5
    assert statuses[5:] == [429] * 3


def test_rate_limit_cheap_tier_more_generous_than_expensive(monkeypatch):
    """
    GET /network/status sits in the "cheap" tier (4x the base rate) --
    confirms tiers are actually differentiated, not just a single global
    limit applied uniformly everywhere.
    """
    client = _reload_app_with_env(
        monkeypatch, SYJ_PEER_RATE_LIMIT_REQUESTS=5, SYJ_PEER_RATE_LIMIT_WINDOW=60
    )

    # 8 requests would exhaust the "expensive" tier's budget (5) but the
    # "cheap" tier's budget is 4x that (20), so all 8 succeed here.
    statuses = [client.get("/network/status").status_code for _ in range(8)]
    assert statuses == [200] * 8


def test_rate_limit_is_independent_per_peer(monkeypatch):
    client = _reload_app_with_env(
        monkeypatch, SYJ_PEER_RATE_LIMIT_REQUESTS=2, SYJ_PEER_RATE_LIMIT_WINDOW=60
    )

    for _ in range(2 * 2):  # moderate tier = 2x base = 4 allowed for peer-a
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


def test_rate_limiter_memory_is_bounded_under_unique_fake_peers(monkeypatch):
    """
    Direct regression test for the original unbounded-dict vulnerability:
    an attacker submitting one request per unique fake `from_peer` value
    must not grow the rate limiter's tracked-key count without bound.
    """
    client = _reload_app_with_env(
        monkeypatch,
        SYJ_PEER_RATE_LIMIT_REQUESTS=60,
        SYJ_PEER_RATE_LIMIT_WINDOW=60,
        SYJ_RATE_LIMIT_MAX_KEYS=50,
    )

    for i in range(500):
        client.post(
            "/network/transactions/receive",
            json={"transaction": {}, "from_peer": f"http://127.0.0.1:{9000+i}"},
        )

    import api.network_routes as network_routes_module

    limiter = network_routes_module._get_rate_limiter("expensive")
    assert limiter.tracked_key_count() <= 50
