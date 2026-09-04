# SYJ Blockchain — Phase 5 Go Production Core Foundation

This directory adds the first production-oriented Go implementation while
leaving the Python reference implementation and frozen protocol vectors
unchanged.

## Compatibility boundary

The implementation is driven by the existing `protocol/test-vectors/*.json`
fixtures. Protocol-critical values are not re-authored in Go tests.

Compatibility coverage includes:

- canonical deterministic JSON used by the frozen fixtures
- SHA-256 hashing
- raw 64-byte SECP256k1 public-key representation
- SYJ address derivation
- ECDSA/SHA-256 verification and raw `r || s` signatures
- transaction signing payload and transaction hash
- Merkle roots
- block-header hashing
- hexadecimal-leading-zero PoW validation
- exact integer chain work (`16 ** difficulty`)
- difficulty retargeting and its 9-interval window
- native SYJ base-unit conversion and supply bounds
- deterministic genesis construction

## Scope

Implemented now:

- protocol constants and types
- canonical serialization
- cryptographic/hash primitives
- SECP256k1 key derivation, signing and verification
- transaction primitives
- block/header and Merkle primitives
- PoW, difficulty, work and reward primitives
- wallet/address primitives
- monetary state primitives
- genesis construction
- Go compatibility tests

Intentionally not implemented in this phase:

- production P2P
- RPC server
- production storage engine
- node lifecycle
- public testnet infrastructure
- protocol redesign or new economics

## Validation

From the repository root:

```text
go test ./...
go vet ./...
```

The compatibility suite reads the frozen JSON vectors directly from
`protocol/test-vectors` and reports expected/actual values when a
protocol-critical result diverges.

## Cryptographic note

The Python reference currently uses non-deterministic ECDSA signing. Go does
not need to reproduce a Python signature byte-for-byte. The frozen transaction
fixture is used to verify the signature and reproduce the exact transaction
hash. New Go signatures are valid raw `r || s` ECDSA signatures.

## Timestamp note

The frozen Python difficulty implementation has undefined fractional
retarget-window behavior because its clamped span is passed directly into
`Fraction`. The Go implementation therefore rejects fractional retarget
spans explicitly instead of silently defining new consensus behavior.
