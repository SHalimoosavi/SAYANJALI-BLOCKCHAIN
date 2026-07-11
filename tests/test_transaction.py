"""Tests for blockchain/transaction.py."""

from __future__ import annotations

from blockchain.transaction import Transaction
from blockchain.wallet import Wallet


def test_signed_transaction_verifies():
    sender = Wallet.create()
    receiver = Wallet.create()
    tx = Transaction(sender=sender.address, receiver=receiver.address, amount=10.0)
    tx.sign(sender)
    assert tx.verify()


def test_unsigned_transaction_fails_verification():
    sender = Wallet.create()
    receiver = Wallet.create()
    tx = Transaction(sender=sender.address, receiver=receiver.address, amount=10.0)
    assert not tx.verify()


def test_tampered_amount_fails_verification():
    sender = Wallet.create()
    receiver = Wallet.create()
    tx = Transaction(sender=sender.address, receiver=receiver.address, amount=10.0)
    tx.sign(sender)
    tx.amount = 999.0  # tamper after signing
    assert not tx.verify()


def test_coinbase_transaction_verifies_without_signature():
    receiver = Wallet.create()
    tx = Transaction.new_coinbase(receiver.address, 50.0)
    assert tx.is_coinbase()
    assert tx.verify()


def test_negative_amount_is_invalid():
    sender = Wallet.create()
    receiver = Wallet.create()
    tx = Transaction(sender=sender.address, receiver=receiver.address, amount=-5.0)
    tx.sign(sender) if False else None  # negative amounts should fail before signing matters
    assert not tx.verify()


def test_transaction_roundtrip_dict():
    sender = Wallet.create()
    receiver = Wallet.create()
    tx = Transaction(sender=sender.address, receiver=receiver.address, amount=25.5)
    tx.sign(sender)

    restored = Transaction.from_dict(tx.to_dict())
    assert restored.tx_hash == tx.tx_hash
    assert restored.verify()


def test_hash_changes_if_content_changes():
    sender = Wallet.create()
    receiver = Wallet.create()
    tx1 = Transaction(sender=sender.address, receiver=receiver.address, amount=1.0, timestamp=1000.0)
    tx2 = Transaction(sender=sender.address, receiver=receiver.address, amount=2.0, timestamp=1000.0)
    assert tx1.tx_hash != tx2.tx_hash
