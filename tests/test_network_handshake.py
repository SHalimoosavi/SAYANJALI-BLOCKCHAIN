"""
Tests for blockchain/network/handshake.py: the challenge-response
handshake, protocol negotiation, and ongoing per-request authentication.
"""

from __future__ import annotations

import time

from blockchain.network.handshake import (
    AuthContext,
    ChallengeStore,
    ReplayCache,
    build_auth_envelope,
    build_handshake_envelope,
    verify_auth_envelope,
    verify_handshake_envelope,
)
from blockchain.network.identity import P2PIdentity

GENESIS = "g" * 64


def _ctx(identity: P2PIdentity, network_name: str = "test-net", genesis: str = GENESIS) -> AuthContext:
    return AuthContext(
        identity=identity, network_name=network_name, genesis_hash=genesis,
        advertised_address=f"http://{identity.node_id}",
    )


class TestChallengeResponseHandshake:
    def test_valid_handshake_accepted(self):
        identity = P2PIdentity.generate("node-a")
        ctx = _ctx(identity)
        store = ChallengeStore()
        challenge = store.issue(identity.node_id)
        envelope = build_handshake_envelope(ctx, challenge)

        verifier_ctx = _ctx(P2PIdentity.generate("verifier"))
        ok, reason = verify_handshake_envelope(envelope, verifier_ctx, store)
        assert ok, reason

    def test_incompatible_protocol_version_rejected(self):
        identity = P2PIdentity.generate("node-a")
        ctx = _ctx(identity)
        store = ChallengeStore()
        challenge = store.issue(identity.node_id)
        envelope = build_handshake_envelope(ctx, challenge)
        envelope["protocol_version"] = "99.0"

        verifier_ctx = _ctx(P2PIdentity.generate("verifier"))
        ok, reason = verify_handshake_envelope(envelope, verifier_ctx, store)
        assert not ok
        assert "protocol version" in reason.lower()

    def test_wrong_network_rejected(self):
        identity = P2PIdentity.generate("node-a")
        ctx = _ctx(identity, network_name="network-alpha")
        store = ChallengeStore()
        challenge = store.issue(identity.node_id)
        envelope = build_handshake_envelope(ctx, challenge)

        verifier_ctx = _ctx(P2PIdentity.generate("verifier"), network_name="network-beta")
        ok, reason = verify_handshake_envelope(envelope, verifier_ctx, store)
        assert not ok
        assert "network" in reason.lower()

    def test_wrong_genesis_rejected(self):
        identity = P2PIdentity.generate("node-a")
        ctx = _ctx(identity, genesis="a" * 64)
        store = ChallengeStore()
        challenge = store.issue(identity.node_id)
        envelope = build_handshake_envelope(ctx, challenge)

        verifier_ctx = _ctx(P2PIdentity.generate("verifier"), genesis="b" * 64)
        ok, reason = verify_handshake_envelope(envelope, verifier_ctx, store)
        assert not ok
        assert "genesis" in reason.lower()

    def test_unsupported_capability_handled_safely(self):
        """
        An envelope declaring a capability this verifier doesn't
        recognize must not crash verification -- capabilities are
        informational, not a hard compatibility gate, for extensibility.
        """
        identity = P2PIdentity.generate("node-a")
        ctx = _ctx(identity)
        ctx.capabilities = ("blocks", "transactions", "sync", "some-future-capability")
        store = ChallengeStore()
        challenge = store.issue(identity.node_id)
        envelope = build_handshake_envelope(ctx, challenge)

        verifier_ctx = _ctx(P2PIdentity.generate("verifier"))
        ok, reason = verify_handshake_envelope(envelope, verifier_ctx, store)
        assert ok, reason

    def test_expired_challenge_rejected(self):
        identity = P2PIdentity.generate("node-a")
        ctx = _ctx(identity)
        store = ChallengeStore(ttl_seconds=0.05)
        challenge = store.issue(identity.node_id)
        time.sleep(0.1)
        envelope = build_handshake_envelope(ctx, challenge)

        verifier_ctx = _ctx(P2PIdentity.generate("verifier"))
        ok, reason = verify_handshake_envelope(envelope, verifier_ctx, store)
        assert not ok
        assert "expired" in reason.lower()

    def test_replayed_challenge_response_rejected(self):
        identity = P2PIdentity.generate("node-a")
        ctx = _ctx(identity)
        store = ChallengeStore()
        challenge = store.issue(identity.node_id)
        envelope = build_handshake_envelope(ctx, challenge)

        verifier_ctx = _ctx(P2PIdentity.generate("verifier"))
        ok1, _ = verify_handshake_envelope(envelope, verifier_ctx, store)
        assert ok1
        ok2, reason2 = verify_handshake_envelope(envelope, verifier_ctx, store)
        assert not ok2

    def test_no_challenge_issued_rejected(self):
        identity = P2PIdentity.generate("node-a")
        ctx = _ctx(identity)
        store = ChallengeStore()
        # Never call store.issue() -- this node_id has no pending challenge.
        envelope = build_handshake_envelope(ctx, "fabricated-challenge-value")

        verifier_ctx = _ctx(P2PIdentity.generate("verifier"))
        ok, reason = verify_handshake_envelope(envelope, verifier_ctx, store)
        assert not ok
        assert "no challenge" in reason.lower()

    def test_mismatched_challenge_rejected(self):
        identity = P2PIdentity.generate("node-a")
        ctx = _ctx(identity)
        store = ChallengeStore()
        store.issue(identity.node_id)
        # Sign a DIFFERENT value than what was actually issued.
        envelope = build_handshake_envelope(ctx, "wrong-challenge-value")

        verifier_ctx = _ctx(P2PIdentity.generate("verifier"))
        ok, reason = verify_handshake_envelope(envelope, verifier_ctx, store)
        assert not ok

    def test_malformed_envelope_rejected(self):
        store = ChallengeStore()
        verifier_ctx = _ctx(P2PIdentity.generate("verifier"))
        ok, reason = verify_handshake_envelope({"nonsense": True}, verifier_ctx, store)
        assert not ok
        assert "malformed" in reason.lower()

    def test_challenge_store_bounded_memory(self):
        """Direct regression test: issuing many challenges must not grow unbounded."""
        store = ChallengeStore(max_size=50)
        for i in range(5000):
            store.issue(f"node-{i}")
        assert len(store._challenges) <= 50


class TestOngoingAuthEnvelope:
    """The lighter, per-request self-signed envelope used once trust exists."""

    def test_valid_envelope_accepted(self):
        identity = P2PIdentity.generate("node-a")
        ctx = _ctx(identity)
        verifier_ctx = _ctx(P2PIdentity.generate("verifier"))
        cache = ReplayCache()

        envelope = build_auth_envelope(ctx, {"x": 1})
        ok, reason = verify_auth_envelope(envelope, {"x": 1}, verifier_ctx, cache, 300.0, None)
        assert ok, reason

    def test_replay_rejected(self):
        identity = P2PIdentity.generate("node-a")
        ctx = _ctx(identity)
        verifier_ctx = _ctx(P2PIdentity.generate("verifier"))
        cache = ReplayCache()

        envelope = build_auth_envelope(ctx, {"x": 1})
        ok1, _ = verify_auth_envelope(envelope, {"x": 1}, verifier_ctx, cache, 300.0, None)
        assert ok1
        ok2, reason2 = verify_auth_envelope(envelope, {"x": 1}, verifier_ctx, cache, 300.0, None)
        assert not ok2
        assert "replay" in reason2.lower()

    def test_expired_timestamp_rejected(self):
        identity = P2PIdentity.generate("node-a")
        ctx = _ctx(identity)
        verifier_ctx = _ctx(P2PIdentity.generate("verifier"))
        cache = ReplayCache()

        envelope = build_auth_envelope(ctx, {"x": 1})
        envelope["timestamp"] = time.time() - 10000
        envelope["nonce"] = "distinct-nonce-for-this-test"
        ok, reason = verify_auth_envelope(envelope, {"x": 1}, verifier_ctx, cache, 300.0, None)
        assert not ok
        assert "expired" in reason.lower()

    def test_payload_substitution_rejected(self):
        """The envelope must be bound to the exact payload it accompanies."""
        identity = P2PIdentity.generate("node-a")
        ctx = _ctx(identity)
        verifier_ctx = _ctx(P2PIdentity.generate("verifier"))
        cache = ReplayCache()

        envelope = build_auth_envelope(ctx, {"x": 1})
        ok, reason = verify_auth_envelope(envelope, {"x": 999}, verifier_ctx, cache, 300.0, None)
        assert not ok
        assert "signature" in reason.lower()

    def test_replay_cache_bounded_memory(self):
        cache = ReplayCache(max_size=50)
        for i in range(5000):
            cache.check_and_record(f"node-{i}", f"nonce-{i}")
        assert len(cache._seen) <= 50
