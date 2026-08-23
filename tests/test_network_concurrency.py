"""
Concurrency tests for SAYANJALI BLOCKCHAIN's P2P layer.

Verifies the Phase 6.5 fix for the propagation race window: FastAPI runs
synchronous route handlers in a threadpool, so concurrent requests can
genuinely execute against the same `Blockchain`/`NetworkNode` instance on
separate threads. These tests drive that concurrency directly (bypassing
HTTP, calling `propagation.receive_block`/`receive_transaction` from
multiple real threads at once) to confirm `Blockchain.mutation_lock`
actually serializes the critical section: concurrent duplicate delivery
must result in exactly one acceptance, no duplicate persistence, no
corrupted chain state, and no unhandled exceptions.
"""

from __future__ import annotations

import threading

from blockchain.blockchain import Blockchain
from blockchain.mining import Miner
from blockchain.network.handshake import AuthContext, build_auth_envelope
from blockchain.network.identity import P2PIdentity
from blockchain.network.node import NetworkNode
from blockchain.network.propagation import receive_block, receive_transaction
from blockchain.transaction import Transaction
from blockchain.wallet import Wallet

PEER_ADDRESS = "http://127.0.0.1:9001"


def _make_trusted_peer(node: NetworkNode) -> AuthContext:
    identity = P2PIdentity.generate("peer-concurrency")
    ctx = AuthContext(
        identity=identity,
        network_name=node.settings.network_name,
        genesis_hash=node.blockchain.genesis_block.hash,
        advertised_address=PEER_ADDRESS,
    )
    node.blockchain.storage.upsert_peer_credential(
        address=PEER_ADDRESS, node_id=identity.node_id,
        public_key_hex=identity.public_key_hex, trusted=True,
    )
    return ctx


def test_concurrent_duplicate_block_delivery_is_safe(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    peer_ctx = _make_trusted_peer(node)

    miner = Wallet.create()
    engine = Miner(blockchain.consensus, blockchain.settings.mining.block_reward)
    block, _ = engine.mine_block(
        index=blockchain.latest_block.index + 1,
        previous_hash=blockchain.latest_block.hash,
        mempool=blockchain.mempool,
        miner_address=miner.address,
        difficulty=1,
    )
    block_dict = block.to_dict()

    results: list[tuple[bool, str, bool]] = []
    errors: list[Exception] = []
    results_lock = threading.Lock()

    def deliver():
        try:
            # Each thread signs its own fresh envelope over the identical
            # block payload -- simulating several near-simultaneous
            # propagation deliveries of the same block from the same
            # trusted peer (e.g. via slightly different relay paths).
            envelope = build_auth_envelope(peer_ctx, block_dict)
            result = receive_block(node, block_dict, PEER_ADDRESS, envelope)
            with results_lock:
                results.append(result)
        except Exception as exc:  # noqa: BLE001
            with results_lock:
                errors.append(exc)

    threads = [threading.Thread(target=deliver) for _ in range(10)]
    for t in threads:
        t.start()
    for t in threads:
        t.join(timeout=10)

    assert not errors, f"Unhandled exceptions during concurrent delivery: {errors}"
    assert len(results) == 10

    accepted_count = sum(1 for accepted, _, _ in results if accepted)
    assert accepted_count == 1, f"Expected exactly one acceptance, got {accepted_count}"

    # Chain state must be consistent: exactly one new block, no duplicates.
    assert blockchain.length == 2
    assert blockchain.latest_block.hash == block.hash

    # Storage must agree with in-memory state (no duplicate rows, no
    # partial/corrupted persistence from a lost race).
    reloaded = blockchain.storage.load_chain()
    assert len(reloaded) == 2
    assert reloaded[-1].hash == block.hash


def test_concurrent_duplicate_transaction_delivery_is_safe(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    peer_ctx = _make_trusted_peer(node)

    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)  # fund the sender

    receiver = Wallet.create()
    tx = Transaction(sender=miner.address, receiver=receiver.address, amount=5.0)
    tx.sign(miner)
    tx_dict = tx.to_dict()

    results: list[tuple[bool, str, bool]] = []
    errors: list[Exception] = []
    results_lock = threading.Lock()

    def deliver():
        try:
            envelope = build_auth_envelope(peer_ctx, tx_dict)
            result = receive_transaction(node, tx_dict, PEER_ADDRESS, envelope)
            with results_lock:
                results.append(result)
        except Exception as exc:  # noqa: BLE001
            with results_lock:
                errors.append(exc)

    threads = [threading.Thread(target=deliver) for _ in range(10)]
    for t in threads:
        t.start()
    for t in threads:
        t.join(timeout=10)

    assert not errors, f"Unhandled exceptions during concurrent delivery: {errors}"
    assert len(results) == 10

    accepted_count = sum(1 for accepted, _, _ in results if accepted)
    assert accepted_count == 1, f"Expected exactly one acceptance, got {accepted_count}"

    # Mempool must contain exactly one copy of the transaction, not ten.
    pending = blockchain.mempool.get_pending()
    matching = [t for t in pending if t.tx_hash == tx.tx_hash]
    assert len(matching) == 1


def test_concurrent_mining_and_propagation_do_not_corrupt_chain(blockchain: Blockchain):
    """
    A block arriving via propagation at the same moment this node is
    independently mining its own block must not corrupt chain state --
    exactly one of the two extends the tip; the loser is cleanly rejected
    (by validation, under the lock), not silently double-applied.
    """
    node = NetworkNode(blockchain)
    peer_ctx = _make_trusted_peer(node)

    local_miner = Wallet.create()
    remote_miner = Wallet.create()

    engine = Miner(blockchain.consensus, blockchain.settings.mining.block_reward)
    remote_block, _ = engine.mine_block(
        index=blockchain.latest_block.index + 1,
        previous_hash=blockchain.latest_block.hash,
        mempool=blockchain.mempool,
        miner_address=remote_miner.address,
        difficulty=1,
    )
    remote_block_dict = remote_block.to_dict()
    envelope = build_auth_envelope(peer_ctx, remote_block_dict)

    errors: list[Exception] = []
    mine_result = {}

    def mine_locally():
        try:
            mine_result["block"] = blockchain.mine_pending_transactions(local_miner.address)
        except Exception as exc:  # noqa: BLE001
            mine_result["error"] = exc

    def deliver_remote():
        try:
            receive_block(node, remote_block_dict, PEER_ADDRESS, envelope)
        except Exception as exc:  # noqa: BLE001
            errors.append(exc)

    t1 = threading.Thread(target=mine_locally)
    t2 = threading.Thread(target=deliver_remote)
    t1.start()
    t2.start()
    t1.join(timeout=10)
    t2.join(timeout=10)

    assert not errors, f"Unhandled exceptions: {errors}"
    # Exactly one block was appended at index 1 -- chain length is 2, not 3,
    # and the chain is internally consistent regardless of which one won.
    assert blockchain.length == 2
    is_valid, reason = blockchain.is_chain_valid()
    assert is_valid, reason
