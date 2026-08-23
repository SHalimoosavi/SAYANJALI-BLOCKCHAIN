"""
Integration tests for api/routes.py using FastAPI's TestClient.

Each test resets the module-level blockchain singleton in api.routes so
that the isolated_settings fixture's fresh temporary database is actually
picked up (otherwise a Blockchain built for a previous test would linger).
"""

from __future__ import annotations

import pytest
from fastapi.testclient import TestClient


@pytest.fixture
def client():
    import api.routes as routes_module
    from api.main import app

    routes_module._blockchain = None  # force re-init against the test DB
    return TestClient(app)


def test_health_endpoint(client: TestClient):
    response = client.get("/health")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_status_endpoint(client: TestClient):
    response = client.get("/status")
    assert response.status_code == 200
    body = response.json()
    assert body["chain_length"] >= 1
    assert body["consensus_algorithm"] == "pow"


def test_get_chain(client: TestClient):
    response = client.get("/chain")
    assert response.status_code == 200
    body = response.json()
    assert body["length"] >= 1
    assert body["chain"][0]["index"] == 0


def test_get_block_not_found(client: TestClient):
    response = client.get("/block/9999")
    assert response.status_code == 404


def test_create_wallet(client: TestClient):
    response = client.post("/wallet/create")
    assert response.status_code == 200
    body = response.json()
    assert body["address"].startswith("SYJ")
    assert "private_key" in body


def test_wallet_balance_for_fresh_address(client: TestClient):
    wallet_response = client.post("/wallet/create").json()
    balance_response = client.get(f"/wallet/{wallet_response['address']}")
    assert balance_response.status_code == 200
    assert balance_response.json()["balance"] == 0.0


def test_full_transaction_and_mining_flow(client: TestClient):
    """
    Phase 6.5: POST /transaction/sign was removed -- private key material
    must never reach this API. The client obtains a canonical, timestamped
    envelope from /transaction/create, then signs it entirely locally
    (via Wallet/Transaction, exactly as the CLI's create-transaction
    command does) before submitting the already-signed result.
    """
    from blockchain.transaction import Transaction
    from blockchain.wallet import Wallet

    sender = client.post("/wallet/create").json()
    receiver = client.post("/wallet/create").json()

    # Fund the sender first via mining.
    mine_response = client.post("/mine", json={"miner_address": sender["address"]})
    assert mine_response.status_code == 200

    create_resp = client.post(
        "/transaction/create",
        json={
            "sender": sender["address"],
            "receiver": receiver["address"],
            "amount": 10.0,
        },
    ).json()

    # Client-side signing -- no private key ever leaves this process.
    wallet = Wallet.from_private_key(sender["private_key"])
    tx = Transaction(
        sender=sender["address"],
        receiver=receiver["address"],
        amount=10.0,
        timestamp=create_resp["timestamp"],
    )
    tx.sign(wallet)

    submit_resp = client.post(
        "/transaction/submit",
        json={
            "sender": tx.sender,
            "receiver": tx.receiver,
            "amount": tx.amount,
            "timestamp": tx.timestamp,
            "sender_public_key": tx.sender_public_key,
            "signature": tx.signature,
        },
    ).json()
    assert submit_resp["accepted"], submit_resp.get("reason")

    pending_resp = client.get("/transactions/pending")
    assert len(pending_resp.json()) == 1

    mine_again = client.post("/mine", json={"miner_address": sender["address"]})
    assert mine_again.status_code == 200

    receiver_balance = client.get(f"/wallet/{receiver['address']}").json()
    assert receiver_balance["balance"] == 10.0


def test_invalid_wallet_address_rejected(client: TestClient):
    response = client.get("/wallet/not-a-real-address")
    assert response.status_code == 400
