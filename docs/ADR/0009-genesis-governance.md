# ADR 0009 --- Genesis Governance

**Status:** Accepted\
**Date:** 2026-09-04

## Decision

Genesis is protocol-critical and must be deterministic and
version-controlled.

The current implementation defines deterministic genesis construction,
but does not define final genesis allocation/token distribution.

No final allocation may be invented during the Python → Go transition.

Any future genesis allocation requires: - explicit
governance/approval; - deterministic configuration; - committed
source/vector; - compatibility impact analysis; - a deliberate
network/protocol version decision where necessary.
