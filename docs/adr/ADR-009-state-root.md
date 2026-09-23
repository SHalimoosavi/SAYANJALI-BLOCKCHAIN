# ADR-009: Authenticated State Root

**Status:** Draft
**Target protocol version:** Phase 9 / target protocol version 2.x
**Scope:** Target authenticated state commitment for finalized production blocks.

> Phase 9 is a protocol-formalization phase. It specifies contracts and architecture; it does not implement PoS/BFT, staking, validator lifecycle, governance execution, a state tree, smart contracts, or a production network.


## Context

The current V2 foundation can reconstruct balances/nonces by replaying blocks. That is useful for a reference implementation but does not by itself provide a compact authenticated commitment to arbitrary application state.

## Problem

Production validators, light clients, snapshots, and migration tooling need a deterministic authenticated state root covering accounts, balances, nonces, validators, staking, governance, module state, and issuance accounting.

## Decision

**PROPOSED:** use a versioned authenticated key-value state tree whose root is committed in every finalized production block. The exact tree implementation is intentionally not selected in Phase 9.

Conceptual namespaces:

```text
acct/<address>          -> balance, nonce, account metadata
val/<validator>         -> validator lifecycle and consensus key
stake/<delegator>/<val> -> delegation records
gov/<proposal>          -> governance state
mod/<module>/<key>      -> module state
sys/supply              -> issuance/supply accounting
sys/version             -> state schema version
```

The root MUST be deterministic, canonical, and independently recomputable. State proofs MUST bind to the finalized block commitment.

## Alternatives

1. Replay-only state — simple, but expensive for arbitrary proofs and recovery.
2. Unauthenticated database snapshot — operationally convenient, but not a consensus commitment.
3. A specific Merkle tree implementation — viable, but premature before benchmark/security review.
4. Authenticated versioned state tree — selected as the architectural direction.

## Security consequences

The root creates an auditable commitment and supports light-client proofs, but implementation bugs could fork consensus. The tree encoding, key ordering, hashing domains, proof validation, and snapshot integrity therefore require independent testing and audit.

## Operational consequences

Validators need snapshot/replay tooling, root verification, state migration tooling, pruning policy, and archive-node procedures.

## Migration consequences

Historical V2 state can be imported through a deterministic migration at a defined boundary. The migration MUST produce a reproducible initial production state root. No mainnet genesis is created in Phase 9.

## Invariants

I-005, I-006, I-012, I-014, I-015.

## Open questions

Tree algorithm, leaf schema, proof size, snapshot format, pruning cadence, historical retention, migration root, and performance targets are TBD.

## Required tests

Differential replay vs tree; proof verification; random state mutation/property tests; snapshot restore; migration determinism; corrupted-root rejection; long-run state growth.

## Review checklist

- [ ] State root is specified, not implemented.
- [ ] All consensus-relevant domains are covered.
- [ ] Migration is deterministic.
- [ ] No specific tree implementation is falsely presented as selected.
