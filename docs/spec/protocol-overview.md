# SYJ Protocol Overview

**Status:** Draft
**Target protocol version:** Phase 9 / target protocol version 2.x
**Scope:** Formal boundary between the historical V1 reference track, the implemented Go V2 foundation, and the proposed production architecture.

> Phase 9 is a protocol-formalization phase. It specifies contracts and architecture; it does not implement PoS/BFT, staking, validator lifecycle, governance execution, a state tree, smart contracts, or a production network.


## 1. Purpose and maturity

The authoritative repository baseline for this document is commit `5e02557363be6319cca06cd7e20a8be906265060`. Phase 8.1 V2 is an implemented Go protocol foundation. Phase 9 freezes a specification contract before any production-consensus implementation.

| Track | Status | Meaning |
|---|---|---|
| V1 | Historical / frozen compatibility / reference | Preserve for compatibility and regression evidence; not the production target. |
| V2 | Implemented Go protocol foundation / migration-research track | Verified transaction, network-identity, replay, nonce, chain, and P2P separation exists in the Go repository. |
| Phase 9 | Formal specification contract | This deliverable defines the target architecture and unresolved parameters. |
| Cosmos SDK + CometBFT | Proposed | Target production runtime; not shipped or integrated. |
| Current production status | **NOT PRODUCTION-READY** | No public mainnet claim is made. |

The current PoW chain is **Legacy / transitional / protocol-research track — not the production consensus target.** The existing implementation provides valuable executable reference behavior, but it is not evidence that a PoS/BFT production network exists.

## 2. Current V2 facts verified from the baseline

The Go V2 transaction implementation defines version `2`, a 64-character lowercase hexadecimal network identity, sender/receiver addresses, amount in base units, nonce, timestamp, raw 128-hex-character public key, raw 128-hex-character signature, and a 64-hex-character `tx_id`. The signing payload is canonical JSON with the fields `amount_base_units`, `network_id`, `nonce`, `receiver`, `sender`, `sender_public_key`, `timestamp`, and `version`. Signing bytes use `SYJ-TX-SIGN-V2\0`; transaction identity uses `SYJ-TX-ID-V2\0` and excludes the signature. These are compatibility facts, not new production choices.

The verified private-testnet vector binds the V2 identity to the historical genesis hash and GenesisState commitment. Its EffectiveNetworkID is `237a1934769295c63fe47771a4996b17c3899e52f6bacf79ed7edefec90eaaf3`, and its P2P identity is `syjnet-v2-` followed by that value. The vector is a test artifact, not a mainnet identity.

Current V2 validation verifies version, network identity, positive amount, address validity, sender/public-key binding, ECDSA signature, and signature-independent transaction ID. Nonce and confirmed-transaction replay protection are enforced by the V2 chain/mempool paths rather than by the transaction object alone. The current V2 transaction implementation does not contain the Phase 9 production fee or expiry fields; those are specified as future protocol-contract requirements and must not be silently backported into the frozen V2 wire format.

## 3. Phase 9 boundary

Phase 9 specifies:
- canonical transaction and block contracts;
- deterministic state-transition semantics;
- authenticated state-root architecture;
- PoS/BFT target behavior;
- validator lifecycle and key separation;
- staking, fee, and governance semantics;
- threat model and launch gates;
- JSON schemas and protocol metadata.

Phase 9 does **not** implement those target systems. A document saying “MUST” describes the future protocol contract unless it is explicitly labeled as current V2 behavior.

## 4. Target architecture

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

The application state machine owns deterministic state transition rules. CometBFT supplies the proposed BFT consensus runtime and finality mechanism. Validator keys, P2P keys, account keys, and governance/treasury keys are separated. The existing Go/Python tracks remain reference/research artifacts during migration.

## 5. Specified, implemented, partial, missing

| Capability | Status |
|---|---|
| V2 transaction identity and signing | IMPLEMENTED in current Go foundation |
| V2 network identity binding | IMPLEMENTED in current Go foundation |
| V2 nonce/replay checks | IMPLEMENTED in current Go foundation |
| Authenticated production state root | SPECIFIED; NOT IMPLEMENTED |
| PoS validator set | PROPOSED/SPECIFIED; NOT IMPLEMENTED |
| BFT finality | PROPOSED/SPECIFIED; NOT IMPLEMENTED |
| Staking/slashing | SPECIFIED; NOT IMPLEMENTED |
| Governance execution | SPECIFIED; NOT IMPLEMENTED |
| Production monetary policy | TBD; NOT FINAL |
| Cosmos SDK integration | PROPOSED; NOT SHIPPED |
| CometBFT integration | PROPOSED; NOT SHIPPED |
| Mainnet genesis | NOT CREATED |

## 6. Normative model

Normative terms mean: **MUST** is required for validity; **SHOULD** is a strong default that requires documented justification to deviate; **MAY** is optional. “TBD — requires governance and economic review” means no implementation may treat the value as final.

## 7. Global invariants

The following 17 invariants are the Phase 9 contract. Their detailed traceability is maintained in `REVIEW_CHECKLIST.md`.

| ID | Invariant | Primary reference |
|---|---|---|
| I-001 | Every valid transaction has exactly one protocol version. | transaction-spec.md §6 |
| I-002 | Every transaction belongs to exactly one network identity. | transaction-spec.md §6 |
| I-003 | Every normal account has exactly one next nonce. | state-transition-spec.md §7 |
| I-004 | A transaction cannot consume a nonce twice. | transaction-spec.md §8 |
| I-005 | Total supply cannot exceed the monetary-policy ceiling. | state-transition-spec.md §12 |
| I-006 | State transition is deterministic. | state-transition-spec.md §5 |
| I-007 | Finalized blocks cannot be reverted under normal protocol rules. | consensus-spec.md §9 |
| I-008 | Validator voting power is derived from authenticated stake state. | validator-spec.md §7 |
| I-009 | Validator-set transitions are deterministic. | validator-spec.md §8 |
| I-010 | Consensus cannot depend on local wall-clock behavior. | consensus-spec.md §6 |
| I-011 | Consensus-critical serialization is versioned and deterministic. | block-spec.md §8 |
| I-012 | Genesis uniquely determines initial network state. | state-transition-spec.md §6 |
| I-013 | Protocol upgrades require explicit version/governance rules. | governance-spec.md §8 |
| I-014 | Application state changes only through consensus-authorized execution. | state-transition-spec.md §10 |
| I-015 | No account, validator, module, governance proposal, or treasury control may mint SYJ outside the specified issuance rules. | staking-spec.md §11 |
| I-016 | No single administrative key may unilaterally upgrade consensus, modify supply, or finalize arbitrary state changes. | governance-spec.md §9 |
| I-017 | All consensus-critical messages must be authenticated, replay-protected, and bound to the correct chain/network identity. | consensus-spec.md §11 |

## 8. Compatibility and migration

The V1 historical track is not rewritten. V2 wire semantics are not silently changed by Phase 9. A production migration requires a separately approved genesis/state migration, versioned transaction/block schemas, explicit network identity, and a documented cutover rule. The private-testnet V2 identity MUST NOT be reused as a production identity.

## 9. Security assumptions

Phase 9 assumes authenticated cryptographic identities, deterministic application execution, Byzantine-aware validator operations, secure key custody, replay protection, and independently monitored infrastructure. These are requirements and assumptions, not evidence that the current repository already satisfies them.

## 10. Failure modes

Relevant failures include malformed transactions, wrong network identity, nonce conflicts, invalid signatures, invalid blocks, inconsistent state roots, unavailable validators, partitioned networks, conflicting upgrades, compromised keys, malicious genesis data, and governance capture. Each failure MUST fail closed at the relevant consensus boundary or enter an explicitly specified recovery mode.

## 11. Required test vectors

At minimum:
- current V2 canonical transaction vector;
- signature-independent tx-ID vector;
- network-identity derivation vector;
- state-transition differential vectors;
- block/state-root vectors;
- validator-set transition vectors;
- equivocation/evidence vectors;
- partition and restart scenarios;
- upgrade-version vectors;
- supply-ceiling and issuance vectors.

## 12. Open questions

Economic parameters, validator-set size, unbonding, reward schedule, fee burn, governance thresholds, emergency authority, maximum transaction/block sizes, state-tree implementation, archive policy, and upgrade timelocks remain **TBD — requires governance and economic review** or technical review as explicitly identified in the relevant documents.

## 13. Review checklist

- [ ] Current V2 behavior is distinguished from proposed production behavior.
- [ ] PoW is labeled legacy/transitional/research.
- [ ] No production readiness is implied.
- [ ] All I-001 through I-017 have references.
- [ ] No final economic parameter has been silently selected.
