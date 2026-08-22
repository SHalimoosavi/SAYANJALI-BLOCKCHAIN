"""Integration tests for api/network_routes.py using FastAPI's TestClient."""

from __future__ import annotations

import pytest
from fastapi.testclient import TestClient


@pytest.fixture
def client():
    import api.network_routes as network_routes_module
    import api.routes as routes_module
    from api.main import app

    routes_module._blockchain = None
    network_routes_module._network_node = None
    network_routes_module._rate_limiter = None
    return TestClient(app)


def test_network_status(client: TestClient):
    response = client.get("/network/status")
    assert response.status_code == 200
    body = response.json()
    assert body["chain_length"] == 1
    assert body["peer_count"] == 0
    assert "node_id" in body


def test_network_peers_empty_initially(client: TestClient):
    response = client.get("/network/peers")
    assert response.status_code == 200
    assert response.json() == {"count": 0, "peers": []}


def test_register_peer_success(client: TestClient):
    response = client.post(
        "/network/peers/register",
        json={"node_id": "peer-node-1", "address": "http://127.0.0.1:8001"},
    )
    assert response.status_code == 200
    body = response.json()
    assert body["accepted"]
    assert "self_node_id" in body

    peers = client.get("/network/peers").json()
    assert peers["count"] == 1
    assert peers["peers"][0]["address"] == "http://127.0.0.1:8001"


def test_register_peer_rejects_malformed_address(client: TestClient):
    response = client.post(
        "/network/peers/register", json={"node_id": "x", "address": "not-a-url"}
    )
    assert response.status_code == 200
    body = response.json()
    assert not body["accepted"]


def test_network_chain_includes_difficulty(client: TestClient):
    response = client.get("/network/chain")
    assert response.status_code == 200
    body = response.json()
    assert body["length"] == 1
    assert body["work"] == 1
    assert "difficulty" in body["chain"][0]


def test_receive_block_extends_chain(client: TestClient):
    from blockchain.mining import Miner

    import api.routes as routes_module

    blockchain = routes_module.get_blockchain()
    wallet_resp = client.post("/wallet/create").json()

    engine = Miner(blockchain.consensus, blockchain.settings.mining.block_reward)
    block, _ = engine.mine_block(
        index=blockchain.latest_block.index + 1,
        previous_hash=blockchain.latest_block.hash,
        mempool=blockchain.mempool,
        miner_address=wallet_resp["address"],
        difficulty=blockchain.settings.consensus.difficulty,
    )

    response = client.post(
        "/network/blocks/receive",
        json={"block": block.to_dict(), "from_peer": "http://127.0.0.1:9001"},
    )
    assert response.status_code == 200
    body = response.json()
    assert body["accepted"], body.get("reason")

    status = client.get("/status").json()
    assert status["chain_length"] == 2


def test_receive_duplicate_block_rejected(client: TestClient):
    from blockchain.mining import Miner

    import api.routes as routes_module

    blockchain = routes_module.get_blockchain()
    wallet_resp = client.post("/wallet/create").json()

    engine = Miner(blockchain.consensus, blockchain.settings.mining.block_reward)
    block, _ = engine.mine_block(
        index=blockchain.latest_block.index + 1,
        previous_hash=blockchain.latest_block.hash,
        mempool=blockchain.mempool,
        miner_address=wallet_resp["address"],
        difficulty=blockchain.settings.consensus.difficulty,
    )
    block_dict = block.to_dict()

    first = client.post(
        "/network/blocks/receive", json={"block": block_dict, "from_peer": "http://peer"}
    )
    assert first.json()["accepted"]

    second = client.post(
        "/network/blocks/receive", json={"block": block_dict, "from_peer": "http://peer"}
    )
    assert not second.json()["accepted"]


def test_receive_transaction_enters_mempool(client: TestClient):
    from blockchain.transaction import Transaction

    sender = client.post("/wallet/create").json()
    receiver = client.post("/wallet/create").json()
    client.post("/mine", json={"miner_address": sender["address"]})

    from blockchain.wallet import Wallet

    wallet = Wallet.from_private_key(sender["private_key"])
    tx = Transaction(sender=sender["address"], receiver=receiver["address"], amount=1.0)
    tx.sign(wallet)

    response = client.post(
        "/network/transactions/receive",
        json={"transaction": tx.to_dict(), "from_peer": "http://peer"},
    )
    assert response.status_code == 200
    assert response.json()["accepted"]

    pending = client.get("/transactions/pending").json()
    assert any(p["tx_hash"] == tx.tx_hash for p in pending)


def test_sync_with_no_peers_returns_empty_results(client: TestClient):
    response = client.post("/network/sync", json={})
    assert response.status_code == 200
    assert response.json()["results"] == []


def test_sync_with_unreachable_peer_reports_failure(client: TestClient):
    client.post(
        "/network/peers/register",
        json={"node_id": "x", "address": "http://127.0.0.1:59999"},
    )
    response = client.post("/network/sync", json={})
    assert response.status_code == 200
    results = response.json()["results"]
    assert len(results) == 1
    assert not results[0]["accepted"]


def test_mine_via_api_broadcasts_without_error(client: TestClient):
    """
    Mining via the Phase 1 /mine endpoint should not fail even though no
    peers are reachable -- broadcast is best-effort.
    """
    wallet = client.post("/wallet/create").json()
    response = client.post("/mine", json={"miner_address": wallet["address"]})
    assert response.status_code == 200


def test_transaction_submit_via_api_broadcasts_without_error(client: TestClient):
    from blockchain.wallet import Wallet
    from blockchain.transaction import Transaction

    sender = client.post("/wallet/create").json()
    receiver = client.post("/wallet/create").json()
    client.post("/mine", json={"miner_address": sender["address"]})

    wallet = Wallet.from_private_key(sender["private_key"])
    tx = Transaction(sender=sender["address"], receiver=receiver["address"], amount=1.0)
    tx.sign(wallet)

    response = client.post(
        "/transaction/submit",
        json={
            "sender": tx.sender,
            "receiver": tx.receiver,
            "amount": tx.amount,
            "timestamp": tx.timestamp,
            "sender_public_key": tx.sender_public_key,
            "signature": tx.signature,
        },
    )
    assert response.status_code == 200
    assert response.json()["accepted"]
