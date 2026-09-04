# ADR 0001 --- Python Reference / Go Production Architecture

**Status:** Accepted\
**Date:** 2026-09-04

## Decision

The existing Python implementation remains the reference/protocol
implementation. Go will become the future production node/core
implementation.

## Rationale

The Python implementation already contains working blockchain, monetary,
consensus, wallet, storage, API and P2P behavior. Rewriting it in
another language without a compatibility boundary would risk silently
changing protocol behavior.

Go is selected for the production core because the target architecture
requires a long-running, concurrent node with explicit ownership of
consensus, state, P2P, storage and observability.

## Consequences

-   Python remains maintained as an oracle and research implementation.
-   Go must pass the same deterministic protocol vectors.
-   Cross-language compatibility becomes an explicit engineering gate.
-   Working Python modules are not rewritten for style alone.
