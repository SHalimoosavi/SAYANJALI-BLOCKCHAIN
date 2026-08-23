"""
Integration tests for api/network_routes.py using FastAPI's TestClient.

Phase 6.5: block/transaction propagation and sync now require
authentication. These tests exercise the full HTTP-level flow, including
establishing peer trust via `Storage.upsert_peer_credential` directly
(equivalent to what the real challenge-response handshake produces --
that handshake's own HTTP-level behavior is covered in
test_network_handshake_api.py) so each test can focus on the endpoint
behavior it's meant to verify.
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


def _trusted_peer_envelope(client: TestClient, address: str = "http://127.0.0.1:9001") -> dict:
    """
    Establish a trusted peer directly against the running app's
    NetworkNode and return a signed auth envelope usable to authenticate
    as that peer for a given operational payload via `_sign_for`.
    """
    from api.network_routes import get_network_node
    from blockchain.network.handshake import AuthContext
    from blockchain.network.identity import P2PIdentity

    node = get_network_node()
    identity = P2PIdentity.generate(f"peer-{address}")
    ctx = AuthContext(
        identity=identity,
        network_name=node.settings.network_name,
        genesis_hash=node.blockchain.genesis_block.hash,
        advertised_address=address,
    )
    node.blockchain.storage.upsert_peer_credential(
        address=address, node_id=identity.node_id, public_key_hex=identity.public_key_hex,
        trusted=True,
    )
    node.peers.register(address, identity.node_id)
    return ctx


def _sign_for(ctx, operational_payload: dict) -> dict:
    from blockchain.network.handshake import build_auth_envelope

    return build_auth_envelope(ctx, operational_payload)


def test_network_status(client: TestClient):
    response = client.get("/network/status")
    assert response.status_code == 200
    body = response.json()
    assert body["chain_length"] == 1
    assert body["peer_count"] == 0
    assert "node_id" in body
    assert "public_key" in body


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
    # Discovery registration alone must NOT grant trust.
    assert peers["peers"][0]["trusted"] is False


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


def test_receive_block_without_auth_rejected(client: TestClient):
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
    assert not response.json()["accepted"]
    status = client.get("/status").json()
    assert status["chain_length"] == 1  # unchanged


def test_receive_block_extends_chain(client: TestClient):
    from blockchain.mining import Miner

    import api.routes as routes_module

    blockchain = routes_module.get_blockchain()
    wallet_resp = client.post("/wallet/create").json()
    ctx = _trusted_peer_envelope(client)

    engine = Miner(blockchain.consensus, blockchain.settings.mining.block_reward)
    block, _ = engine.mine_block(
        index=blockchain.latest_block.index + 1,
        previous_hash=blockchain.latest_block.hash,
        mempool=blockchain.mempool,
        miner_address=wallet_resp["address"],
        difficulty=blockchain.settings.consensus.difficulty,
    )
    block_dict = block.to_dict()
    envelope = _sign_for(ctx, block_dict)

    response = client.post(
        "/network/blocks/receive",
        json={"block": block_dict, "from_peer": ctx.advertised_address, "auth": envelope},
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
    ctx = _trusted_peer_envelope(client)

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
        "/network/blocks/receive",
        json={"block": block_dict, "from_peer": ctx.advertised_address, "auth": _sign_for(ctx, block_dict)},
    )
    assert first.json()["accepted"]

    second = client.post(
        "/network/blocks/receive",
        json={"block": block_dict, "from_peer": ctx.advertised_address, "auth": _sign_for(ctx, block_dict)},
    )
    assert not second.json()["accepted"]


def test_receive_transaction_without_auth_rejected(client: TestClient):
    from blockchain.transaction import Transaction
    from blockchain.wallet import Wallet

    sender = client.post("/wallet/create").json()
    receiver = client.post("/wallet/create").json()
    client.post("/mine", json={"miner_address": sender["address"]})

    wallet = Wallet.from_private_key(sender["private_key"])
    tx = Transaction(sender=sender["address"], receiver=receiver["address"], amount=1.0)
    tx.sign(wallet)

    response = client.post(
        "/network/transactions/receive",
        json={"transaction": tx.to_dict(), "from_peer": "http://peer"},
    )
    assert response.status_code == 200
    assert not response.json()["accepted"]
    pending = client.get("/transactions/pending").json()
    assert not any(p["tx_hash"] == tx.tx_hash for p in pending)


def test_receive_transaction_enters_mempool(client: TestClient):
    from blockchain.transaction import Transaction
    from blockchain.wallet import Wallet

    sender = client.post("/wallet/create").json()
    receiver = client.post("/wallet/create").json()
    client.post("/mine", json={"miner_address": sender["address"]})
    ctx = _trusted_peer_envelope(client)

    wallet = Wallet.from_private_key(sender["private_key"])
    tx = Transaction(sender=sender["address"], receiver=receiver["address"], amount=1.0)
    tx.sign(wallet)
    tx_dict = tx.to_dict()

    response = client.post(
        "/network/transactions/receive",
        json={"transaction": tx_dict, "from_peer": ctx.advertised_address, "auth": _sign_for(ctx, tx_dict)},
    )
    assert response.status_code == 200
    assert response.json()["accepted"]

    pending = client.get("/transactions/pending").json()
    assert any(p["tx_hash"] == tx.tx_hash for p in pending)


def test_sync_without_auth_rejected(client: TestClient):
    response = client.post("/network/sync", json={})
    assert response.status_code == 401


def test_sync_with_authenticated_peer_attempts_known_peers(client: TestClient):
    """
    Establishing trust also registers the peer for discovery (matching
    real handshake behavior), so a subsequent "sync from all known
    peers" call legitimately attempts that peer -- and reports failure
    since it isn't a real running server in this test.
    """
    ctx = _trusted_peer_envelope(client)
    envelope = _sign_for(ctx, {"peer_address": None})
    response = client.post("/network/sync", json={"auth": envelope})
    assert response.status_code == 200
    results = response.json()["results"]
    assert len(results) == 1
    assert not results[0]["accepted"]


def test_sync_rejects_unregistered_peer_address(client: TestClient):
    ctx = _trusted_peer_envelope(client)
    envelope = _sign_for(ctx, {"peer_address": "http://127.0.0.1:59999"})
    response = client.post(
        "/network/sync", json={"peer_address": "http://127.0.0.1:59999", "auth": envelope}
    )
    assert response.status_code == 403


def test_sync_with_registered_unreachable_peer_reports_failure(client: TestClient):
    ctx = _trusted_peer_envelope(client, address="http://127.0.0.1:9001")
    # Also register a second (target) peer this authenticated caller may sync against.
    client.post(
        "/network/peers/register",
        json={"node_id": "x", "address": "http://127.0.0.1:59999"},
    )
    envelope = _sign_for(ctx, {"peer_address": "http://127.0.0.1:59999"})
    response = client.post(
        "/network/sync", json={"peer_address": "http://127.0.0.1:59999", "auth": envelope}
    )
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


def test_transaction_sign_endpoint_no_longer_exists(client: TestClient):
    """
    Phase 6.5: /transaction/sign was removed entirely -- private key
    material must never reach this API by any documented path.
    """
    response = client.post(
        "/transaction/sign",
        json={
            "sender": "SYJ" + "0" * 40,
            "receiver": "SYJ" + "1" * 40,
            "amount": 1.0,
            "timestamp": 123.0,
            "private_key": "00" * 32,
        },
    )
    assert response.status_code == 404
