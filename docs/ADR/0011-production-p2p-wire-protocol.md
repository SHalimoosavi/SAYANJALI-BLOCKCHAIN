# ADR 0011 — Production P2P Wire Protocol

**Status:** Accepted / Frozen
**Date:** 2026-09-05

## Decision

The production SAYANJALI P2P protocol is frozen as `sayanjali-p2p` version `1.0` by `protocol/p2p-wire-spec.md`.

The transport profile is long-lived TCP with a 20-byte fixed frame header and bounded binary payloads. The eleven message types are assigned stable numeric values 1–11. Request/response correlation uses a connection-local uint64 request ID. Consensus objects remain separate from network framing and are carried as bounded opaque consensus-encoded byte strings.

The handshake uses HELLO / HELLO_ACK with fresh connection challenges and the existing P2P secp256k1 identity domain. The wire layer does not use wallet keys and does not add `chain_id`; network and genesis identity remain the established chain boundary.

## Compatibility constraint

This ADR does not modify the frozen Phase 4 consensus specification or its nine deterministic vector groups. The existing Python HTTP networking remains reference/prototype behavior and is not silently declared wire-compatible with the new production protocol.

## Future changes

Any incompatible change requires an explicit protocol-version transition and new compatibility vectors. The Phase 6 node MUST implement this frozen grammar rather than inventing an alternative wire format.
