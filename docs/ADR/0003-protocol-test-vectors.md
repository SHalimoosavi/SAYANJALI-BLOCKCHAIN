# ADR 0003 --- Protocol Test Vectors

**Status:** Accepted\
**Date:** 2026-09-04

## Decision

Deterministic protocol test vectors will be the compatibility contract
between Python and Go.

## Required vector families

-   genesis
-   blocks
-   transactions
-   signatures
-   Merkle roots
-   difficulty
-   chain work
-   monetary rules
-   addresses
-   serialization
-   networking

## Rules

-   Python may generate the initial expected vectors.
-   Vectors are committed to the repository.
-   Go tests consume the same vectors.
-   Python re-verifies its own vectors.
-   A Python/Go mismatch blocks compatibility.
-   Vectors must not be silently regenerated to hide an implementation
    mismatch.

## Future extension

Vector files should carry explicit protocol and serialization versions
so future protocol changes can coexist with historical vectors.
