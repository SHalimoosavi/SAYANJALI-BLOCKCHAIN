# SYJ Protocol Overview

**Status:** Phase 9.2 contract remediation / freeze candidate
**Target protocol version:** Phase 9 / target protocol version 2.x
**Canonical protocol identifier:** `network_id`
**Scope:** Formal boundary between the historical V1 compatibility track, the implemented Go V2 foundation, the Phase 9 specification contract, and the proposed production runtime.

> **SYJ is NOT PRODUCTION-READY and NOT MAINNET-READY.** Phase 9.2 closes documentation and schema conditions identified during Phase 9.1 review. It does not implement the production consensus runtime, staking, validator lifecycle, governance execution, state tree, smart contracts, public network, or mainnet.

## 1. Purpose and maturity

The audited Phase 9 baseline is the main merge commit `75571c2a6fcbdbbf506d703ea379a3d9b8a4b2d8`, with Phase 9 specification commit `aef1a4944674e77c8745d5a20bdccf8a5d7ca11a`. Phase 9.2 is a documentation/schema remediation package intended to make the Phase 9 contract internally traceable and freezeable before implementation work begins.

| Track | Status | Meaning |
|---|---|---|
| V1 | Historical / frozen compatibility / reference | Preserve historical behavior and regression evidence. V1 `chain_id` configuration remains a historical compatibility artifact and is not the Phase 9 canonical identifier. |
| V2 | Implemented Go protocol foundation / migration-research track | The audited Go foundation contains V2 transaction identity, `network_id` binding, nonce/replay controls, and related P2P/network separation. |
| Phase 9 | Formal protocol specification contract | Defines normative target behavior, unresolved parameters, security boundaries, and migration constraints. |
| Phase 9.2 | Contract remediation / freeze candidate | Corrects normative section references, freezes identifier terminology, adds the genesis contract, and defines the Phase 10 implementation boundary. |
| Cosmos SDK + CometBFT | PROPOSED / NOT SHIPPED / NOT IMPLEMENTED | Proposed production runtime only; no integration code is introduced by Phase 9.2. |
| Current production status | **NOT PRODUCTION-READY / NOT MAINNET-READY** | No production launch claim is made. |

The current PoW implementation is **Legacy / transitional / protocol-research track — not the production consensus target.** Its executable behavior remains useful as a compatibility/reference track but is not evidence of a production PoS/BFT network.

## 2. Canonical protocol identifier

`network_id` is the canonical SYJ protocol network identifier.

The following rules are normative for the Phase 9 target:

1. Every consensus-relevant transaction, block, genesis, P2P identity context, and consensus-critical message MUST bind to exactly one `network_id`.
2. `network_id` MUST be represented as exactly 64 lowercase hexadecimal characters (32 bytes).
3. A declared `network_id` MUST equal the deterministic value derived from the approved network-identity inputs. It MUST NOT be independently selected in a way that creates a second identity.
4. The audited V2 EffectiveNetworkID derivation remains the compatibility reference: canonical inputs are `consensus_protocol_version`, `genesis_state_commitment`, `historical_genesis_hash`, and `network_name`, serialized with the repository's deterministic canonical-JSON rules and hashed under `SYJ-EFFECTIVE-NETWORK-ID-V1\0`.
5. The current audited private-testnet V2 value `237a1934769295c63fe47771a4996b17c3899e52f6bacf79ed7edefec90eaaf3` remains a compatibility/test artifact. It is NOT a production or mainnet identity.
6. Cosmos SDK/CometBFT `chain-id` is an external runtime compatibility representation only. ADR-013 defines its deterministic mapping from `network_id`; the two values MUST NOT be treated as independent identities.

### 2.1 Historical V1 boundary

Pre-Phase-9 V1 code and documentation may contain a numeric `chain_id`. Those references are historical/frozen compatibility material. They MUST NOT be used as the canonical Phase 9 identity and MUST NOT be silently interpreted as a second production identity. Phase 9 migration tooling MUST explicitly map or retire such legacy configuration.

## 3. Current V2 facts verified from the baseline

The audited Go V2 transaction implementation defines version `2`, a 64-character lowercase hexadecimal `network_id`, sender/receiver addresses, amount in base units, nonce, timestamp, raw 128-hex-character public key, raw 128-hex-character signature, and a 64-hex-character `tx_id`. The signing payload includes `network_id`; the signing domain is `SYJ-TX-SIGN-V2\0`; transaction identity uses `SYJ-TX-ID-V2\0` and excludes the signature.

The audited private-testnet vector binds the V2 identity to the historical genesis hash and GenesisState commitment. Its EffectiveNetworkID is the compatibility value stated above. The vector is not a mainnet identity.

Current V2 validation verifies version, network identity, amount, address validity, sender/public-key binding, ECDSA signature, and signature-independent transaction ID. Nonce and confirmed-transaction replay protection are enforced by the V2 chain/mempool paths rather than by the transaction object alone. Phase 9.2 MUST NOT silently change those frozen V2 wire vectors.

## 4. Phase 9 and Phase 9.2 boundary

Phase 9 defines:
- canonical transaction and block contracts;
- deterministic state-transition semantics;
- authenticated state-root architecture;
- PoS/BFT target behavior;
- validator lifecycle and key separation;
- staking, fee, and governance semantics;
- threat model and launch gates;
- JSON schemas and protocol metadata.

Phase 9.2 specifically closes the Phase 9.1 conditions:
- exact invariant-to-section traceability;
- canonical `network_id` terminology and explicit Cosmos/CometBFT mapping;
- a normative field-level genesis contract aligned with `genesis-v2.json`;
- a frozen Phase 10 prototype boundary.

Phase 9.2 does **not** add runtime implementation. Any normative `MUST`/`MUST NOT` statement in these documents is a target protocol rule unless the text explicitly identifies an already-implemented V2 behavior.

## 5. Target architecture

```text
                 Wallet / CLI / RPC clients
                           |
                    API / Query boundary
                           |
             +-------------v-------------+
             |       Cosmos SDK          |  <- PROPOSED
             | deterministic application |
             +-------------+-------------+
                           |
                  ABCI / consensus API
                           |
             +-------------v-------------+
             |        CometBFT           |  <- PROPOSED
             | proposer / prevote /      |
             | precommit / commit        |
             +-------------+-------------+
                           |
                    authenticated P2P
                           |
              validator nodes / sentries
```

Cosmos SDK + CometBFT remains **PROPOSED / NOT SHIPPED / NOT IMPLEMENTED**. Phase 10 is explicitly constrained to a local 4–7 validator prototype and may begin only after the Phase 9 contract is accepted.

## 6. Specified, implemented, partial, missing

| Capability | Status |
|---|---|
| V2 transaction identity and signing | IMPLEMENTED in current Go foundation |
| V2 `network_id` binding | IMPLEMENTED in current Go foundation |
| V2 nonce/replay checks | IMPLEMENTED in current Go foundation |
| Authenticated production state root | SPECIFIED; NOT IMPLEMENTED |
| PoS validator set | PROPOSED/SPECIFIED; NOT IMPLEMENTED |
| BFT finality | PROPOSED/SPECIFIED; NOT IMPLEMENTED |
| Staking/slashing | SPECIFIED; NOT IMPLEMENTED |
| Governance execution | SPECIFIED; NOT IMPLEMENTED |
| Production monetary policy | TBD — requires governance, economic, security, and legal review |
| Cosmos SDK integration | PROPOSED; NOT SHIPPED; NOT IMPLEMENTED |
| CometBFT integration | PROPOSED; NOT SHIPPED; NOT IMPLEMENTED |
| Mainnet genesis | NOT CREATED |

## 7. Normative model

**MUST** is mandatory for validity. **MUST NOT** is a prohibition. **SHOULD** is a strong default that requires documented justification to deviate. **MAY** is optional. Any unresolved value explicitly marked **TBD — requires governance, economic, security, or legal review** MUST NOT be treated as a final production parameter.

## 8. Global invariants and traceability

The following 17 invariants are the Phase 9 contract. The references below point to the actual normative sections in the current Phase 9 documents and are intentionally aligned with `REVIEW_CHECKLIST.md`.

| ID | Invariant | Normative specification reference |
|---|---|---|
| I-001 | Every valid transaction has exactly one protocol version. | `transaction-spec.md` §5 (Validation rules) |
| I-002 | Every transaction belongs to exactly one network identity. | `transaction-spec.md` §5; `ADR-013-network-identifier.md` §6 |
| I-003 | Every normal account has exactly one next nonce. | `state-transition-spec.md` §5 |
| I-004 | A transaction cannot consume a nonce twice. | `transaction-spec.md` §7; `state-transition-spec.md` §5 |
| I-005 | Total supply cannot exceed the monetary-policy ceiling. | `state-transition-spec.md` §13; `staking-spec.md` §11 |
| I-006 | State transition is deterministic. | `state-transition-spec.md` §4 |
| I-007 | Finalized blocks cannot be reverted under normal protocol rules. | `consensus-spec.md` §9 |
| I-008 | Validator voting power is derived from authenticated stake state. | `validator-spec.md` §4; `staking-spec.md` §3 |
| I-009 | Validator-set transitions are deterministic. | `validator-spec.md` §5; `consensus-spec.md` §8 |
| I-010 | Consensus cannot depend on local wall-clock behavior. | `consensus-spec.md` §6; `block-spec.md` §3 |
| I-011 | Consensus-critical serialization is versioned and deterministic. | `block-spec.md` §8; `transaction-spec.md` §3 |
| I-012 | Genesis uniquely determines initial network state. | `genesis-spec.md` §§3–9; `state-transition-spec.md` §3 |
| I-013 | Protocol upgrades require explicit version/governance rules. | `governance-spec.md` §6; `consensus-spec.md` §13 |
| I-014 | Application state changes only through consensus-authorized execution. | `state-transition-spec.md` §§1,4,9 |
| I-015 | No account, validator, module, governance proposal, or treasury control may mint SYJ outside the specified issuance rules. | `staking-spec.md` §11; `state-transition-spec.md` §13 |
| I-016 | No single administrative key may unilaterally upgrade consensus, modify supply, or finalize arbitrary state changes. | `governance-spec.md` §10; `validator-spec.md` §10 |
| I-017 | All consensus-critical messages must be authenticated, replay-protected, and bound to the correct `network_id`. | `consensus-spec.md` §11; `transaction-spec.md` §5; `ADR-013-network-identifier.md` §6 |

## 9. Compatibility and migration

V1 historical material is not rewritten as if it were Phase 9. V2 wire semantics are not silently changed. A production migration MUST define an approved source state, destination genesis, deterministic transformation, versioned schemas, explicit `network_id`, validation commitments, cutover height, rollback/failure behavior, and compatibility window.

The private-testnet V2 identity MUST NOT be reused as a production identity unless a future governance-approved migration explicitly establishes that identity; Phase 9.2 does not do so.

Legacy V1 `chain_id` configuration MAY remain in historical compatibility code until migration, but it MUST be treated as legacy configuration and MUST NOT be exposed as an independent Phase 9 identity.

## 10. Security assumptions and failure modes

The target protocol assumes authenticated cryptographic identities, deterministic application execution, Byzantine-aware validator operations, secure key custody, replay protection, and monitored infrastructure. These are requirements and assumptions, not evidence that the current repository already satisfies them.

Relevant failure modes include malformed transactions, wrong `network_id`, nonce conflicts, invalid signatures, invalid blocks, inconsistent state commitments, unavailable validators, partitions, conflicting upgrades, compromised keys, malicious genesis data, and governance capture. Invalid consensus inputs MUST fail closed at the relevant boundary.

## 11. Required test vectors

At minimum:
- current V2 canonical transaction vector;
- signature-independent transaction-ID vector;
- V2 EffectiveNetworkID derivation vector;
- wrong-network rejection vector;
- genesis field/schema negative vectors;
- genesis commitment and canonical-serialization vectors;
- state-transition differential vectors;
- block/state-root vectors;
- validator-set transition vectors;
- equivocation/evidence vectors;
- partition/restart scenarios;
- upgrade-version vectors;
- supply-ceiling and issuance vectors.

## 12. Open questions

Economic parameters, validator-set size, unbonding, reward schedule, fee burn, governance thresholds, emergency authority, maximum transaction/block sizes, state-tree implementation, archive policy, and upgrade timelocks remain **TBD — requires governance, economic, security, or legal review** where not otherwise resolved by a normative technical contract.

## 13. Review checklist

- [ ] SYJ is explicitly **NOT PRODUCTION-READY** and **NOT MAINNET-READY**.
- [ ] V1 is historical/frozen compatibility; V2 is the Go foundation/migration-research track.
- [ ] Phase 9 is the formal specification contract.
- [ ] Phase 9.2 closes the Phase 9.1 remediation conditions.
- [ ] PoW is explicitly legacy/transitional/research, not the production consensus target.
- [ ] Cosmos SDK + CometBFT remains **PROPOSED / NOT SHIPPED / NOT IMPLEMENTED**.
- [ ] I-001 through I-017 reference actual normative sections.
- [ ] `network_id` is canonical and `chain-id` is compatibility-only.
- [ ] No implementation or mainnet genesis is introduced.
