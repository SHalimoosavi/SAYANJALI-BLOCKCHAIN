# SAYANJALI BLOCKCHAIN — C-2 Design Freeze Report

## 1. Executive status

**FINAL AUTHORIZATION: DESIGN NOT FROZEN — BUILD 2B BLOCKED**

Build 2A has a reproducible candidate SMT vector artifact and the current PR head is CI-clean. The protocol itself is not frozen because multiple consensus-critical decisions remain explicitly unresolved.

This report does not claim C-2 implementation, production readiness, or security-audit completion.

## 2. Exact Git baseline

| Field | Value |
|---|---|
| C-1 tag | `v0.9.6-build1.1-c1` |
| Annotated tag object | `c3b61d9a0a249633a98ea9c04e143e9eecd3006a` |
| Peeled baseline commit | `9502979638e48a43519360a3215eca2cb84e6ce9` |
| Build 2A branch | `build2a-c2-design` |
| Current remote HEAD | `233ef88550ba3b50cd336bf48244c5c0c365985d` |
| Original Build 2A commit | `c32d40f9fa2ef0cb46de22d25bb0fefcc19873af` |
| Current HEAD parent | `c32d40f9fa2ef0cb46de22d25bb0fefcc19873af` |
| Merge base with C-1 | `9502979638e48a43519360a3215eca2cb84e6ce9` |
| PR | #16 |
| PR state | OPEN / NOT MERGED |
| main | `9502979638e48a43519360a3215eca2cb84e6ce9` |

Remote comparison reports the Build 2A branch exactly two commits ahead of C-1 and not behind it.

Local Termux working-tree state was **NOT VERIFIED in this audit environment**.

## 3. CI status

Current PR-head workflows for `233ef885...`:

- Build 1 C-1 Verification — success, run `37067226240`.
- Production Validation — success, run `37067226109`.

Production Go job passed `go test ./...`, `go vet ./...`, `go build ./...`, `go test -race ./...`, `govulncheck ./...`, `staticcheck ./...`, and `gosec ./...`.

Production Python job passed under Python 3.12: dependency installation, pytest, compileall, pip-audit and bandit.

CI PASS is automated repository validation only. It is not protocol approval.

## 4. Files reviewed

### Build 2A
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

### Referenced legacy source
- `internal/storage/storage.go`
- `internal/statecommitment/state.go`
- `internal/chain/chain.go`
- `internal/chain/v2.go`
- `internal/transaction/transaction.go`
- `internal/transaction/v2.go`
- `internal/block/block.go`
- `internal/block/header.go`
- `internal/block/merkle_v3.go`
- `internal/wallet/address.go`
- `pkg/protocol/constants.go`
- `pkg/protocol/v3params.go`
- `docs/protocol/V3_DECISIONS.md`
- relevant storage/state/chain regression tests.

## 5. State model findings

FACTS:
1. `Snapshot` contains balances, nonces, GenesisSupply, MiningIssued and Supply (`state.go:19-25`).
2. Legacy `Snapshot.Root()` includes the three global supply fields and sorted balance/nonce account entries (`state.go:52-89`).
3. Checkpoints serialize full state, not deltas (`state.go:91-128`).
4. Storage is an append-only journal with block, tip and state-checkpoint records (`storage.go:67-138`).
5. One journal record is limited to 4 MiB; checkpoint payload is limited to 4 MiB minus 64 bytes (`storage.go:18-25, 240-256`).
6. V2 transfers debit sender, credit receiver and increment sender nonce (`chain.go:687-725`).
7. Zero-balance balance entries are not deleted by `sDebit` (`chain.go:627-633`).
8. Candidate validation clones the full snapshot before applying a block (`chain.go:800-847`).
9. Restart/reorg reconstruction uses checkpoints plus replay and retains state snapshots (`chain.go:850-925`).
10. V3 C-1 preserves the V2 state architecture and explicitly does not enable V3 Merkle activation (`chain.go:52-70`).
11. Current block header has no state-root field (`internal/block/header.go:3-9`).

### Analytical checkpoint sizing

Valid protocol addresses are 43 bytes: `SYJ` + 40 hex characters (`pkg/protocol/constants.go:13-14`; `internal/wallet/address.go:13-18`). Current checkpoint encoding is 67 bytes/account plus 106 fixed bytes.

- 62,600 accounts = 4,194,306 bytes before the journal block-hash prefix.
- Current checkpoint payload ceiling = 4,194,240 bytes.
- 62,600 therefore exceeds that payload ceiling by 66 bytes.
- 200,000 accounts = 13,400,106 bytes before the journal prefix.

These are analytical calculations, **NOT runtime measurements**.

## 6. State-transition findings

Current V2 facts include ordered block transaction application, sequential sender nonce checks, duplicate transaction identity rejection, and candidate-state mutation on a clone. C-2 must not silently reinterpret these rules.

The V3 state transition additionally needs frozen minimum-balance, fee, deletion/recreation and failure-atomicity semantics.

## 7. SMT findings

The candidate/reference vectors define SHA-256(address UTF-8) keys, 256-bit MSB-first traversal, domain-separated leaf/node hashes, empty-subtree recursion and 256-sibling reference proofs. The frozen vector SHA is:

`3941cc06c641557f896a6f6235e9a94abe91ed66eb99949e0069860e2d9d7fc2`

Critical limitations:
- production D3 tree shape remains unresolved;
- leaf version `0x01` is generator-defined but not explicitly frozen in prose;
- proof directions are not bound to query key by the Go verifier;
- `PROOF_TAG` is declared but unused in proof hashing;
- V10 uses a non-protocol-valid address;
- V4 tests first-byte prefix extremes, not literal 256-bit all-zero/all-one keys;
- Go verifier parses JSON numbers through `float64`.

## 8. Serialization findings

Candidate leaf serialization is deterministic, but the exact leaf-version byte must be explicitly frozen. No production binary proof/delta/chunk/manifest encoding is frozen. JSON must remain transport-only and no floating-point value may participate in consensus encoding.

## 9. State delta findings

Per-block deltas are architectural direction only. Exact mutation schema, ordering, duplicate handling, linkage, empty-delta form and fee effects are missing. Build 2B cannot implement them yet.

## 10. Checkpoint findings

Chunked checkpoints are direction only. Chunk size, interval, boundaries, IDs, exact bytes, hash domains, manifest format, commit marker and recovery matrix are missing.

The existing 4 MiB limit is a current implementation fact, not an intended C-2 consensus ceiling.

## 11. Journal/recovery findings

Current complete records are CRC-protected and synced; incomplete final headers/payloads are truncated; complete-record checksum corruption is fatal. Block/checkpoint/tip are separate records. C-2 has no selected multi-record atomic commit protocol, so crash behavior is not frozen.

## 12. Reorg findings

Current source reconstructs fork state from checkpoint/common ancestor plus replay. Existing tests compare current V2 incremental reorg state with legacy replay. D4 remains unresolved for C-2.

Required Build 2B invariant:

`recovered_state(winning_chain) == clean_replay(winning_chain)`

for state root, accounts, balances, nonces, global state and tip.

## 13. Economic findings

Owner direction: minimum account balance + ordinary fees, no separate creation fee.

Still absent: numeric minimum, fee amount/encoding, fee ordering, below-minimum behavior. Current V2 transaction has no fee field (`internal/transaction/transaction.go:16-27`).

Analytical formulas:
- 62,600 accounts: `62,600 × (M + F)`.
- 200,000 accounts: `200,000 × (M + F)`.

No runtime economic benchmark was performed.

## 14. Compatibility findings

V1/V2 preservation is a requirement. No evidence was found for a repository-level “testnet reset” protocol mechanism; therefore reset-based V3 activation is treated as an unverified planning assumption, not source fact.

No V1/V2 migration may be introduced for convenience.

## 15. Security dependencies

Authoritative 21-finding status:

C-1 VERIFIED  
C-2 BLOCKED / IN PROGRESS  
H-1..H-8 OPEN  
M-1..M-8 OPEN  
L-1..L-3 OPEN

Known dependencies:
- H-2 OPEN — economic direction exists, but numeric and fee encoding semantics remain unresolved.
- M-1 OPEN — V3 Merkle activation is not selected.
- M-3 OPEN — block/transaction resource limits are not frozen.
- M-4 OPEN — bounded validation memory/disk rules are not frozen.
- M-5 OPEN — C-2 binary serialization is not frozen.
- M-7 OPEN — C-2 crash consistency is not frozen.

Other H/M/L findings remain OPEN exactly as the deferred-findings register states.

## 16. D1-D7 status

D1: NOT DECIDED — BLOCKING  
D2: PARTIAL — BLOCKING  
D3: NOT DECIDED — BLOCKING  
D4: NOT DECIDED — BLOCKING  
D5: NOT DECIDED — BLOCKING  
D6: PARTIAL — BLOCKING  
D7: NOT DECIDED — BLOCKING

## 17. Protocol ambiguity status

The ambiguity register identifies independent blockers around state-root commitment, global-state coverage, account lifecycle, fees, production SMT shape, delta schema, checkpoint schema, journal atomicity, recovery, reorg strategy and activation.

## 18. Test-vector status

| Item | Status |
|---|---|
| Generator | PASS — independent Python standard-library generator |
| Reproducibility | PASS — regenerated twice with identical bytes |
| SHA-256 | PASS — `3941cc06c641557f896a6f6235e9a94abe91ed66eb99949e0069860e2d9d7fc2` |
| Local Go verifier | PASS — Go 1.23.2 linux/amd64, non-authoritative |
| Current PR CI | PASS — Go 1.27.1 workflow passed full Go validation |
| Independence | PASS at source level — generator does not import/call Go implementation |
| Full protocol coverage | NOT VERIFIED / INCOMPLETE |
| uint64 boundary coverage | NOT VERIFIED |
| malformed-proof rejection | NOT VERIFIED |
| protocol-valid non-inclusion | NOT VERIFIED |

An independent container rerun of the packaged generator under Python 3.13.5 reproduced the exact frozen vector SHA and byte-for-byte output. An independent local verifier under Go 1.23.2 passed. These do not substitute for protocol freeze.

## 19. Build 2B contract status

`BUILD2B_CONTRACT.md` is complete as a conditional contract. It intentionally does not select unresolved consensus rules.

## 20. Remaining blockers

At minimum: A-01, A-02, A-03, A-04, A-05, A-06, A-13, A-14, A-15, A-16, A-17, A-18 and A-20; plus D1-D7 as recorded in OWNER_DECISIONS.md.

## 21. Required owner decisions

1. Select D1.
2. Complete D2 including exact existence/minimum semantics and H-2 fee semantics.
3. Select D3.
4. Select D4.
5. Select D5.
6. Complete D6 including state-root commitment and activation mechanism.
7. Select D7 numeric/resource parameters.
8. Close all consensus-critical ambiguity IDs explicitly.

## 22. Final authorization status

**DESIGN NOT FROZEN — BUILD 2B BLOCKED**

No C-2 production implementation is authorized by this report.
