# Phase 6 — Go Production Node Architecture

Phase 6 adds the first executable Go node above the frozen Phase 4/5.1/5.2 layers.

## Boundaries

- `internal/consensus`: existing frozen consensus algorithms.
- `internal/block`, `internal/transaction`, `internal/state`: existing protocol/core types.
- `internal/p2p`: frozen v1.0 wire codec and state primitives; **not modified by Phase 6**.
- `internal/p2pnode`: TCP transport, handshake integration, request/response dispatch, synchronization and propagation.
- `internal/chain`: persistent block graph, active-chain selection by accumulated work, state replay and reorganization.
- `internal/storage`: append-only binary journal with CRC32 records and atomic tip records. No JSON database is used.
- `internal/identity`: persistent secp256k1 node identity and frozen raw public-key representation.
- `internal/mempool`: bounded concurrent transaction pool using the existing validation and balance rules.
- `internal/node`: lifecycle, configuration, local API and mining orchestration.
- `cmd/syjd`: executable CLI.

## Acceptance pipeline

`decode -> structural validation -> consensus validation -> state transition -> chain selection -> durable persistence -> propagation`

A stored branch is retained. Only a strictly higher-work valid branch becomes the active tip.

## Storage recovery

The journal stores blocks and tip records with a fixed record header and CRC32. Startup replays the journal and rejects malformed/truncated/checksum-invalid records. A crash after a block record but before its tip record leaves an unreferenced block, not a divergent canonical tip.

## Current security boundary

The binary P2P grammar is unchanged. TLS, peer reputation/ban scoring, NAT traversal, and public-facing API authentication are not introduced as implicit protocol changes; they remain explicit future hardening gates.
