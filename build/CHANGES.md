# Build 2A — C-2 Design / SMT Vector Gate

## Baseline

- Tag: `v0.9.6-build1.1-c1`
- Commit: `9502979638e48a43519360a3215eca2cb84e6ce9`
- Intended branch: `build2a-c2-design`

The uploaded frozen baseline artifact was independently checksum-verified before inspection:

`9701ce955acb188f7429a5f14e87c350ae29e251f8b386e0e84f51c3b2127aa1`

## Purpose

Build 2A is a design, specification, and test-vector gate for C-2. It does not implement C-2 production behavior.

Approved direction documented:

- per-block deltas,
- chunked checkpoints,
- authenticated Sparse Merkle Tree,
- minimum account balance + ordinary transaction fees as the later account-growth direction.

## Added files

- `docs/protocol/c2/INSPECTION.md`
- `docs/protocol/c2/DESIGN.md`
- `docs/protocol/c2/TEST_VECTORS.md`
- `docs/protocol/c2/BUILD2B_PLAN.md`
- `protocol/test-vectors/state-smt-v3.json`
- `scripts/c2/generate_state_smt_v3_vectors.py`
- `scripts/c2/state_smt_v3_vectors_test.go`
- `build/CHANGES.md`
- `build/EVIDENCE.md`
- `build/SHA256SUMS`
- `build/changes.patch`

## Production-code boundary

No production consensus, state, storage, mempool, P2P, PoW, difficulty, tokenomics, protocol-version, fee, account-creation, or Merkle-state activation code is changed by the Build 2A package.

`go.mod` and `go.sum` are intentionally unchanged.

## Vector freeze

The independent Python generator produced the frozen vector file twice with identical bytes.

Vector SHA-256:

`3941cc06c641557f896a6f6235e9a94abe91ed66eb99949e0069860e2d9d7fc2`

The Go verifier passed locally under the available Go toolchain using `GO111MODULE=off` so the verification package itself could be executed without activating the repository's Go 1.27 toolchain download.

This local result is advisory only; Ubuntu Go 1.27.1 remains the authoritative CI toolchain.

## Scope status

- C-1: VERIFIED (owner-supplied authoritative baseline status)
- C-2: IN PROGRESS
- H-1 through H-8: OPEN
- M-1 through M-8: OPEN
- L-1 through L-3: OPEN

Build 2A leaves D1-D7 as OWNER DECISION REQUIRED.

Build 2B is blocked pending those owner decisions.
