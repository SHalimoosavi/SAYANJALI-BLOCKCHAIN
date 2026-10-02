# Build 2A — C-2 Evidence

## Evidence rule

This file records executed commands/results only. Proposed architecture is not treated as runtime evidence.

## Baseline artifact

| Check | Status | Actual result |
|---|---|---|
| Frozen ZIP SHA-256 | PASS | `9701ce955acb188f7429a5f14e87c350ae29e251f8b386e0e84f51c3b2127aa1` |
| ZIP integrity | PASS | `No errors detected in compressed data` |
| ZIP entries | PASS | `407` |
| Remote baseline commit fetch | PASS | GitHub connector resolved `9502979638e48a43519360a3215eca2cb84e6ce9` |
| Remote Build 2A branch creation | NOT EXECUTED / BLOCKED | GitHub connector returned HTTP 403 `Resource not accessible by integration` |

The uploaded ZIP is a Git archive and therefore does not contain `.git` metadata. The tag object/ref itself was not independently resolved from inside the uploaded artifact.

## Phase 1 — source inspection

| Item | Status |
|---|---|
| I1 state persistence | CONFIRMED |
| I2 4 MiB limit | CONFIRMED |
| I3 state changes per block | CONFIRMED |
| I4 reusable incremental state code | CONFIRMED |
| I5 startup/restart/replay/reorg | CONFIRMED |
| I6 current state root | CONFIRMED |
| I7 reusable V3 code | CONFIRMED |
| I8 delta journal interface targets | CONFIRMED as design targets |
| I9 crash consistency | CONFIRMED current guarantees/gaps |
| I10 test infrastructure | CONFIRMED inventory |

Audit observations 1-8 were independently re-located in `docs/protocol/c2/INSPECTION.md`.

## Source-derived sizing

Current valid address format is 43 bytes (`SYJ` + 40 hex characters).

Current checkpoint account encoding is 67 bytes/account plus 106 fixed bytes.

Calculated uniform-address checkpoint sizes:

- 62,600 accounts: 4,194,306 bytes before the 64-byte journal block-hash prefix; exceeds the current 4,194,240-byte checkpoint payload limit by 66 bytes.
- 200,000 accounts: 13,400,106 bytes before the journal block-hash prefix.

Status: **NOT MEASURED** as runtime storage tests. These are source-derived arithmetic checks only.

## Phase 2 — design

`docs/protocol/c2/DESIGN.md` created.

D1-D7 all explicitly remain:

`OWNER DECISION REQUIRED`

No production design option was activated.

## Phase 3 — vector generation

### Generator

Command executed:

```text
python3 scripts/c2/generate_state_smt_v3_vectors.py
```

Python version:

```text
Python 3.13.5
```

Python dependencies: standard library only.

### First generation

PASS — generator completed and produced 10 vectors / 1,000-account V8.

### Second generation

PASS — generator completed again.

### Reproducibility

PASS.

Both generated vector files had SHA-256:

`3941cc06c641557f896a6f6235e9a94abe91ed66eb99949e0069860e2d9d7fc2`

### Vector file

PASS — `protocol/test-vectors/state-smt-v3.json` exists and contains V1-V10 plus 257 empty-subtree hashes.

### Go verifier

Command executed:

```text
GO111MODULE=off go test ./scripts/c2 -run TestFrozenStateSMTV3Vectors -count=1 -v
```

Actual result:

```text
=== RUN   TestFrozenStateSMTV3Vectors
--- PASS: TestFrozenStateSMTV3Vectors (0.17s)
PASS
ok   _/mnt/data/syj-build2a/work/scripts/c2  0.170s
```

Toolchain used for this local verifier:

`go1.23.2 linux/amd64`

This is **ADVISORY**, not the required Ubuntu Go 1.27.1 authority.

## Phase 4 — Build 2B plan

`docs/protocol/c2/BUILD2B_PLAN.md` created.

Required planning coverage:

- 70,000-account Accept -> checkpoint -> restart -> Open: SPECIFIED
- 200,000-account Ubuntu Go 1.27.1 CI test: SPECIFIED
- every-byte-offset corruption injection: SPECIFIED
- deep reorg vs clean replay: SPECIFIED
- V1/V2 byte-for-byte compatibility: SPECIFIED
- performance metrics: SPECIFIED

No Build 2B implementation executed.

## Repository validation

### `go.mod` / `go.sum`

`cmp` verification against the frozen uploaded baseline: PASS for both files; no changes were made.

Inspection shows the frozen baseline declares Go 1.27.1 and one existing secp256k1 dependency. No changes were made to these files in the Build 2A working copy.

### Go toolchain

`GOTOOLCHAIN=local go version` returned `go version go1.23.2 linux/amd64`. An attempt to use the repository-required Go 1.27.1 toolchain could not download the toolchain because external network resolution is unavailable.

Authoritative Go 1.27.1 Ubuntu CI: **NOT EXECUTED for Build 2A**.

### gofmt

`gofmt -w scripts/c2/state_smt_v3_vectors_test.go` — PASS for the new Go verifier source.

Repository-wide `gofmt -l .` under the required Go 1.27.1 toolchain: **NOT EXECUTED**.

### vet

`go vet ./...` with authoritative Go 1.27.1: **NOT EXECUTED**.

### full test suite

`go test ./...` with authoritative Go 1.27.1: **NOT EXECUTED**.

### race

`go test -race ./...` with authoritative Go 1.27.1: **NOT EXECUTED**.

### vector-specific test

PASS locally as recorded above; authoritative CI version: NOT EXECUTED.

## Production-code boundary

Working-copy changes are restricted to documentation, vector data, generator, verifier test, and build evidence artifacts.

No production consensus/storage/state/mempool/P2P/PoW/difficulty/tokenomics/protocol activation change was made.

## Finding status — authoritative 21-finding list

| # | Finding | Status |
|---:|---|---|
| 1 | C-1 | VERIFIED |
| 2 | C-2 | IN PROGRESS |
| 3 | H-1 | OPEN |
| 4 | H-2 | OPEN |
| 5 | H-3 | OPEN |
| 6 | H-4 | OPEN |
| 7 | H-5 | OPEN |
| 8 | H-6 | OPEN |
| 9 | H-7 | OPEN |
| 10 | H-8 | OPEN |
| 11 | M-1 | OPEN |
| 12 | M-2 | OPEN |
| 13 | M-3 | OPEN |
| 14 | M-4 | OPEN |
| 15 | M-5 | OPEN |
| 16 | M-6 | OPEN |
| 17 | M-7 | OPEN |
| 18 | M-8 | OPEN |
| 19 | L-1 | OPEN |
| 20 | L-2 | OPEN |
| 21 | L-3 | OPEN |

The identifiers are the complete 21-item list established in the frozen baseline's deferred-findings material (`docs/protocol/V3_DECISIONS.md:108-110`).

## Build 2A status

Because the required remote branch creation and authoritative Go 1.27.1 Build 2A CI could not be executed from this environment, this package is **INCOMPLETE/BLOCKED**, not represented as CI-verified.

The design/vector work itself is locally reproducible, and the SMT vectors are frozen by checksum pending owner review.

## Build 2A artifact validation

Artifact command executed after packaging:

```text
unzip -t /mnt/data/syj-build-02A-C2-design-20261002.zip
```

Actual result:

```text
No errors detected in compressed data of /mnt/data/syj-build-02A-C2-design-20261002.zip.
```

Artifact entry count: `20` ZIP entries including directories.

Artifact contents were scanned for `.git`, `.env`, private-key/certificate extensions, database files, logs, and generated binary extensions; no forbidden entries were present.

## Git patch-scope limitation

The uploaded frozen baseline is a Git archive without `.git` metadata. Therefore the exact commands:

```text
git diff --stat v0.9.6-build1.1-c1...HEAD
git diff --name-status v0.9.6-build1.1-c1...HEAD
```

are **NOT EXECUTED** in this environment.

A byte-for-byte comparison of every original file against the Build 2A working copy was executed, excluding only the intentionally replaced `build/CHANGES.md` and `build/EVIDENCE.md`; it passed. Explicit production-sensitive files were also byte-identical.
