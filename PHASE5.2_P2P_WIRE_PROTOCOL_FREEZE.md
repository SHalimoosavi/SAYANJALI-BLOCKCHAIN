# SAYANJALI BLOCKCHAIN — P2P Wire Protocol Formalization

This build resolves networking gap G-014 by freezing the production `sayanjali-p2p` v1.0 wire grammar in `protocol/p2p-wire-spec.md` and implementing its standalone Go codec under `internal/p2p`.

## Scope boundary

- Phase 4 consensus remains unchanged.
- All nine frozen Phase 4 vector files remain unchanged.
- The legacy Python HTTP network remains reference/prototype code.
- No Phase 6 node, storage, peer manager, or synchronization engine is included.

## Frozen artifacts

- `protocol/p2p-wire-spec.md`
- `docs/ADR/0011-production-p2p-wire-protocol.md`
- `internal/p2p/*.go`
- `tests/p2p/fixtures/*.json`
- `tests/p2p/reference_codec.py`
- `tests/p2p/*_test.go`

## Protocol identity

`sayanjali-p2p` / `1.0`

## Framing

20-byte fixed header, TCP stream transport, big-endian integers, 4 MiB maximum frame, bounded binary message payloads, connection-local uint64 request IDs.

## Consensus boundary

Blocks, headers, and transactions are bounded opaque consensus-encoded byte strings to the wire codec. The future node MUST decode and validate them using the frozen consensus implementation after network decoding.

## Remaining gaps

G-015 incremental synchronization is not implemented here; its wire locator semantics are now defined and can be consumed by Phase 6. Peer reputation, transport confidentiality/TLS, Sybil resistance, and operational CI remain outside this build and are not falsely marked solved.
