# ADR 0004 --- Native SYJ Monetary Representation

**Status:** Accepted\
**Date:** 2026-09-04

## Decision

The authoritative SYJ monetary representation is integer base units.

`1 SYJ = 100,000,000 base units`

Maximum supply:

`720,000,000 SYJ = 72,000,000,000,000,000 base units`

## Rationale

Floating-point monetary state is unsuitable for consensus-critical
accounting. The current Python Phase 2 implementation already migrated
authoritative monetary state to integer base units.

## Compatibility

Go must use integer arithmetic for: - transaction amounts - balances -
coinbase rewards - supply - state transitions - persistence -
serialization - RPC monetary fields where protocol values are
represented

Human-readable decimals remain presentation-only.
