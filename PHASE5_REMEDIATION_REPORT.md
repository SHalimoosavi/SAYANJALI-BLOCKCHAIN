# Phase 5.1 Remediation Report

## Changes

1. Canonical JSON float formatting now preserves positive and negative zero and normalizes scientific exponents to CPython spelling. ASCII escaping now covers the full non-printable/non-ASCII range expected from `ensure_ascii=True` behavior. Unsupported Go value types fail explicitly.
2. Added a deterministic CPython differential oracle with 244 serialization cases and a Go byte-for-byte comparison test.
3. Replaced the custom `math/big` secp256k1 arithmetic and signer with Decred's `github.com/decred/dcrd/dcrec/secp256k1/v4` v4.4.1 and its optimized ECDSA package.
4. Preserved raw 64-byte X||Y public keys and raw 64-byte R||S signatures at the SYJ boundary.
5. Added adversarial crypto tests for malformed/boundary keys, signatures, modified messages/keys, high-S compatibility, deterministic signing, and fuzzing.
6. Hardened monetary supply invariants against invalid pre-existing totals and uint64 underflow.

## Frozen protocol status

The Phase 4 protocol specification and all 9 deterministic JSON vector files are copied unchanged from the Phase 4 compatibility artifact. This remediation does not regenerate or alter those vectors.

## Dependency rationale

Selected: `github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.1`.

Rationale: tagged stable release, pure-Go implementation, specialized secp256k1 field/scalar arithmetic, dedicated ECDSA implementation, RFC6979 deterministic signing, canonical low-S signing, strict public-key parsing, and extensive upstream cryptographic tests. The upstream implementation should still be treated as a cryptographic dependency requiring normal dependency/security review; its API and algorithms are not a blanket guarantee that every operation is constant-time. The module is ISC licensed. Its module graph requires `github.com/decred/dcrd/crypto/blake256 v1.1.0`.

This is a stronger cryptographic foundation than the removed local curve arithmetic, but it is not a substitute for an independent security audit of the complete SYJ protocol implementation.

## Security policy note

The Python reference verifier accepts valid high-S ECDSA signatures. The remediation therefore does not silently change consensus semantics by rejecting high-S signatures. New Go signatures are low-S due to the selected library's signing behavior.
