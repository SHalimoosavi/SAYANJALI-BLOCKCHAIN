# ADR 0002 --- Protocol Preservation

**Status:** Accepted\
**Date:** 2026-09-04

## Decision

Changing programming language does not constitute a protocol change.

The behavior discovered in the Python reference at Phase 3 commit
`ead5330986debbc6420bdd1d392d81051eac8f22` is preserved unless a
separately approved protocol change is documented.

## Preserved areas

-   block hash inputs and serialization
-   transaction hash/signing semantics
-   SECP256k1 wallet/address behavior
-   Merkle calculation
-   PoW difficulty semantics
-   chain-work calculation
-   chain selection
-   monetary base units and maximum supply
-   coinbase rules
-   genesis construction
-   current state-transition behavior
-   current P2P authentication semantics

## Consequence

Any intentional divergence must be explicit, versioned, vectorized and
covered by an ADR.
