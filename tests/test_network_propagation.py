"""
Tests for blockchain/network/propagation.py.

Phase 6.5: block and transaction propagation now requires the sender to
present a valid, trusted auth envelope. These tests establish trust
directly via `Storage.upsert_peer_credential` (the same effect the real
challenge-response handshake produces) so they can focus on
propagation.py's own logic without re-testing the handshake itself,
which has its own dedicated test file.
"""

from __future__ import annotations

from blockchain.blockchain import Blockchain
from blockchain.mining import Miner
from blockchain.network.handshake import AuthContext, build_auth_envelope
from blockchain.network.identity import P2PIdentity
from blockchain.network.node import NetworkNode
from blockchain.network.propagation import (
    authenticate_request,
    receive_block,
    receive_transaction,
)
from blockchain.transaction import Transaction
from blockchain.wallet import Wallet

PEER_ADDRESS = "http://127.0.0.1:9001"


def _mine_block_dict(blockchain: Blockchain, miner_address: str) -> tuple:
    """Mine one block directly (bypassing the API/CLI) and return its wire dict."""
    engine = Miner(blockchain.consensus, blockchain.settings.mining.block_reward)
    block, included = engine.mine_block(
        index=blockchain.latest_block.index + 1,
        previous_hash=blockchain.latest_block.hash,
        mempool=blockchain.mempool,
        miner_address=miner_address,
        difficulty=1,
    )
    return block, block.to_dict()


def _make_trusted_peer(node: NetworkNode, address: str = PEER_ADDRESS) -> AuthContext:
    """
    Generate a fresh peer identity, establish it as trusted on `node`
    (as if it had already completed the challenge-response handshake),
    and return an AuthContext usable to sign requests as that peer.
    """
    identity = P2PIdentity.generate(f"peer-{address}")
    peer_ctx = AuthContext(
        identity=identity,
        network_name=node.settings.network_name,
        genesis_hash=node.blockchain.genesis_block.hash,
        advertised_address=address,
    )
    node.blockchain.storage.upsert_peer_credential(
        address=address,
        node_id=identity.node_id,
        public_key_hex=identity.public_key_hex,
        trusted=True,
    )
    return peer_ctx


# --------------------------------------------------------------------- #
# Authentication gate
# --------------------------------------------------------------------- #


def test_authenticate_request_rejects_missing_envelope(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    ok, reason = authenticate_request(node, None, {}, PEER_ADDRESS)
    assert not ok
    assert "required" in reason.lower()


def test_authenticate_request_rejects_untrusted_peer(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    identity = P2PIdentity.generate("never-authenticated")
    ctx = AuthContext(
        identity=identity,
        network_name=node.settings.network_name,
        genesis_hash=node.blockchain.genesis_block.hash,
        advertised_address=PEER_ADDRESS,
    )
    envelope = build_auth_envelope(ctx, {})
    ok, reason = authenticate_request(node, envelope, {}, PEER_ADDRESS)
    assert not ok
    assert "not a trusted peer" in reason.lower()


def test_authenticate_request_accepts_trusted_peer(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    peer_ctx = _make_trusted_peer(node)
    envelope = build_auth_envelope(peer_ctx, {})
    ok, reason = authenticate_request(node, envelope, {}, PEER_ADDRESS)
    assert ok, reason


def test_authenticate_request_rejects_invalid_signature(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    peer_ctx = _make_trusted_peer(node)
    envelope = build_auth_envelope(peer_ctx, {})
    envelope["signature"] = "ff" * 64
    ok, reason = authenticate_request(node, envelope, {}, PEER_ADDRESS)
    assert not ok
    assert "signature" in reason.lower()


def test_authenticate_request_rejects_replay(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    peer_ctx = _make_trusted_peer(node)
    envelope = build_auth_envelope(peer_ctx, {})
    ok1, _ = authenticate_request(node, envelope, {}, PEER_ADDRESS)
    assert ok1
    ok2, reason2 = authenticate_request(node, envelope, {}, PEER_ADDRESS)
    assert not ok2
    assert "replay" in reason2.lower()


def test_authenticate_request_rejects_identity_mismatch(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    _make_trusted_peer(node)  # trusted under one identity

    # A different identity now claims the same address.
    impostor_identity = P2PIdentity.generate("impostor")
    impostor_ctx = AuthContext(
        identity=impostor_identity,
        network_name=node.settings.network_name,
        genesis_hash=node.blockchain.genesis_block.hash,
        advertised_address=PEER_ADDRESS,
    )
    envelope = build_auth_envelope(impostor_ctx, {})
    ok, reason = authenticate_request(node, envelope, {}, PEER_ADDRESS)
    assert not ok
    assert "mismatch" in reason.lower()


# --------------------------------------------------------------------- #
# Block propagation
# --------------------------------------------------------------------- #


def test_receive_valid_block_extends_chain(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    peer_ctx = _make_trusted_peer(node)
    miner = Wallet.create()
    block, block_dict = _mine_block_dict(blockchain, miner.address)
    envelope = build_auth_envelope(peer_ctx, block_dict)

    accepted, reason, rebroadcast = receive_block(node, block_dict, PEER_ADDRESS, envelope)
    assert accepted, reason
    assert rebroadcast
    assert blockchain.latest_block.hash == block.hash
    assert blockchain.get_balance(miner.address) == blockchain.settings.mining.block_reward


def test_receive_block_without_auth_rejected(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    _make_trusted_peer(node)
    miner = Wallet.create()
    _, block_dict = _mine_block_dict(blockchain, miner.address)

    accepted, reason, rebroadcast = receive_block(node, block_dict, PEER_ADDRESS, None)
    assert not accepted
    assert not rebroadcast
    assert blockchain.length == 1


def test_receive_duplicate_block_rejected(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    peer_ctx = _make_trusted_peer(node)
    miner = Wallet.create()
    _, block_dict = _mine_block_dict(blockchain, miner.address)

    envelope1 = build_auth_envelope(peer_ctx, block_dict)
    accepted1, _, _ = receive_block(node, block_dict, PEER_ADDRESS, envelope1)
    assert accepted1

    envelope2 = build_auth_envelope(peer_ctx, block_dict)
    accepted2, reason2, rebroadcast2 = receive_block(node, block_dict, PEER_ADDRESS, envelope2)
    assert not accepted2
    assert not rebroadcast2
    assert "duplicate" in reason2.lower()


def test_receive_invalid_block_rejected(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    peer_ctx = _make_trusted_peer(node)
    miner = Wallet.create()
    block, block_dict = _mine_block_dict(blockchain, miner.address)
    block_dict["hash"] = "0" * 64  # corrupt: won't match recomputed hash
    envelope = build_auth_envelope(peer_ctx, block_dict)

    accepted, reason, rebroadcast = receive_block(node, block_dict, PEER_ADDRESS, envelope)
    assert not accepted
    assert not rebroadcast
    assert blockchain.length == 1  # unchanged


def test_receive_malformed_block_payload_rejected(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    peer_ctx = _make_trusted_peer(node)
    bad_payload = {"not": "a block"}
    envelope = build_auth_envelope(peer_ctx, bad_payload)

    accepted, reason, rebroadcast = receive_block(node, bad_payload, PEER_ADDRESS, envelope)
    assert not accepted
    assert not rebroadcast
    assert "malformed" in reason.lower()


def test_receive_block_that_does_not_extend_tip_rejected(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    peer_ctx = _make_trusted_peer(node)
    miner = Wallet.create()
    engine = Miner(blockchain.consensus, blockchain.settings.mining.block_reward)
    far_block, _ = engine.mine_block(
        index=5,
        previous_hash="ab" * 32,
        mempool=blockchain.mempool,
        miner_address=miner.address,
        difficulty=1,
    )
    far_dict = far_block.to_dict()
    envelope = build_auth_envelope(peer_ctx, far_dict)

    accepted, reason, rebroadcast = receive_block(node, far_dict, PEER_ADDRESS, envelope)
    assert not accepted
    assert "sync" in reason.lower()


# --------------------------------------------------------------------- #
# Transaction propagation
# --------------------------------------------------------------------- #


def test_receive_valid_transaction_enters_mempool(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    peer_ctx = _make_trusted_peer(node)
    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)  # fund the sender

    receiver = Wallet.create()
    tx = Transaction(sender=miner.address, receiver=receiver.address, amount=5.0)
    tx.sign(miner)
    envelope = build_auth_envelope(peer_ctx, tx.to_dict())

    accepted, reason, rebroadcast = receive_transaction(node, tx.to_dict(), PEER_ADDRESS, envelope)
    assert accepted, reason
    assert rebroadcast
    assert blockchain.mempool.contains(tx.tx_hash)


def test_receive_transaction_without_auth_rejected(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    _make_trusted_peer(node)
    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)
    receiver = Wallet.create()
    tx = Transaction(sender=miner.address, receiver=receiver.address, amount=5.0)
    tx.sign(miner)

    accepted, reason, rebroadcast = receive_transaction(node, tx.to_dict(), PEER_ADDRESS, None)
    assert not accepted
    assert not blockchain.mempool.contains(tx.tx_hash)


def test_receive_duplicate_transaction_rejected(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    peer_ctx = _make_trusted_peer(node)
    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)

    receiver = Wallet.create()
    tx = Transaction(sender=miner.address, receiver=receiver.address, amount=5.0)
    tx.sign(miner)

    envelope1 = build_auth_envelope(peer_ctx, tx.to_dict())
    accepted1, _, _ = receive_transaction(node, tx.to_dict(), PEER_ADDRESS, envelope1)
    assert accepted1

    envelope2 = build_auth_envelope(peer_ctx, tx.to_dict())
    accepted2, reason2, rebroadcast2 = receive_transaction(node, tx.to_dict(), PEER_ADDRESS, envelope2)
    assert not accepted2
    assert not rebroadcast2


def test_receive_invalid_transaction_rejected(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    peer_ctx = _make_trusted_peer(node)
    miner = Wallet.create()
    receiver = Wallet.create()
    tx = Transaction(sender=miner.address, receiver=receiver.address, amount=5.0)
    tx.sign(miner)
    tx_dict = tx.to_dict()
    tx_dict["amount"] = 999999.0  # tamper post-signature
    envelope = build_auth_envelope(peer_ctx, tx_dict)

    accepted, reason, rebroadcast = receive_transaction(node, tx_dict, PEER_ADDRESS, envelope)
    assert not accepted
    assert not rebroadcast
    assert not blockchain.mempool.contains(tx_dict["tx_hash"])


def test_receive_malformed_transaction_payload_rejected(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    peer_ctx = _make_trusted_peer(node)
    bad_payload = {"not": "a tx"}
    envelope = build_auth_envelope(peer_ctx, bad_payload)

    accepted, reason, rebroadcast = receive_transaction(node, bad_payload, PEER_ADDRESS, envelope)
    assert not accepted
    assert "malformed" in reason.lower()
