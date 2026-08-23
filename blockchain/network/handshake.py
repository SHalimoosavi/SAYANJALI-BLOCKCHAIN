"""
Handshake and per-request authentication for SAYANJALI BLOCKCHAIN's P2P
layer (Phase 6.5).

This module is the single place handshake/authentication logic lives.
Routes and other network modules call into `build_auth_envelope()` and
`verify_auth_envelope()` rather than re-implementing any part of this
themselves -- per the explicit instruction not to scatter handshake logic
through routes.

--------------------------------------------------------------------
What gets signed, and what gets verified
--------------------------------------------------------------------

Every authenticated request carries an "auth envelope" alongside its
normal payload:

    {
      "protocol_name": "sayanjali-p2p",
      "protocol_version": "1.0",
      "node_id": "<claimed sender node_id>",
      "public_key": "<claimed sender P2P public key, hex>",
      "network_name": "<sender's network name>",
      "genesis_hash": "<sender's genesis block hash>",
      "advertised_address": "<sender's own advertised address>",
      "capabilities": ["blocks", "transactions", "sync"],
      "timestamp": 1730000000.123,
      "nonce": "<random hex string, fresh per request>",
      "signature": "<hex signature>"
    }

The signature covers every field above *except itself*, plus a SHA-256
hash of the operational payload the envelope accompanies (the block
dict, transaction dict, or sync request body) -- so an attacker cannot
detach a validly-signed envelope from one request and reattach it to a
different payload ("envelope replay with substituted content").

This is a fully stateless, per-request design deliberately: rather than
establishing a session token after an initial handshake, every
authenticated request is independently, cryptographically self-proving.
That avoids needing session storage/expiry logic and means replay
protection (nonce + timestamp freshness) applies uniformly to every
authenticated call, not just an initial login.

An address becomes a *trusted* peer only after a request bearing a valid
envelope for that address has been verified by `POST
/network/peers/authenticate` at least once (see api/network_routes.py).
Discovery registration (`/network/peers/register`) never grants trust by
itself.
"""

from __future__ import annotations

import os
import time
from dataclasses import dataclass, field
from typing import TYPE_CHECKING, Any, Optional

from blockchain.network.identity import (
    P2PIdentity,
    is_valid_public_key,
    verify_identity_signature,
)
from blockchain.utils import BoundedSet, deterministic_json, get_logger, sha256

if TYPE_CHECKING:
    from blockchain.network.node import NetworkNode

logger = get_logger("blockchain.network.handshake")

PROTOCOL_NAME = "sayanjali-p2p"

# Supported protocol versions this node can speak. A single-entry set for
# the MVP; future versions get added here rather than requiring an exact
# string match, keeping negotiation extensible without touching callers.
SUPPORTED_PROTOCOL_VERSIONS = {"1.0"}

REQUIRED_ENVELOPE_FIELDS = (
    "protocol_name",
    "protocol_version",
    "node_id",
    "public_key",
    "network_name",
    "genesis_hash",
    "advertised_address",
    "capabilities",
    "timestamp",
    "nonce",
    "signature",
)


@dataclass
class AuthContext:
    """
    What this node needs on hand to build or verify an auth envelope:
    its own P2P identity, and the network facts a peer's envelope must
    match to be accepted.
    """

    identity: P2PIdentity
    network_name: str
    genesis_hash: str
    advertised_address: str
    capabilities: tuple[str, ...] = ("blocks", "transactions", "sync")


def _signing_fields(envelope: dict[str, Any], payload_hash: str) -> dict[str, Any]:
    """Return the exact field set covered by the envelope's signature."""
    return {
        "protocol_name": envelope["protocol_name"],
        "protocol_version": envelope["protocol_version"],
        "node_id": envelope["node_id"],
        "public_key": envelope["public_key"],
        "network_name": envelope["network_name"],
        "genesis_hash": envelope["genesis_hash"],
        "advertised_address": envelope["advertised_address"],
        "capabilities": envelope["capabilities"],
        "timestamp": envelope["timestamp"],
        "nonce": envelope["nonce"],
        "payload_hash": payload_hash,
    }


def build_auth_envelope(
    context: AuthContext, operational_payload: Optional[dict[str, Any]] = None
) -> dict[str, Any]:
    """
    Build and sign a fresh auth envelope for an outbound authenticated
    request, binding it to `operational_payload` (the block/transaction/
    sync body it accompanies, or None for payload-less requests like the
    handshake itself).
    """
    payload_hash = sha256(deterministic_json(operational_payload or {}))
    envelope: dict[str, Any] = {
        "protocol_name": PROTOCOL_NAME,
        "protocol_version": "1.0",
        "node_id": context.identity.node_id,
        "public_key": context.identity.public_key_hex,
        "network_name": context.network_name,
        "genesis_hash": context.genesis_hash,
        "advertised_address": context.advertised_address,
        "capabilities": list(context.capabilities),
        "timestamp": time.time(),
        "nonce": os.urandom(16).hex(),
    }
    signing_message = deterministic_json(_signing_fields(envelope, payload_hash))
    envelope["signature"] = context.identity.sign(signing_message)
    return envelope


def build_handshake_envelope(
    context: AuthContext, challenge: str
) -> dict[str, Any]:
    """
    Build and sign a handshake envelope proving possession of this node's
    P2P private key over a *specific, server-issued* `challenge` -- the
    literal challenge-response step that establishes trust.

    Structurally identical to `build_auth_envelope`'s output plus one
    additional signed field, `challenge`, which is what makes this
    specifically a response to a particular verifier-issued challenge
    rather than a self-generated proof of freshness.
    """
    envelope = build_auth_envelope(context, operational_payload={"challenge": challenge})
    # Re-sign including the challenge as an explicit top-level field (not
    # just folded into the payload hash) so verification can check it
    # directly without needing to know the operational-payload convention.
    envelope["challenge"] = challenge
    signing_fields = _signing_fields(
        envelope, sha256(deterministic_json({"challenge": challenge}))
    )
    signing_fields["challenge"] = challenge
    signing_message = deterministic_json(signing_fields)
    envelope["signature"] = context.identity.sign(signing_message)
    return envelope


def verify_handshake_envelope(
    envelope: dict[str, Any],
    context: AuthContext,
    challenge_store: "ChallengeStore",
) -> tuple[bool, str]:
    """
    Verify a handshake envelope against a challenge this node itself
    issued, consuming the challenge on success (single-use).

    Unlike `verify_auth_envelope`, this does not consult the ongoing
    replay cache -- the challenge store's single-use consumption already
    provides strictly stronger replay protection for this specific
    exchange (a captured handshake response cannot be replayed even
    within the same freshness window, since the challenge itself is
    deleted the moment it's successfully used).
    """
    if not isinstance(envelope, dict):
        return False, "Malformed authentication: envelope is not an object."

    required = REQUIRED_ENVELOPE_FIELDS + ("challenge",)
    missing = [f for f in required if f not in envelope]
    if missing:
        return False, f"Malformed authentication: missing fields {missing}."

    if envelope["protocol_name"] != PROTOCOL_NAME:
        return False, "Malformed authentication: unknown protocol name."

    if envelope["protocol_version"] not in SUPPORTED_PROTOCOL_VERSIONS:
        return False, (
            f"Unsupported protocol version {envelope['protocol_version']!r}; "
            f"this node supports {sorted(SUPPORTED_PROTOCOL_VERSIONS)}."
        )

    if envelope["network_name"] != context.network_name:
        return False, "Peer belongs to a different network."

    if envelope["genesis_hash"] != context.genesis_hash:
        return False, "Peer has a different genesis identity."

    if not is_valid_public_key(envelope["public_key"]):
        return False, "Malformed authentication: invalid public key."

    if not isinstance(envelope["node_id"], str) or not envelope["node_id"]:
        return False, "Malformed authentication: invalid node_id."

    challenge_ok, challenge_reason = challenge_store.verify_and_consume(
        envelope["node_id"], envelope["challenge"]
    )
    if not challenge_ok:
        return False, challenge_reason

    payload_hash = sha256(deterministic_json({"challenge": envelope["challenge"]}))
    signing_fields = _signing_fields(envelope, payload_hash)
    signing_fields["challenge"] = envelope["challenge"]
    signing_message = deterministic_json(signing_fields)
    if not verify_identity_signature(
        envelope["public_key"], signing_message, envelope["signature"]
    ):
        return False, "Invalid signature."

    return True, ""


class ReplayCache:
    """
    Bounded cache of recently seen (node_id, nonce) pairs, used to reject
    replayed authenticated requests.

    Deliberately in-memory only and deliberately bounded (via
    `BoundedSet`'s FIFO eviction) -- replay protection only needs to hold
    for the freshness window (a few minutes by default), so nothing here
    needs to survive a restart, and nothing here can grow without limit
    regardless of how many distinct (real or fabricated) nonces an
    attacker presents.
    """

    def __init__(self, max_size: int = 10000) -> None:
        self._seen = BoundedSet(max_size)

    def check_and_record(self, node_id: str, nonce: str) -> bool:
        """
        Return True and record the pair if (node_id, nonce) has not been
        seen before; return False without recording if it has (a replay).
        """
        key = f"{node_id}:{nonce}"
        if key in self._seen:
            return False
        self._seen.add(key)
        return True


class ChallengeStore:
    """
    Server-issued challenge-response state for the trust-establishing
    handshake (`POST /network/peers/authenticate`).

    This is intentionally a *separate* mechanism from the self-signed
    envelope (`build_auth_envelope`/`verify_auth_envelope`) used for
    ongoing authenticated traffic once a peer is already trusted:
    establishing trust in the first place uses a true challenge-response
    exchange (this node issues a random challenge; the peer must sign
    *that specific value* to prove key possession), which is the
    strongest, most literal interpretation of "prove possession of a
    private key" and is worth the extra round trip for a one-time trust
    decision. Requiring a fresh server-issued challenge for *every*
    subsequent block/transaction propagation would double the network
    traffic for routine operation without a meaningful security gain
    over the self-signed-envelope approach, which already provides
    freshness (timestamp) and replay protection (nonce + bounded cache)
    for that high-frequency traffic.

    Bounded and self-expiring: a challenge not consumed within
    `ttl_seconds` is simply never matched again (still occupies a slot
    until evicted by the bounded cache's FIFO policy, but is rejected on
    the freshness check regardless of eviction timing) -- no unbounded
    growth is possible since the underlying store has a fixed capacity.
    """

    def __init__(self, max_size: int = 2000, ttl_seconds: float = 60.0) -> None:
        self._challenges: dict[str, tuple[str, float]] = {}
        self._order: list[str] = []
        self._max_size = max_size
        self._ttl_seconds = ttl_seconds

    def issue(self, node_id: str) -> str:
        """Issue and remember a fresh challenge for `node_id`, returning it."""
        challenge = os.urandom(24).hex()
        key = node_id
        if key in self._challenges:
            self._order.remove(key)
        self._challenges[key] = (challenge, time.time())
        self._order.append(key)
        if len(self._order) > self._max_size:
            oldest_key = self._order.pop(0)
            self._challenges.pop(oldest_key, None)
        return challenge

    def verify_and_consume(self, node_id: str, presented_challenge: str) -> tuple[bool, str]:
        """
        Verify `presented_challenge` matches the one most recently issued
        for `node_id` and has not expired, consuming it on success (a
        challenge can only be used once, closing off replaying a
        previously valid handshake response).
        """
        entry = self._challenges.get(node_id)
        if entry is None:
            return False, "No challenge was issued for this node_id."

        issued_challenge, issued_at = entry
        if time.time() - issued_at > self._ttl_seconds:
            self._challenges.pop(node_id, None)
            return False, "Challenge expired."

        if presented_challenge != issued_challenge:
            return False, "Challenge does not match the one issued."

        self._challenges.pop(node_id, None)  # single-use
        if node_id in self._order:
            self._order.remove(node_id)
        return True, ""


def verify_auth_envelope(
    envelope: dict[str, Any],
    operational_payload: Optional[dict[str, Any]],
    context: AuthContext,
    replay_cache: ReplayCache,
    freshness_window_seconds: float,
    known_credential: Optional[dict[str, Any]],
) -> tuple[bool, str]:
    """
    Verify an inbound auth envelope against this node's own network
    identity and replay-protection state.

    Args:
        envelope: The claimed auth envelope from the request body.
        operational_payload: The block/transaction/sync body the
            envelope accompanies (must match what the sender signed).
        context: This node's own identity/network facts to check against.
        replay_cache: This node's bounded replay-detection cache.
        freshness_window_seconds: Maximum allowed clock skew/staleness.
        known_credential: The previously stored credential for this
            sender's claimed address, if any (from
            `Storage.get_peer_credential`) -- used for identity-change
            detection. None if this sender has never been seen before.

    Returns:
        (is_valid, reason) -- reason is empty when valid. Reasons are
        deliberately generic/consistent across distinct failure causes
        where the difference would otherwise leak information useful to
        an attacker probing for valid identities (see the "malformed
        authentication" vs "invalid signature" cases below, which are
        kept distinguishable for legitimate debugging but never reveal
        *why* a given node_id/public_key pairing specifically failed).
    """
    if not isinstance(envelope, dict):
        return False, "Malformed authentication: envelope is not an object."

    missing = [f for f in REQUIRED_ENVELOPE_FIELDS if f not in envelope]
    if missing:
        return False, f"Malformed authentication: missing fields {missing}."

    if envelope["protocol_name"] != PROTOCOL_NAME:
        return False, "Malformed authentication: unknown protocol name."

    if envelope["protocol_version"] not in SUPPORTED_PROTOCOL_VERSIONS:
        return False, (
            f"Unsupported protocol version {envelope['protocol_version']!r}; "
            f"this node supports {sorted(SUPPORTED_PROTOCOL_VERSIONS)}."
        )

    if envelope["network_name"] != context.network_name:
        return False, "Peer belongs to a different network."

    if envelope["genesis_hash"] != context.genesis_hash:
        return False, "Peer has a different genesis identity."

    if not is_valid_public_key(envelope["public_key"]):
        return False, "Malformed authentication: invalid public key."

    if not isinstance(envelope["node_id"], str) or not envelope["node_id"]:
        return False, "Malformed authentication: invalid node_id."

    now = time.time()
    try:
        timestamp = float(envelope["timestamp"])
    except (TypeError, ValueError):
        return False, "Malformed authentication: invalid timestamp."
    if abs(now - timestamp) > freshness_window_seconds:
        return False, "Authentication expired: timestamp outside the freshness window."

    nonce = envelope["nonce"]
    if not isinstance(nonce, str) or not nonce:
        return False, "Malformed authentication: invalid nonce."
    if not replay_cache.check_and_record(envelope["node_id"], nonce):
        return False, "Authentication rejected: replayed nonce."

    # Identity-change detection: if we've previously authenticated this
    # address under a different node_id or public_key, do not silently
    # accept the new claim -- require it to independently pass signature
    # verification (which it will, below, if this is a legitimate
    # rotation) but flag it loudly, since a mismatch here is exactly the
    # signature of a spoofing attempt or an unannounced identity change.
    if known_credential is not None:
        if (
            known_credential["node_id"] != envelope["node_id"]
            or known_credential["public_key_hex"] != envelope["public_key"]
        ):
            logger.warning(
                "Peer identity change detected for a previously trusted "
                "address: stored node_id=%s, claimed node_id=%s. "
                "Re-authentication required before this claim is trusted.",
                known_credential["node_id"],
                envelope["node_id"],
            )
            # Fall through to signature verification rather than reject
            # outright here -- a legitimate identity rotation must still
            # prove possession of the new private key. The caller (the
            # route handler) is responsible for treating this as a fresh
            # authentication rather than silently extending prior trust.

    payload_hash = sha256(deterministic_json(operational_payload or {}))
    signing_message = deterministic_json(_signing_fields(envelope, payload_hash))
    if not verify_identity_signature(
        envelope["public_key"], signing_message, envelope["signature"]
    ):
        return False, "Invalid signature."

    return True, ""


def perform_handshake(node: "NetworkNode", peer_address: str) -> tuple[bool, str]:
    """
    Initiate the client side of the challenge-response handshake against
    `peer_address`: request a challenge, sign it, and submit the result.

    On success, `peer_address` now trusts *this* node (the credential is
    stored on the peer's side, not this node's). Establishing mutual
    trust requires the peer to independently perform the same handshake
    against this node -- see `cli/main.py`'s `add-peer` command, which
    calls this in one direction and documents that the operator on the
    other side needs to run the equivalent command to complete the
    other direction.
    """
    challenge_response = node.client.request_challenge(peer_address, node.node_id)
    if challenge_response is None:
        return False, "Could not obtain a challenge from the peer (unreachable or rejected)."

    challenge = challenge_response.get("challenge")
    if not challenge:
        return False, "Peer did not return a valid challenge."

    envelope = build_handshake_envelope(node.auth_context, challenge)
    auth_response = node.client.authenticate_with(peer_address, envelope)
    if auth_response is None:
        return False, "Peer was unreachable while submitting the handshake response."

    if not auth_response.get("authenticated"):
        return False, auth_response.get("reason") or "Peer rejected the handshake."

    return True, ""
