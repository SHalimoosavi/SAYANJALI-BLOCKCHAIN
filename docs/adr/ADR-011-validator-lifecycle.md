# ADR-011: Validator Lifecycle

**Status:** Draft
**Target protocol version:** Phase 9 / target protocol version 2.x
**Scope:** Formal lifecycle and key-separation contract for future validators.

> Phase 9 is a protocol-formalization phase. It specifies contracts and architecture; it does not implement PoS/BFT, staking, validator lifecycle, governance execution, a state tree, smart contracts, or a production network.


## Context

Consensus security requires a deterministic validator lifecycle and explicit handling of downtime, equivocation, key compromise, and unbonding.

## Decision

**PROPOSED:** use the lifecycle:

`Candidate -> Active -> Jailed -> Unjailed -> Active`

with terminal or delayed paths:

`Active -> Unbonding -> Removed`
`Jailed -> Tombstoned -> Removed`
`Candidate -> Rejected`

Transitions MUST be triggered by authenticated state/evidence and activate at deterministic boundaries.

## Key compromise

Consensus-key compromise MUST be handled through evidence, rotation, and emergency procedures that cannot authorize arbitrary historical finalization. A governance/treasury key compromise MUST be treated separately.

## Emergency removal

Emergency validator removal, if retained, MUST have a narrowly defined trigger, authenticated evidence, auditable authority, and deterministic activation. Exact authority is TBD.

## Alternatives

- Informal operator-controlled lifecycle: rejected as a protocol contract because it is not deterministic.
- Pure governance-only lifecycle: too slow for evidence-driven safety events.
- Automated evidence-driven lifecycle with governance-defined parameters: proposed.

## Security consequences

A precise lifecycle makes slashing and recovery testable. It also creates complex state transitions that must be fuzzed and audited.

## Operational consequences

Operators need key rotation, evidence submission, jail recovery, monitoring, and secure custody procedures.

## Migration consequences

No production validator state exists in the current PoW/V2 foundation. A production genesis or migration must create validator records explicitly.

## Open questions

Jail durations, tombstone rules, downtime windows, maximum validators, minimum stake, activation epochs, and emergency authority are TBD.

## Required tests

Lifecycle transition matrix; duplicate evidence; replayed evidence; key rotation; downtime; equivocation; mass outage; partition recovery; governance override attempts.

## Review checklist

- [ ] Key classes remain separate.
- [ ] Lifecycle transitions are deterministic.
- [ ] No validator implementation is included.
