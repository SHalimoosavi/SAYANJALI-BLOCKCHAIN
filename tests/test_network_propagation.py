"""Tests for blockchain/network/propagation.py."""

from __future__ import annotations

from blockchain.blockchain import Blockchain
from blockchain.mining import Miner
from blockchain.network.node import NetworkNode
from blockchain.network.propagation import receive_block, receive_transaction
from blockchain.transaction import Transaction
from blockchain.wallet import Wallet


def _mine_block_dict(blockchain: Blockchain, miner_address: str) -> dict:
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


def test_receive_valid_block_extends_chain(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    miner = Wallet.create()
    block, block_dict = _mine_block_dict(blockchain, miner.address)

    accepted, reason, rebroadcast = receive_block(node, block_dict, "http://peer")
    assert accepted, reason
    assert rebroadcast
    assert blockchain.latest_block.hash == block.hash
    assert blockchain.get_balance(miner.address) == blockchain.settings.mining.block_reward


def test_receive_duplicate_block_rejected(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    miner = Wallet.create()
    _, block_dict = _mine_block_dict(blockchain, miner.address)

    accepted1, _, _ = receive_block(node, block_dict, "http://peer")
    assert accepted1

    accepted2, reason2, rebroadcast2 = receive_block(node, block_dict, "http://peer")
    assert not accepted2
    assert not rebroadcast2
    assert "duplicate" in reason2.lower()


def test_receive_invalid_block_rejected(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    miner = Wallet.create()
    block, block_dict = _mine_block_dict(blockchain, miner.address)
    block_dict["hash"] = "0" * 64  # corrupt: won't match recomputed hash

    accepted, reason, rebroadcast = receive_block(node, block_dict, "http://peer")
    assert not accepted
    assert not rebroadcast
    assert blockchain.length == 1  # unchanged


def test_receive_malformed_block_payload_rejected(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    accepted, reason, rebroadcast = receive_block(node, {"not": "a block"}, "http://peer")
    assert not accepted
    assert not rebroadcast
    assert "malformed" in reason.lower()


def test_receive_block_that_does_not_extend_tip_rejected(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    miner = Wallet.create()
    engine = Miner(blockchain.consensus, blockchain.settings.mining.block_reward)
    # Mine a block at index 5, far ahead of the current tip (index 0).
    far_block, _ = engine.mine_block(
        index=5,
        previous_hash="ab" * 32,
        mempool=blockchain.mempool,
        miner_address=miner.address,
        difficulty=1,
    )
    accepted, reason, rebroadcast = receive_block(node, far_block.to_dict(), "http://peer")
    assert not accepted
    assert "sync" in reason.lower()


def test_receive_valid_transaction_enters_mempool(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)  # fund the sender

    receiver = Wallet.create()
    tx = Transaction(sender=miner.address, receiver=receiver.address, amount=5.0)
    tx.sign(miner)

    accepted, reason, rebroadcast = receive_transaction(node, tx.to_dict(), "http://peer")
    assert accepted, reason
    assert rebroadcast
    assert blockchain.mempool.contains(tx.tx_hash)


def test_receive_duplicate_transaction_rejected(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)

    receiver = Wallet.create()
    tx = Transaction(sender=miner.address, receiver=receiver.address, amount=5.0)
    tx.sign(miner)

    accepted1, _, _ = receive_transaction(node, tx.to_dict(), "http://peer")
    assert accepted1

    accepted2, reason2, rebroadcast2 = receive_transaction(node, tx.to_dict(), "http://peer")
    assert not accepted2
    assert not rebroadcast2


def test_receive_invalid_transaction_rejected(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    miner = Wallet.create()
    receiver = Wallet.create()
    tx = Transaction(sender=miner.address, receiver=receiver.address, amount=5.0)
    tx.sign(miner)
    tx_dict = tx.to_dict()
    tx_dict["amount"] = 999999.0  # tamper post-signature

    accepted, reason, rebroadcast = receive_transaction(node, tx_dict, "http://peer")
    assert not accepted
    assert not rebroadcast
    assert not blockchain.mempool.contains(tx_dict["tx_hash"])


def test_receive_malformed_transaction_payload_rejected(blockchain: Blockchain):
    node = NetworkNode(blockchain)
    accepted, reason, rebroadcast = receive_transaction(node, {"not": "a tx"}, "http://peer")
    assert not accepted
    assert "malformed" in reason.lower()
