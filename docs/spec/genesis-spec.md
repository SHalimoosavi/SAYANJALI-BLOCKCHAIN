# Genesis Specification — V2 Target Contract

**Status:** Phase 9.2 normative specification candidate
**Schema:** `protocol/schemas/genesis-v2.json`
**Implementation status:** NOT IMPLEMENTED
**Mainnet status:** NO MAINNET GENESIS CREATED
**Canonical identifier:** `network_id`

> This document defines the structure and validation contract for a future deterministic genesis artifact. It does not create, approve, or populate a mainnet genesis. Any undecided production parameter is **TBD — requires governance, economic, security, or legal review**.

## 1. Scope

A genesis document MUST deterministically establish the initial protocol version, network identity, historical lineage anchor, initial state commitment, account state, validator state, staking state, governance state, and monetary-policy state.

The authoritative top-level field set is exactly:

`protocol_version`, `network_name`, `network_id`, `historical_genesis_hash`, `genesis_state_commitment`, `accounts`, `validators`, `staking`, `governance`, `monetary_policy`.

No other top-level field is permitted by the Phase 9.2 schema.

## 2. Status and normative terms

**MUST** and **MUST NOT** are mandatory. **SHOULD** and **SHOULD NOT** are strong recommendations. **MAY** is optional. A schema-valid document can still be semantically invalid and MUST undergo all semantic validation rules in this specification.

## 3. Genesis identity and binding

### 3.1 Network identity

`network_id` is the canonical SYJ identity. It MUST be exactly 64 lowercase hexadecimal characters and MUST match the deterministic EffectiveNetworkID derived from:

```text
{
  "consensus_protocol_version": protocol_version,
  "genesis_state_commitment": genesis_state_commitment,
  "historical_genesis_hash": historical_genesis_hash,
  "network_name": network_name
}
```

The derivation uses the existing audited compatibility algorithm and domain `SYJ-EFFECTIVE-NETWORK-ID-V1\0` over the repository's deterministic canonical JSON representation. Phase 9.2 does not silently replace the audited V2 derivation.

The current private-testnet value `237a1934769295c63fe47771a4996b17c3899e52f6bacf79ed7edefec90eaaf3` is historical/test-only and MUST NOT be copied into a production genesis.

### 3.2 Historical lineage

`historical_genesis_hash` is the approved 64-hexadecimal hash identifying the historical genesis from which a migration lineage is derived. For any future production deployment, its exact value is **TBD — requires governance, security, and migration review**. A new independent network MUST define an explicit lineage rule before genesis approval; Phase 9.2 does not choose a production value.

### 3.3 Genesis state commitment

`genesis_state_commitment` is a domain-separated SHA-256 commitment over the canonical genesis-state payload excluding the `genesis_state_commitment` field itself:

```text
SHA256(
  ASCII("SYJ-GENESIS-STATE-COMMIT-V2\0") ||
  canonical_json(genesis_state_payload)
)
```

`genesis_state_payload` consists of all top-level genesis fields except `genesis_state_commitment`, with arrays and nested objects normalized under the canonicalization rules in §4. This bootstrap commitment is distinct from the later authenticated runtime state-root algorithm in ADR-009; the concrete production state-tree implementation remains separately specified and MUST NOT be inferred from this field.

### 3.4 Derived genesis hash

The genesis hash is a derived identity, not a serialized top-level field:

```text
SHA256(
  ASCII("SYJ-GENESIS-V2\0") ||
  canonical_json(full_genesis_document)
)
```

The derived hash MUST be computed after the declared `genesis_state_commitment` and `network_id` have been validated. It MUST NOT be used to alter either field after calculation.

## 4. Canonical genesis serialization

The Phase 9.2 target serialization profile follows the audited repository canonical-JSON behavior:

- UTF-8 output;
- object keys sorted lexicographically;
- no insignificant whitespace;
- deterministic JSON string escaping;
- JSON integers represented as integers;
- arrays preserve their declared order only where the specification defines an ordered sequence;
- account and validator arrays MUST be sorted by their canonical identifier before commitment;
- nested module-state maps MUST use sorted object keys;
- non-finite numeric values MUST be rejected;
- duplicate JSON object keys MUST be rejected before semantic parsing.

The canonicalization profile is a protocol contract; implementations MUST NOT substitute a different serializer without a versioned protocol decision.

## 5. Top-level field contract

| Field | Type | Required | Canonical/format | Core constraints | Initialization semantics |
|---|---|---:|---|---|---|
| `protocol_version` | integer | MUST | integer | MUST be `2` for `genesis-v2.json`; later protocol versions require a new versioned schema/contract | Selects the target Phase 9/V2 state-transition contract. |
| `network_name` | string | MUST | ASCII, 1–64 chars; `[A-Za-z0-9][A-Za-z0-9._-]{0,63}` | MUST be non-empty and deterministic | Human-readable network label; not the canonical identity. |
| `network_id` | string | MUST | 64 lowercase hex | MUST equal the deterministic EffectiveNetworkID | Binds every consensus-relevant object to one network. |
| `historical_genesis_hash` | string | MUST | 64 lowercase hex | MUST identify the approved historical lineage anchor; production value TBD | Establishes migration/history lineage. |
| `genesis_state_commitment` | string | MUST | 64 lowercase hex | MUST equal the §3.3 commitment | Authenticates the canonical initial state material. |
| `accounts` | array | MUST | canonical account records sorted by address | No duplicate addresses; balances/nonce values MUST be non-negative and representable | Initializes deterministic account state. |
| `validators` | array | MUST | canonical validator records sorted by `validator_id` | No duplicate validator IDs or consensus keys; stake values MUST be non-negative | Initializes the candidate/authorized validator state. |
| `staking` | object | MUST | canonical JSON object | MUST be semantically valid; final economic parameters remain TBD | Initializes staking module state and/or approved parameter placeholders. |
| `governance` | object | MUST | canonical JSON object | MUST be semantically valid; final thresholds and emergency authority remain TBD | Initializes governance module state and approved parameter placeholders. |
| `monetary_policy` | object | MUST | canonical JSON object | MUST be semantically valid; final supply/issuance/burn policy remains TBD | Initializes monetary-policy state without selecting final production economics. |

## 6. Field-by-field normative rules

### 6.1 `protocol_version`

- Type: integer.
- Required: MUST.
- Allowed value in this schema: exactly `2`.
- Any other value MUST be rejected as an unsupported genesis schema version.
- A future protocol version MUST use a separately versioned genesis schema and explicit migration/upgrade rules.
- No implementation may treat an unsupported value as a best-effort compatibility mode.

### 6.2 `network_name`

- Type: string.
- Required: MUST.
- Format: ASCII `[A-Za-z0-9][A-Za-z0-9._-]{0,63}`.
- Canonical form: exact byte-preserving string; no case-folding or whitespace trimming is permitted during commitment.
- It MUST NOT be used as a substitute for `network_id`.
- Changing it changes the derived `network_id` and therefore creates a different network identity.

### 6.3 `network_id`

- Type: string.
- Required: MUST.
- Format: 64 lowercase hexadecimal characters.
- Allowed values: any valid non-empty 256-bit value; the all-zero value MUST be rejected for an approved production genesis because it represents a null identity.
- Validation: recompute the EffectiveNetworkID from §3.1 and require exact equality.
- Rejection: malformed hex, uppercase hex, wrong length, all-zero production identity, or derivation mismatch MUST fail closed.
- Migration: changing `network_id` is a network-identity change and MUST be treated as a new genesis/explicit migration boundary.

### 6.4 `historical_genesis_hash`

- Type: string.
- Required: MUST.
- Format: 64 lowercase hexadecimal characters.
- Validation: MUST be cryptographically well-formed and match the approved migration lineage record when one exists.
- It MUST NOT be silently replaced by the new genesis hash.
- Exact production lineage value is **TBD — requires governance and security review**.

### 6.5 `genesis_state_commitment`

- Type: string.
- Required: MUST.
- Format: 64 lowercase hexadecimal characters.
- Validation: recompute the §3.3 domain-separated commitment and require exact equality.
- The field MUST be validated before deriving the final genesis hash and network ID.
- Any mismatch MUST reject genesis loading.

### 6.6 `accounts`

- Type: array of objects.
- Required: MUST, including when empty.
- Each account MUST contain exactly `address`, `balance_base_units`, and `nonce` under the current V2 genesis contract.
- `address` MUST use the audited V2 SYJ address form `SYJ` followed by exactly 40 lowercase hexadecimal characters.
- `balance_base_units` MUST be an integer >= 0.
- `nonce` MUST be an integer >= 0; a newly initialized normal account SHOULD use zero unless a separately approved migration requires another value.
- Addresses MUST be unique.
- Accounts MUST be sorted by canonical address before commitment.
- The same account MUST NOT appear twice under aliases or alternate casing.
- The aggregate balance MUST satisfy the active monetary-policy/supply rules; final production ceiling is **TBD — requires governance and economic review**.

### 6.7 `validators`

- Type: array of objects.
- Required: MUST, including when empty.
- Each validator MUST contain exactly `validator_id`, `consensus_key`, and `stake_base_units` under the current schema.
- `validator_id` MUST be a stable non-empty ASCII identifier.
- `consensus_key` MUST be a non-empty canonical public-key encoding accepted by the selected Phase 10 consensus runtime. The exact production consensus-key encoding is **TBD — requires runtime/security review**; an empty or malformed key MUST be rejected.
- `stake_base_units` MUST be an integer >= 0.
- Validator IDs and consensus keys MUST be unique.
- Validator records MUST be sorted by `validator_id` before commitment.
- Voting power MUST be derived from authenticated stake state; a separate serialized voting-power authority is intentionally not introduced in this genesis schema.
- Minimum stake, maximum validator count, activation rules, and final validator economics remain **TBD — requires governance, economic, decentralization, and security review**.

### 6.8 `staking`

- Type: object.
- Required: MUST.
- It represents deterministic initial staking module state and/or a versioned parameter container.
- It MUST NOT contain final values for minimum stake, unbonding, redelegation, slashing, rewards, delegator rewards, or commission unless those values have been separately approved through governance/security/economic review.
- Unknown nested fields are not granted protocol authority merely because the JSON container is extensible; module-level semantic validation MUST reject unsupported fields for the active module version.
- Empty state is valid only when the selected module contract explicitly defines it as valid.

### 6.9 `governance`

- Type: object.
- Required: MUST.
- It represents deterministic initial governance state and/or a versioned parameter container.
- Final quorum, approval, veto, proposal deposit/refund, upgrade timelock, and emergency authority values are **TBD — requires governance, security, and legal review**.
- Proposal/vote state MUST be internally consistent if populated.
- Unknown nested fields MUST be rejected by the active module semantic validator.

### 6.10 `monetary_policy`

- Type: object.
- Required: MUST.
- It represents deterministic initial monetary-policy state and/or a versioned parameter container.
- It MUST NOT silently finalize total supply ceiling, inflation/emission, reward schedule, fee burn, minimum gas price, treasury share, allocation, or vesting.
- If a field is present, its meaning MUST be explicitly defined by the active monetary-policy version.
- Any supply/issuance rule MUST be consistent with account balances, validator stake, and all other genesis state.

## 7. Deterministic initial-state construction

The genesis loader MUST construct initial state in this order:

1. Validate JSON structure and reject duplicate keys.
2. Validate `protocol_version`.
3. Validate and canonicalize `network_name`.
4. Validate account records, uniqueness, and canonical order.
5. Validate validator records, uniqueness, keys, stake values, and canonical order.
6. Validate staking, governance, and monetary-policy module state.
7. Validate the historical lineage anchor.
8. Recompute `genesis_state_commitment`.
9. Recompute the deterministic `network_id` and compare it with the declared value.
10. Construct the initial application state in deterministic module order.
11. Enforce supply and authorization invariants.
12. Compute the derived genesis hash.
13. Persist/use the resulting state only if every validation step succeeds.

No local time, randomness, network response, filesystem enumeration order, map iteration order, or operator-provided hidden default may affect the result.

## 8. Genesis replay and cross-network isolation

A genesis MUST be bound to exactly one `network_id`. Transactions, blocks, P2P handshakes, consensus messages, and validator state derived from a different `network_id` MUST be rejected.

The canonical `network_id` MUST be carried into every domain where cross-network replay is possible. Domain separation alone is not sufficient when the network identity itself is inconsistent.

Legacy V1 numeric `chain_id` values MUST NOT be accepted as aliases for a Phase 9 `network_id` without an explicit migration adapter.

## 9. Validation and rejection rules

A node MUST reject genesis when any of the following occurs:

- missing required top-level field;
- unknown top-level field;
- malformed JSON or duplicate object key;
- unsupported `protocol_version`;
- invalid `network_name`;
- invalid `network_id` or derivation mismatch;
- invalid historical genesis hash;
- invalid genesis-state commitment;
- duplicate account address;
- invalid account address, balance, or nonce;
- duplicate validator ID or consensus key;
- invalid validator key or stake;
- invalid derived voting power or inconsistent stake-derived power;
- invalid staking state;
- invalid governance state;
- invalid monetary-policy state;
- supply ceiling/issuance invariant violation;
- inconsistent canonical serialization;
- any module-state commitment mismatch.

Schema validation is necessary but not sufficient. Semantic validation MUST execute after schema validation and before any state is activated.

## 10. Genesis upgrade and migration rules

A genesis change that modifies protocol semantics, state layout, network identity, or consensus-relevant initial state MUST use a new versioned migration boundary.

A migration MUST provide:
- source genesis hash;
- source protocol/schema version;
- destination schema version;
- deterministic transformation rules;
- destination `network_id` derivation and validation;
- source/destination state commitments;
- activation height or genesis cutover rule;
- rollback/failure behavior;
- reproducible migration evidence.

No migration may infer state from mutable external APIs or operator memory. A migration that changes `network_id` MUST be treated as a new network identity unless a future protocol explicitly specifies a safe identity-preserving transition.

## 11. Required negative test vectors

The Phase 9.2 validation suite MUST include at least:

| ID | Negative case | Expected result |
|---|---|---|
| G-NEG-001 | Missing required field | Reject before state activation |
| G-NEG-002 | Unknown top-level field | Reject |
| G-NEG-003 | Invalid `network_id` | Reject |
| G-NEG-004 | Invalid `protocol_version` | Reject |
| G-NEG-005 | Duplicate account address | Reject |
| G-NEG-006 | Invalid public/consensus key | Reject |
| G-NEG-007 | Invalid account balance | Reject |
| G-NEG-008 | Invalid validator entry | Reject |
| G-NEG-009 | Duplicate validator | Reject |
| G-NEG-010 | Invalid derived voting power | Reject |
| G-NEG-011 | Invalid staking state | Reject |
| G-NEG-012 | Invalid governance state | Reject |
| G-NEG-013 | Invalid monetary policy | Reject |
| G-NEG-014 | Supply ceiling violation | Reject |
| G-NEG-015 | Invalid genesis-state commitment | Reject |
| G-NEG-016 | Invalid historical genesis hash | Reject |
| G-NEG-017 | Inconsistent canonical serialization | Reject |

These are required test cases, not evidence that the current repository already implements the validation runtime.

## 12. Production-parameter boundary

The following remain explicitly unresolved:

- minimum validator stake;
- maximum validator count;
- unbonding period;
- inflation/emission;
- validator and delegator rewards;
- commission bounds;
- slashing/downtime parameters;
- fee burn;
- minimum gas price;
- block gas limit;
- governance quorum/approval/veto thresholds;
- proposal deposits/refunds;
- upgrade timelocks;
- emergency halt authority;
- treasury controls;
- genesis allocation/vesting/distribution;
- final SYJ supply/issuance policy.

Each is **TBD — requires governance, economic, security, or legal review**.

## 13. Review checklist

- [ ] Top-level fields match `protocol/schemas/genesis-v2.json` exactly.
- [ ] `network_id` is canonical and derived deterministically.
- [ ] Genesis commitment is domain-separated and excludes itself from its preimage.
- [ ] Derived genesis hash is deterministic.
- [ ] Account and validator ordering is canonical.
- [ ] Duplicate and malformed entries fail closed.
- [ ] No mainnet genesis is created.
- [ ] No final economic or distribution parameters are selected.
- [ ] Negative vectors G-NEG-001 through G-NEG-017 are specified.
