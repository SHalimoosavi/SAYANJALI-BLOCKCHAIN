# ADR 0010 --- Testnet Strategy

**Status:** Accepted\
**Date:** 2026-09-04

## Decision

The production transition follows:

`LOCAL TESTNET -> PRIVATE SYJ TESTNET -> PUBLIC TESTNET -> MAINNET READINESS -> SYJ MAINNET`

Private testnet readiness requires correctness before performance
claims.

Required private-testnet coverage includes: - discovery - peer
authentication - transaction propagation - block propagation -
synchronization - chain convergence - forks/reorgs - offline recovery -
malformed messages - invalid transactions/blocks - replay - supply
manipulation - consensus manipulation - resource exhaustion

No TPS or performance figure may be published without a reproducible
benchmark.
