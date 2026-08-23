"""Tests for blockchain/network/identity.py."""

from __future__ import annotations

from blockchain.network.identity import (
    P2PIdentity,
    is_valid_public_key,
    verify_identity_signature,
)


def test_generate_produces_distinct_keypairs():
    id1 = P2PIdentity.generate("node-a")
    id2 = P2PIdentity.generate("node-b")
    assert id1.private_key_hex != id2.private_key_hex
    assert id1.public_key_hex != id2.public_key_hex


def test_sign_and_verify_roundtrip():
    identity = P2PIdentity.generate("node-a")
    message = "hello peer"
    signature = identity.sign(message)
    assert verify_identity_signature(identity.public_key_hex, message, signature)


def test_verify_fails_on_tampered_message():
    identity = P2PIdentity.generate("node-a")
    signature = identity.sign("original")
    assert not verify_identity_signature(identity.public_key_hex, "tampered", signature)


def test_verify_fails_with_wrong_public_key():
    identity_a = P2PIdentity.generate("node-a")
    identity_b = P2PIdentity.generate("node-b")
    signature = identity_a.sign("message")
    assert not verify_identity_signature(identity_b.public_key_hex, "message", signature)


def test_is_valid_public_key_accepts_real_key():
    identity = P2PIdentity.generate("node-a")
    assert is_valid_public_key(identity.public_key_hex)


def test_is_valid_public_key_rejects_garbage():
    assert not is_valid_public_key("not-a-key")
    assert not is_valid_public_key("")
    assert not is_valid_public_key("ff" * 10)  # wrong length


def test_identity_never_exposes_private_key_via_verification():
    """
    A meta-test confirming the verification path only ever needs the
    public key -- structurally, nothing in the verification API accepts
    or requires private key material.
    """
    import inspect

    from blockchain.network import identity as identity_module

    sig = inspect.signature(identity_module.verify_identity_signature)
    assert "private" not in " ".join(sig.parameters.keys()).lower()
