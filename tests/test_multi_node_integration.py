"""
Multi-node integration test for SAYANJALI BLOCKCHAIN's P2P layer.

This test launches two genuinely independent node processes (separate OS
processes, separate SQLite databases, real HTTP over localhost) rather
than simulating two nodes within one Python process. Phase 1's routing
layer uses simple module-level singletons rather than per-app dependency
injection, so two `NetworkNode` instances cannot safely coexist in one
process's module state -- real subprocesses sidestep that entirely and,
as a bonus, exercise the actual deployment topology described in
`README.md`'s "Running a Local Multi-Node Network" section:

    Node A -> 127.0.0.1:<port A>
    Node B -> 127.0.0.1:<port B>

Demonstrates, in order: both nodes start, mutual peer registration,
transaction propagation, block propagation, and chain convergence -- in
both directions (A mines and B catches up, then B mines and A catches
up).
"""

from __future__ import annotations

import os
import subprocess
import sys
import time
import uuid
from pathlib import Path

import httpx
import pytest

PROJECT_ROOT = Path(__file__).resolve().parent.parent
PORT_A = 18971
PORT_B = 18972
STARTUP_TIMEOUT_SECONDS = 15
POLL_INTERVAL_SECONDS = 0.2


def _wait_for_health(base_url: str, timeout: float = STARTUP_TIMEOUT_SECONDS) -> None:
    """Poll a node's /health endpoint until it responds or times out."""
    deadline = time.time() + timeout
    last_error = None
    while time.time() < deadline:
        try:
            response = httpx.get(f"{base_url}/health", timeout=1.0)
            if response.status_code == 200:
                return
        except httpx.HTTPError as exc:
            last_error = exc
        time.sleep(POLL_INTERVAL_SECONDS)
    raise TimeoutError(f"Node at {base_url} did not become healthy: {last_error}")


def _wait_until(predicate, timeout: float = 10.0, interval: float = POLL_INTERVAL_SECONDS) -> bool:
    """Poll `predicate` (a zero-arg callable returning bool) until True or timeout."""
    deadline = time.time() + timeout
    while time.time() < deadline:
        if predicate():
            return True
        time.sleep(interval)
    return False


def _launch_node(port: int, db_file: str) -> subprocess.Popen:
    """Start a node as a real subprocess, isolated by its own SQLite file."""
    env = os.environ.copy()
    env["SYJ_DB_FILE"] = db_file
    env["SYJ_PORT"] = str(port)
    env["SYJ_HOST"] = "127.0.0.1"
    env["SYJ_ADVERTISED_ADDRESS"] = f"http://127.0.0.1:{port}"
    env["SYJ_DIFFICULTY"] = "1"
    env["SYJ_NETWORK_NAME"] = "sayanjali-test-net"
    env["SYJ_LOG_LEVEL"] = "WARNING"

    return subprocess.Popen(
        [
            sys.executable, "-m", "uvicorn", "api.main:app",
            "--host", "127.0.0.1", "--port", str(port), "--log-level", "warning",
        ],
        cwd=str(PROJECT_ROOT),
        env=env,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )


@pytest.fixture
def two_nodes():
    """
    Launch two independent SAYANJALI BLOCKCHAIN node processes and tear
    them down (and their databases) afterward, regardless of test outcome.
    """
    db_a = f"test_node_a_{uuid.uuid4().hex}.db"
    db_b = f"test_node_b_{uuid.uuid4().hex}.db"
    base_a = f"http://127.0.0.1:{PORT_A}"
    base_b = f"http://127.0.0.1:{PORT_B}"

    proc_a = _launch_node(PORT_A, db_a)
    proc_b = _launch_node(PORT_B, db_b)
    try:
        _wait_for_health(base_a)
        _wait_for_health(base_b)
        yield base_a, base_b
    finally:
        proc_a.terminate()
        proc_b.terminate()
        try:
            proc_a.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc_a.kill()
        try:
            proc_b.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc_b.kill()

        for db_file in (db_a, db_b):
            db_path = PROJECT_ROOT / "database" / db_file
            db_path.unlink(missing_ok=True)


def test_two_node_network_converges_in_both_directions(two_nodes):
    base_a, base_b = two_nodes

    # --- Step 1 & 2: both nodes already started (fixture setup). ---
    status_a = httpx.get(f"{base_a}/network/status").json()
    status_b = httpx.get(f"{base_b}/network/status").json()
    assert status_a["node_id"] != status_b["node_id"]
    assert status_a["chain_length"] == 1
    assert status_b["chain_length"] == 1

    # --- Step 3: nodes discover/register each other (bidirectional). ---
    reg_a_to_b = httpx.post(
        f"{base_a}/network/peers/register",
        json={"node_id": status_b["node_id"], "address": base_b},
    ).json()
    assert reg_a_to_b["accepted"], reg_a_to_b

    reg_b_to_a = httpx.post(
        f"{base_b}/network/peers/register",
        json={"node_id": status_a["node_id"], "address": base_a},
    ).json()
    assert reg_b_to_a["accepted"], reg_b_to_a

    peers_a = httpx.get(f"{base_a}/network/peers").json()
    peers_b = httpx.get(f"{base_b}/network/peers").json()
    assert peers_a["count"] == 1
    assert peers_b["count"] == 1

    # --- Fund a wallet on A so it can send a transaction. ---
    sender = httpx.post(f"{base_a}/wallet/create").json()
    receiver = httpx.post(f"{base_b}/wallet/create").json()

    mine_response = httpx.post(f"{base_a}/mine", json={"miner_address": sender["address"]})
    assert mine_response.status_code == 200

    # Block propagation, A -> B: B should pick up the new block via the
    # broadcast triggered inside A's /mine handler.
    converged = _wait_until(
        lambda: httpx.get(f"{base_b}/network/status").json()["chain_length"] == 2
    )
    assert converged, "Node B did not receive A's mined block."

    # --- Step 4 & 5: a transaction is created and propagates, A -> B. ---
    from blockchain.transaction import Transaction
    from blockchain.wallet import Wallet

    wallet = Wallet.from_private_key(sender["private_key"])
    tx = Transaction(sender=sender["address"], receiver=receiver["address"], amount=10.0)
    tx.sign(wallet)

    submit_response = httpx.post(
        f"{base_a}/transaction/submit",
        json={
            "sender": tx.sender,
            "receiver": tx.receiver,
            "amount": tx.amount,
            "timestamp": tx.timestamp,
            "sender_public_key": tx.sender_public_key,
            "signature": tx.signature,
        },
    )
    assert submit_response.status_code == 200
    assert submit_response.json()["accepted"]

    tx_propagated = _wait_until(
        lambda: any(
            p["tx_hash"] == tx.tx_hash
            for p in httpx.get(f"{base_b}/transactions/pending").json()
        )
    )
    assert tx_propagated, "Transaction did not propagate from A to B."

    # --- Step 6, 7, 8: a block is mined (confirming the tx) and propagates. ---
    mine_response_2 = httpx.post(f"{base_a}/mine", json={"miner_address": sender["address"]})
    assert mine_response_2.status_code == 200
    mined_block = mine_response_2.json()["block"]

    # --- Step 9: both nodes converge on the same chain. ---
    converged_2 = _wait_until(
        lambda: httpx.get(f"{base_b}/network/status").json()["chain_length"] == 3
    )
    assert converged_2, "Node B did not receive A's second mined block."

    chain_a = httpx.get(f"{base_a}/network/chain").json()
    chain_b = httpx.get(f"{base_b}/network/chain").json()
    assert chain_a["chain"][-1]["hash"] == chain_b["chain"][-1]["hash"] == mined_block["hash"]
    assert chain_a["work"] == chain_b["work"]

    receiver_balance = httpx.get(f"{base_b}/wallet/{receiver['address']}").json()
    assert receiver_balance["balance"] == 10.0

    # --- Reverse direction: B mines, A must catch up. ---
    b_miner = httpx.post(f"{base_b}/wallet/create").json()
    mine_on_b = httpx.post(f"{base_b}/mine", json={"miner_address": b_miner["address"]})
    assert mine_on_b.status_code == 200

    converged_reverse = _wait_until(
        lambda: httpx.get(f"{base_a}/network/status").json()["chain_length"] == 4
    )
    assert converged_reverse, "Node A did not receive B's mined block."

    chain_a_final = httpx.get(f"{base_a}/network/chain").json()
    chain_b_final = httpx.get(f"{base_b}/network/chain").json()
    assert chain_a_final["chain"][-1]["hash"] == chain_b_final["chain"][-1]["hash"]

    a_side_balance_for_b_miner = httpx.get(f"{base_a}/wallet/{b_miner['address']}").json()
    assert a_side_balance_for_b_miner["balance"] == 50.0  # default block reward
