"""
Multi-node integration test for SAYANJALI BLOCKCHAIN's P2P layer
(Phase 6.5: authenticated protocol).

This test launches two genuinely independent node processes (separate OS
processes, separate SQLite databases, real HTTP over localhost) rather
than simulating two nodes within one Python process. Phase 1's routing
layer uses simple module-level singletons rather than per-app dependency
injection, so two `NetworkNode` instances cannot safely coexist in one
process's module state -- real subprocesses sidestep that entirely and,
as a bonus, exercise the actual deployment topology described in
`README.md`'s "Running a Local Multi-Node Network" section.

Demonstrates, in order:
    1. Both nodes start (real, independent processes).
    2. Bidirectional discovery registration.
    3. Bidirectional cryptographic authentication (challenge-response
       handshake, exercising real network round trips, not simulated).
    4. Protocol negotiation is implicit in every authenticated call
       (network_name/genesis_hash/protocol_version are checked on every
       handshake and every propagated message).
    5. Transaction propagation, authenticated, A -> B.
    6. Block propagation, authenticated, A -> B.
    7. Explicit synchronization call.
    8. Chain convergence, verified via /network/chain on both sides.
    9. Reverse direction: B mines, A catches up.
    10. Adversarial checks against the live pair: an unauthenticated
        third party cannot propagate blocks/transactions or trigger
        sync; an unregistered/unauthenticated peer cannot spoof
        propagation; a replayed handshake is rejected.
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


def _run_cli(db_file: str, port: int, *args: str) -> subprocess.CompletedProcess:
    """
    Run a CLI command against a node's database, using the same
    advertised address the node's live process uses, so authentication
    performed via the CLI is visible to the live server (both read/write
    the same SQLite file).
    """
    env = os.environ.copy()
    env["SYJ_DB_FILE"] = db_file
    env["SYJ_ADVERTISED_ADDRESS"] = f"http://127.0.0.1:{port}"
    env["SYJ_DIFFICULTY"] = "1"
    env["SYJ_NETWORK_NAME"] = "sayanjali-test-net"
    env["SYJ_LOG_LEVEL"] = "WARNING"
    return subprocess.run(
        [sys.executable, "-m", "cli.main", *args],
        cwd=str(PROJECT_ROOT),
        env=env,
        capture_output=True,
        text=True,
        timeout=30,
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
        yield {
            "base_a": base_a, "base_b": base_b,
            "db_a": db_a, "db_b": db_b,
        }
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


def test_two_node_network_converges_with_authentication(two_nodes):
    base_a, base_b = two_nodes["base_a"], two_nodes["base_b"]
    db_a, db_b = two_nodes["db_a"], two_nodes["db_b"]

    # --- Nodes already started (fixture setup). ---
    status_a = httpx.get(f"{base_a}/network/status").json()
    status_b = httpx.get(f"{base_b}/network/status").json()
    assert status_a["node_id"] != status_b["node_id"]
    assert status_a["public_key"] != status_b["public_key"]
    assert status_a["chain_length"] == 1
    assert status_b["chain_length"] == 1

    # --- Bidirectional discovery registration + authentication via the
    # real CLI, exactly as an operator would run it. ---
    result_a_adds_b = _run_cli(db_a, PORT_A, "add-peer", base_b)
    assert "Authenticated" in result_a_adds_b.stdout, result_a_adds_b.stdout
    result_b_adds_a = _run_cli(db_b, PORT_B, "add-peer", base_a)
    assert "Authenticated" in result_b_adds_a.stdout, result_b_adds_a.stdout

    peers_a = httpx.get(f"{base_a}/network/peers").json()
    peers_b = httpx.get(f"{base_b}/network/peers").json()
    assert peers_a["count"] == 1 and peers_a["peers"][0]["trusted"]
    assert peers_b["count"] == 1 and peers_b["peers"][0]["trusted"]

    # --- Fund a wallet on A so it can send a transaction. ---
    sender = httpx.post(f"{base_a}/wallet/create").json()
    receiver = httpx.post(f"{base_b}/wallet/create").json()

    mine_response = httpx.post(f"{base_a}/mine", json={"miner_address": sender["address"]})
    assert mine_response.status_code == 200

    # Block propagation, A -> B, authenticated, triggered by /mine's
    # broadcast hook.
    converged = _wait_until(
        lambda: httpx.get(f"{base_b}/network/status").json()["chain_length"] == 2
    )
    assert converged, "Node B did not receive A's mined block."

    # --- A transaction is created and propagates, authenticated, A -> B. ---
    from blockchain.transaction import Transaction
    from blockchain.wallet import Wallet

    wallet = Wallet.from_private_key(sender["private_key"])
    tx = Transaction(sender=sender["address"], receiver=receiver["address"], amount=10.0)
    tx.sign(wallet)

    submit_response = httpx.post(
        f"{base_a}/transaction/submit",
        json={
            "sender": tx.sender, "receiver": tx.receiver, "amount": tx.amount_syj.__str__(), "amount_base_units": tx.amount_base_units,
            "timestamp": tx.timestamp, "sender_public_key": tx.sender_public_key,
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

    # --- A block is mined (confirming the tx) and propagates. ---
    mine_response_2 = httpx.post(f"{base_a}/mine", json={"miner_address": sender["address"]})
    assert mine_response_2.status_code == 200
    mined_block = mine_response_2.json()["block"]

    converged_2 = _wait_until(
        lambda: httpx.get(f"{base_b}/network/status").json()["chain_length"] == 3
    )
    assert converged_2, "Node B did not receive A's second mined block."

    chain_a = httpx.get(f"{base_a}/network/chain").json()
    chain_b = httpx.get(f"{base_b}/network/chain").json()
    assert chain_a["chain"][-1]["hash"] == chain_b["chain"][-1]["hash"] == mined_block["hash"]
    assert chain_a["work"] == chain_b["work"]

    receiver_balance = httpx.get(f"{base_b}/wallet/{receiver['address']}").json()
    assert receiver_balance["balance"] == "10"

    # --- Explicit authenticated sync call (CLI-driven, real HTTP). ---
    sync_result = _run_cli(db_b, PORT_B, "sync")
    assert sync_result.returncode == 0, sync_result.stdout + sync_result.stderr

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
    assert a_side_balance_for_b_miner["balance"] == "50"  # default block reward


def test_unauthenticated_third_party_cannot_propagate(two_nodes):
    """
    An adversarial third party (not a registered/authenticated peer of
    either node) must not be able to inject blocks or transactions into
    either node, or trigger synchronization, over real HTTP.
    """
    base_a = two_nodes["base_a"]

    block_response = httpx.post(
        f"{base_a}/network/blocks/receive",
        json={"block": {"index": 99}, "from_peer": "http://attacker.invalid"},
    )
    assert block_response.status_code == 200
    assert not block_response.json()["accepted"]

    tx_response = httpx.post(
        f"{base_a}/network/transactions/receive",
        json={"transaction": {}, "from_peer": "http://attacker.invalid"},
    )
    assert tx_response.status_code == 200
    assert not tx_response.json()["accepted"]

    sync_response = httpx.post(f"{base_a}/network/sync", json={})
    assert sync_response.status_code == 401

    status = httpx.get(f"{base_a}/network/status").json()
    assert status["chain_length"] == 1  # nothing was injected


def test_replayed_handshake_response_rejected(two_nodes):
    """
    A captured, previously-valid handshake response must not be
    replayable against the real live server -- the challenge is
    single-use and consumed on first success.
    """
    from blockchain.network.handshake import build_handshake_envelope, AuthContext
    from blockchain.network.identity import P2PIdentity

    base_a = two_nodes["base_a"]

    genesis_hash = httpx.get(f"{base_a}/network/chain").json()["chain"][0]["hash"]
    network_name = httpx.get(f"{base_a}/network/status").json()["network_name"]

    identity = P2PIdentity.generate("adversary-node")
    ctx = AuthContext(
        identity=identity,
        network_name=network_name,
        genesis_hash=genesis_hash,
        advertised_address="http://127.0.0.1:9999",
    )

    challenge = httpx.post(
        f"{base_a}/network/peers/challenge", json={"node_id": identity.node_id}
    ).json()["challenge"]
    envelope = build_handshake_envelope(ctx, challenge)

    first = httpx.post(f"{base_a}/network/peers/authenticate", json={"auth": envelope}).json()
    assert first["authenticated"], first

    replay = httpx.post(f"{base_a}/network/peers/authenticate", json={"auth": envelope}).json()
    assert not replay["authenticated"]
    assert "challenge" in replay["reason"].lower() or "expired" in replay["reason"].lower()


def test_unregistered_peer_address_rejected_for_sync(two_nodes):
    """
    Even an authenticated peer cannot direct a node to sync against an
    arbitrary third-party address that node hasn't itself registered --
    the SSRF-via-authenticated-caller closure verified against the real
    live server.
    """
    from blockchain.network.handshake import (
        AuthContext,
        build_auth_envelope,
        build_handshake_envelope,
    )
    from blockchain.network.identity import P2PIdentity

    base_a = two_nodes["base_a"]

    genesis_hash = httpx.get(f"{base_a}/network/chain").json()["chain"][0]["hash"]
    network_name = httpx.get(f"{base_a}/network/status").json()["network_name"]

    identity = P2PIdentity.generate("legit-authenticated-peer")
    ctx = AuthContext(
        identity=identity,
        network_name=network_name,
        genesis_hash=genesis_hash,
        advertised_address="http://127.0.0.1:9998",
    )
    challenge = httpx.post(
        f"{base_a}/network/peers/challenge", json={"node_id": identity.node_id}
    ).json()["challenge"]
    envelope = build_handshake_envelope(ctx, challenge)
    auth_result = httpx.post(
        f"{base_a}/network/peers/authenticate", json={"auth": envelope}
    ).json()
    assert auth_result["authenticated"]

    malicious_target = "http://10.0.0.1:9999"
    sync_envelope = build_auth_envelope(ctx, {"peer_address": malicious_target})
    sync_response = httpx.post(
        f"{base_a}/network/sync",
        json={"peer_address": malicious_target, "auth": sync_envelope},
    )
    assert sync_response.status_code == 403

PORT_C = 18973


@pytest.fixture
def three_nodes():
    """Launch three independent node processes for the Phase 2 acceptance path."""
    ports = (PORT_A, PORT_B, PORT_C)
    dbs = tuple(f"test_node_{name}_{uuid.uuid4().hex}.db" for name in ("a3", "b3", "c3"))
    bases = tuple(f"http://127.0.0.1:{port}" for port in ports)
    procs = [_launch_node(port, db) for port, db in zip(ports, dbs)]
    try:
        for base in bases:
            _wait_for_health(base)
        yield {"bases": bases, "ports": ports, "dbs": dbs}
    finally:
        for proc in procs:
            proc.terminate()
        for proc in procs:
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()
        for db_file in dbs:
            (PROJECT_ROOT / "database" / db_file).unlink(missing_ok=True)


def test_three_node_phase2_end_to_end(three_nodes):
    """Exercise authenticated A->B/A->C propagation and final convergence."""
    bases = three_nodes["bases"]
    ports = three_nodes["ports"]
    dbs = three_nodes["dbs"]
    base_a, base_b, base_c = bases

    assert all(httpx.get(f"{base}/network/status").json()["chain_length"] == 1 for base in bases)

    for peer_base in (base_b, base_c):
        result = _run_cli(dbs[0], ports[0], "add-peer", peer_base)
        assert "Authenticated" in result.stdout, result.stdout + result.stderr
    # Complete mutual trust so B and C will accept A's propagated blocks
    # and transactions.
    for idx, peer_base in ((1, base_a), (2, base_a)):
        result = _run_cli(dbs[idx], ports[idx], "add-peer", peer_base)
        assert "Authenticated" in result.stdout, result.stdout + result.stderr

    peers = httpx.get(f"{base_a}/network/peers").json()
    assert peers["count"] == 2
    assert all(peer["trusted"] for peer in peers["peers"])

    sender = httpx.post(f"{base_a}/wallet/create").json()
    receiver = httpx.post(f"{base_c}/wallet/create").json()

    mined = httpx.post(f"{base_a}/mine", json={"miner_address": sender["address"]})
    assert mined.status_code == 200

    assert _wait_until(lambda: httpx.get(f"{base_b}/network/status").json()["chain_length"] == 2)
    assert _wait_until(lambda: httpx.get(f"{base_c}/network/status").json()["chain_length"] == 2)

    from blockchain.transaction import Transaction
    from blockchain.wallet import Wallet

    wallet = Wallet.from_private_key(sender["private_key"])
    tx = Transaction(sender["address"], receiver["address"], "10")
    tx.sign(wallet)
    response = httpx.post(
        f"{base_a}/transaction/submit",
        json={**tx.to_dict(), "amount": tx.to_dict()["amount"]},
    )
    assert response.status_code == 200
    assert response.json()["accepted"], response.text

    assert _wait_until(lambda: any(
        item["tx_hash"] == tx.tx_hash
        for item in httpx.get(f"{base_b}/transactions/pending").json()
    ))
    assert _wait_until(lambda: any(
        item["tx_hash"] == tx.tx_hash
        for item in httpx.get(f"{base_c}/transactions/pending").json()
    ))

    mined2 = httpx.post(f"{base_a}/mine", json={"miner_address": sender["address"]})
    assert mined2.status_code == 200
    assert _wait_until(lambda: httpx.get(f"{base_b}/network/status").json()["chain_length"] == 3)
    assert _wait_until(lambda: httpx.get(f"{base_c}/network/status").json()["chain_length"] == 3)

    chains = [httpx.get(f"{base}/network/chain").json() for base in bases]
    assert len({c["chain"][-1]["hash"] for c in chains}) == 1
    assert len({c["work"] for c in chains}) == 1

    balances = [httpx.get(f"{base}/wallet/{receiver['address']}").json()["balance"] for base in bases]
    assert balances == ["10", "10", "10"]
    supplies = [
        sum(
            txd["amount_base_units"]
            for block in c["chain"]
            for txd in block["transactions"]
            if txd["sender"] == "SYJ-COINBASE-0000000000000000000000000000"
        )
        for c in chains
    ]
    assert supplies == [10_000_000_000, 10_000_000_000, 10_000_000_000]
