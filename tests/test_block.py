"""Tests for blockchain/block.py."""

from __future__ import annotations

from blockchain.block import Block
from blockchain.transaction import Transaction
from blockchain.wallet import Wallet


def test_block_hash_is_deterministic_for_same_content():
    block1 = Block(index=1, previous_hash="0" * 64, timestamp=1000.0, nonce=0)
    block2 = Block(index=1, previous_hash="0" * 64, timestamp=1000.0, nonce=0)
    assert block1.hash == block2.hash


def test_block_hash_changes_with_nonce():
    block = Block(index=1, previous_hash="0" * 64, timestamp=1000.0, nonce=0)
    original_hash = block.hash
    block.nonce = 1
    block.recompute()
    assert block.hash != original_hash


def test_merkle_root_changes_when_transactions_change():
    sender = Wallet.create()
    receiver = Wallet.create()
    tx = Transaction(sender=sender.address, receiver=receiver.address, amount=1.0)
    tx.sign(sender)

    empty_block = Block(index=1, previous_hash="0" * 64)
    filled_block = Block(index=1, previous_hash="0" * 64, transactions=[tx])
    assert empty_block.merkle_root != filled_block.merkle_root


def test_meets_difficulty():
    block = Block(index=1, previous_hash="0" * 64)
    block.hash = "0000abcdef"
    assert block.meets_difficulty(4)
    assert not block.meets_difficulty(5)


def test_block_roundtrip_dict():
    sender = Wallet.create()
    receiver = Wallet.create()
    tx = Transaction(sender=sender.address, receiver=receiver.address, amount=1.0)
    tx.sign(sender)
    block = Block(index=2, previous_hash="a" * 64, transactions=[tx])

    restored = Block.from_dict(block.to_dict())
    assert restored.hash == block.hash
    assert restored.merkle_root == block.merkle_root
    assert len(restored.transactions) == 1


def test_genesis_block_is_deterministic():
    g1 = Block.genesis("0" * 64, 1735689600.0, 0, "GENESIS")
    g2 = Block.genesis("0" * 64, 1735689600.0, 0, "GENESIS")
    assert g1.hash == g2.hash
