"""Tests for blockchain/mining.py, blockchain/consensus.py, and end-to-end mining."""

from __future__ import annotations

from blockchain.blockchain import Blockchain
from blockchain.transaction import Transaction
from blockchain.wallet import Wallet


def test_mine_empty_block_succeeds(blockchain: Blockchain):
    miner = Wallet.create()
    initial_length = blockchain.length
    block = blockchain.mine_pending_transactions(miner.address)
    assert blockchain.length == initial_length + 1
    assert block.meets_difficulty(blockchain.settings.consensus.difficulty)


def test_coinbase_reward_credited_to_miner(blockchain: Blockchain):
    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)
    balance = blockchain.get_balance(miner.address)
    assert balance == blockchain.settings.mining.block_reward


def test_mining_includes_pending_transactions(blockchain: Blockchain):
    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)  # fund the miner first

    receiver = Wallet.create()
    tx = Transaction(sender=miner.address, receiver=receiver.address, amount=5.0)
    tx.sign(miner)
    accepted, reason = blockchain.submit_transaction(tx)
    assert accepted, reason

    block = blockchain.mine_pending_transactions(miner.address)
    tx_hashes = [t.tx_hash for t in block.transactions]
    assert tx.tx_hash in tx_hashes
    assert blockchain.mempool.size() == 0


def test_mined_block_passes_consensus_validation(blockchain: Blockchain):
    miner = Wallet.create()
    block = blockchain.mine_pending_transactions(miner.address)
    difficulty = blockchain.settings.consensus.difficulty
    assert blockchain.consensus.validate(block, difficulty)
