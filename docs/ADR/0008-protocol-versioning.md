# ADR 0008 --- Protocol Versioning

**Status:** Accepted\
**Date:** 2026-09-04

## Decision

SAYANJALI BLOCKCHAIN distinguishes: - implementation version; - protocol
version; - network identifier/version; - serialization version.

The current P2P protocol is `sayanjali-p2p` version `1.0`.

A breaking protocol change requires an explicit version transition and
compatibility plan.

Changing Python to Go does not change protocol version.
