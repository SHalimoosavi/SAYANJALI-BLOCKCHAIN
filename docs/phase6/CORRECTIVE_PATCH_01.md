# Phase 6 Corrective Patch 01

## Trigger
Clean-room Termux validation exposed two integration defects:

1. Newly constructed Go transactions could reach block construction with an empty `tx_hash`. The frozen transaction codec correctly requires the persisted/wire transaction hash, so block propagation and storage round-trip failed.
2. Unit-test networks that bind an unspecified TCP listener produced an advertised address such as `[::]:<port>`. The frozen handshake validation correctly rejects unspecified advertised hosts, so two-node handshake tests failed even though explicitly configured private testnet nodes worked.

## Corrections

### Transaction hash materialization
`internal/block.Block.New` now copies the supplied transaction slice and computes a missing transaction hash using the existing `Transaction.ComputeHash()` implementation. Existing non-empty hashes are preserved unchanged.

This does **not** alter:
- the transaction hash algorithm,
- transaction serialization,
- Merkle calculation,
- block hashing,
- consensus validation.

It ensures newly constructed protocol objects are fully materialized before persistence or P2P encoding.

### Local advertised endpoint
`internal/p2pnode.Network.Start` now derives `127.0.0.1:<listener-port>` when no advertised address was supplied and the listener is bound to an unspecified address.

An explicitly configured advertised address remains authoritative. The frozen handshake grammar and validation are unchanged.

## Regression coverage
Added/strengthened tests for:
- block construction materializing missing transaction hashes;
- default advertised endpoint using a reachable loopback host;
- existing two-node handshake;
- existing block propagation.

## Validation requirement
The authoritative dependency-backed validation must be rerun from a clean extraction in Termux:

```text
go mod download
go test ./...
go vet ./...
go build ./...
python -m pytest -q tests/p2p/test_reference_codec.py
./scripts/phase6_frozen_verify.sh
```

Then the real three-node testnet must be restarted from fresh data directories and block propagation/restart recovery must be rechecked.
