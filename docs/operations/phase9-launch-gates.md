# Phase 9 Launch Gates

**Status:** Draft
**Target protocol version:** Phase 9 / target protocol version 2.x
**Scope:** Objective evidence required before moving from specification to prototype, public testnet, and mainnet candidate.

> Phase 9 is a protocol-formalization phase. It specifies contracts and architecture; it does not implement PoS/BFT, staking, validator lifecycle, governance execution, a state tree, smart contracts, or a production network.


## Gate 0 — Phase 9 Specification Freeze

**Objective:** establish a reviewable protocol contract.

Required evidence:
- all Phase 9 documents reviewed;
- I-001 through I-017 traceable;
- schemas parse and validate;
- economic/governance parameters explicitly TBD;
- threat model reviewed;
- ADRs accepted or annotated;
- no implementation code included.

**Status:** NOT PASSED.

## Gate 1 — Private Prototype

**Objective:** implement only after the specification is frozen.

Required evidence:
- proposed runtime selected and version-pinned;
- deterministic state machine prototype;
- 4–7 validator local/private network;
- validator key separation;
- state-root implementation;
- transaction/block differential tests;
- restart and partition tests;
- evidence/slashing simulation;
- reproducible genesis.

**Status:** NOT PASSED.

## Gate 2 — Local Multi-Validator Testnet

Required evidence:
- 4–7 independent validator processes;
- automated validator-set transition tests;
- proposer failure and recovery;
- equivocation evidence;
- network partition tests;
- state snapshot/restore;
- monitoring and incident procedures;
- invariant/property/fuzz suite.

**Status:** NOT PASSED.

## Gate 3 — Public Testnet

A public testnet MUST NOT launch before secure P2P and validator identity separation are demonstrated.

Required evidence:
- independent external validators;
- documented operational runbooks;
- secure P2P transport/identity;
- RPC abuse controls;
- monitoring and alerting;
- incident response rehearsal;
- upgrade rehearsal;
- public documentation;
- economic/staking simulations;
- external security review of critical consensus components.

**Status:** NOT PASSED.

## Gate 4 — Mainnet Readiness

Mainnet is not ready unless all of the following have objective evidence:
- independent security audits;
- external validator participation;
- governance rehearsal;
- monitoring and alerting;
- incident response;
- key compromise drills;
- secure genesis/custody approval;
- audited monetary policy;
- finalized validator/staking/governance parameters;
- upgrade and rollback plan;
- legal/regulatory review;
- operational disaster recovery;
- reproducible release artifacts;
- supply and state-root verification;
- successful public-testnet history.

**Status:** NOT PASSED.

## Hard prohibitions

- No PoS implementation before Phase 9 specs are frozen.
- No token launch before monetary policy is audited and approved.
- No public testnet before secure P2P and validator separation.
- No mainnet before independent audits, external validators, governance rehearsal, monitoring, incident response, and legal review.
- No mainnet genesis or token distribution is created in Phase 9.

## Review checklist

- [ ] No future gate is marked passed.
- [ ] Evidence requirements are measurable.
- [ ] Legal and operational controls are explicit.
