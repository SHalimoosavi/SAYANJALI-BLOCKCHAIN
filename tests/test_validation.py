"""Tests for blockchain/validators.py."""

from __future__ import annotations

from blockchain.blockchain import Blockchain
from blockchain.wallet import Wallet


def test_fresh_chain_is_valid(blockchain: Blockchain):
    is_valid, reason = blockchain.is_chain_valid()
    assert is_valid, reason


def test_chain_remains_valid_after_mining(blockchain: Blockchain):
    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)
    blockchain.mine_pending_transactions(miner.address)
    is_valid, reason = blockchain.is_chain_valid()
    assert is_valid, reason


def test_tampering_with_block_hash_invalidates_chain(blockchain: Blockchain):
    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)
    blockchain.chain[-1].hash = "tampered" * 8  # corrupt the tip
    is_valid, reason = blockchain.is_chain_valid()
    assert not is_valid


def test_tampering_with_transaction_amount_invalidates_chain(blockchain: Blockchain):
    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)
    # Tamper with the coinbase amount without recomputing the merkle root.
    blockchain.chain[-1].transactions[0].amount = 999999.0
    is_valid, reason = blockchain.is_chain_valid()
    assert not is_valid
