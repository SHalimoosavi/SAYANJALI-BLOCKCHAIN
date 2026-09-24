# ADR-013 — Canonical Network Identifier

**Status:** Accepted for Phase 9.2 contract freeze candidate
**Date:** 2026-09-24
**Decision:** `network_id` is the canonical SYJ protocol identifier.

## 1. Context

The repository contains multiple historical identity concepts. The audited V2 Go foundation already uses a 64-character lowercase hexadecimal `network_id` and derives an EffectiveNetworkID from protocol version, genesis-state commitment, historical genesis hash, and network name. Older V1 code and documentation also contain a numeric `chain_id` configuration. The proposed production runtime is Cosmos SDK + CometBFT, where an external `chain-id` string is required by the runtime.

Without an explicit decision, these names could be treated as separate identities, creating cross-network replay, migration, genesis, or consensus-message ambiguity.

## 2. Decision

`network_id` is the **single canonical SYJ protocol network identity**.

The protocol MUST use `network_id` for:

- transaction binding;
- block binding;
- genesis binding;
- P2P identity context;
- consensus-message authentication/replay protection;
- network-specific test vectors;
- migration and state-commitment identity.

The Cosmos SDK/CometBFT `chain-id` is a **compatibility representation only**. It MUST be deterministically derived from `network_id` and MUST NOT define a second independent identity.

## 3. Rationale

The decision preserves the already audited V2 identity model and avoids silently changing existing V2 vectors. It also gives the production runtime a deterministic way to satisfy an external `chain-id` field without introducing a second source of truth.

## 4. Canonical `network_id` format

`network_id` MUST:

- be exactly 64 characters;
- contain only lowercase hexadecimal characters `0-9a-f`;
- represent exactly 32 bytes;
- be non-zero for an approved production genesis;
- match the deterministic EffectiveNetworkID derivation.

The audited V2 derivation is retained:

```text
canonical_payload = canonical_json({
  "consensus_protocol_version": 2,
  "genesis_state_commitment": <64-hex>,
  "historical_genesis_hash": <64-hex>,
  "network_name": <canonical name>
})

network_id = SHA256(
  ASCII("SYJ-EFFECTIVE-NETWORK-ID-V1\0") ||
  canonical_payload
)
```

For a versioned future protocol, the derivation input MUST be versioned explicitly; Phase 9.2 does not authorize a silent derivation change.

## 5. Deterministic Cosmos/CometBFT mapping

The external runtime `chain-id` MUST be derived losslessly from `network_id` as follows:

1. Decode the 64 lowercase hexadecimal characters into 32 raw bytes.
2. Encode those bytes using unpadded Base64URL (`A-Z`, `a-z`, `0-9`, `-`, `_`).
3. Prefix the result with the ASCII string `syj-`.
4. The resulting value is the external `chain-id`.

The mapping is therefore:

```text
chain-id = "syj-" + base64url_no_padding(hex_decode(network_id))
```

This produces a deterministic 47-character ASCII value for every valid 32-byte `network_id`. The mapping is injective: the original `network_id` can be recovered exactly from the mapped `chain-id`.

The runtime adapter MUST verify both directions. A supplied external `chain-id` that does not decode to the active `network_id` MUST be rejected. No operator-supplied independent `chain-id` is authoritative.

## 6. Binding rules

### Transactions

The transaction `network_id` MUST equal the active network identity before signature acceptance and state execution. It is part of the V2 signing payload and therefore replay protection.

### Blocks

A production block MUST carry `network_id` in its consensus-critical contract. A block for a different `network_id` MUST be rejected before execution.

### Genesis

`genesis-v2.json` MUST contain `network_id`, and the value MUST equal the deterministic EffectiveNetworkID derived from the approved genesis inputs. A mismatched declaration MUST fail closed.

### P2P handshakes

P2P sessions MUST be associated with the active `network_id` and genesis identity. A legacy V1 numeric `chain_id` MUST NOT be treated as a substitute for Phase 9 identity.

### Consensus messages

Consensus-critical messages MUST be authenticated, replay-protected, and bound to `network_id`, height, round, step, and the active protocol version. The external CometBFT `chain-id` representation MUST be derived from the same `network_id` and MUST NOT be independently selected.

## 7. Alternatives considered

### A. Make `chain_id` canonical

Rejected for Phase 9.2 because it would make the production contract depend on a runtime-specific compatibility field and would conflict with the already audited V2 `network_id` identity.

### B. Maintain independent `network_id` and `chain-id`

Rejected because two independent identities create ambiguity for replay protection, genesis binding, migration, and consensus evidence.

### C. Replace the V2 EffectiveNetworkID derivation

Rejected for Phase 9.2 because it would silently invalidate existing V2 vectors and would expand the remediation scope beyond contract clarification.

## 8. Consequences

Positive consequences:
- one canonical protocol identity;
- deterministic cross-runtime compatibility;
- preserved V2 network-ID vectors;
- explicit replay isolation;
- unambiguous genesis and migration rules.

Negative/operational consequences:
- Phase 10 must implement a strict runtime adapter for `chain-id` mapping;
- legacy V1 `chain_id` configuration must be retired or explicitly adapted during migration;
- external tooling may display a `chain-id` that differs textually from `network_id`, even though it represents the same identity.

## 9. Migration impact on V2 vectors

Existing V2 vectors that contain the audited `network_id` remain valid compatibility evidence. Phase 9.2 MUST NOT rewrite the value `237a1934769295c63fe47771a4996b17c3899e52f6bacf79ed7edefec90eaaf3` or alter the canonical signing payload solely to satisfy Cosmos naming conventions.

The Cosmos/CometBFT `chain-id` mapping is an adapter-layer addition; it does not change V2 transaction bytes.

## 10. Test requirements

At minimum, Phase 10 MUST test:

- valid 64-hex `network_id` acceptance;
- uppercase/wrong-length/non-hex rejection;
- all-zero production identity rejection;
- EffectiveNetworkID derivation against existing V2 vectors;
- transaction wrong-network rejection;
- block wrong-network rejection;
- genesis declaration/derivation mismatch rejection;
- P2P wrong-network handshake rejection;
- consensus-message replay across network IDs;
- exact forward mapping `network_id -> chain-id`;
- exact reverse mapping `chain-id -> network_id`;
- malformed/foreign `chain-id` rejection;
- no independent operator override of `chain-id`.

## 11. Open questions

- Exact Phase 10 runtime configuration API for injecting the derived `chain-id` is TBD — requires runtime integration review.
- Any future protocol version that changes the EffectiveNetworkID derivation requires a new versioned ADR and migration contract.
- Legacy V1 `chain_id` retirement timing is TBD — requires migration/operational review.

## 12. Review checklist

- [ ] `network_id` is the only canonical SYJ identity.
- [ ] Cosmos/CometBFT `chain-id` is compatibility-only.
- [ ] The mapping is deterministic and reversible.
- [ ] Existing V2 vectors remain unchanged.
- [ ] Genesis, blocks, transactions, P2P, and consensus all bind to the same identity.
- [ ] No runtime implementation is introduced by this ADR.
