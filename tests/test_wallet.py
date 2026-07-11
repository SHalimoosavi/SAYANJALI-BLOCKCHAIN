"""Tests for blockchain/wallet.py."""

from __future__ import annotations

import pytest

from blockchain.utils import WalletError
from blockchain.wallet import Wallet, derive_address, is_valid_address, verify_signature


def test_create_wallet_has_valid_address():
    wallet = Wallet.create()
    assert wallet.address.startswith("SYJ")
    assert is_valid_address(wallet.address)


def test_two_wallets_are_different():
    w1 = Wallet.create()
    w2 = Wallet.create()
    assert w1.address != w2.address
    assert w1.private_key_hex != w2.private_key_hex


def test_address_derivation_is_deterministic():
    wallet = Wallet.create()
    assert derive_address(wallet.public_key_hex) == wallet.address


def test_from_private_key_reconstructs_same_wallet():
    original = Wallet.create()
    restored = Wallet.from_private_key(original.private_key_hex)
    assert restored.address == original.address
    assert restored.public_key_hex == original.public_key_hex


def test_from_private_key_rejects_garbage():
    with pytest.raises(WalletError):
        Wallet.from_private_key("not-a-valid-hex-key")


def test_sign_and_verify_roundtrip():
    wallet = Wallet.create()
    message = "hello sayanjali"
    signature = wallet.sign(message)
    assert verify_signature(wallet.public_key_hex, message, signature)


def test_verify_fails_on_tampered_message():
    wallet = Wallet.create()
    signature = wallet.sign("original message")
    assert not verify_signature(wallet.public_key_hex, "tampered message", signature)


def test_is_valid_address_rejects_bad_formats():
    assert not is_valid_address("not-an-address")
    assert not is_valid_address("SYJ123")  # too short
    assert not is_valid_address(12345)  # type: ignore[arg-type]
