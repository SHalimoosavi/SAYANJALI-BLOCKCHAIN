# ADR 0006 --- State Transition Architecture

**Status:** Accepted\
**Date:** 2026-09-04

## Decision

The production architecture follows:

`Transaction -> Validation -> State Transition -> Ledger State -> Block Validation -> Persistence`

Consensus validation remains authoritative over whether a block is
acceptable. State transition remains authoritative over balances and
supply.

A state root is not introduced by this ADR.
