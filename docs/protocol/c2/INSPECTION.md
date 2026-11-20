# SAYANJALI BLOCKCHAIN — Build 2A C-2 Baseline Inspection

**Baseline source:** `v0.9.6-build1.1-c1` frozen artifact, merge commit `9502979638e48a43519360a3215eca2cb84e6ce9`

**Scope:** source inspection only. No production implementation changes are made by Build 2A.

## Audit observations re-located against the frozen source

| Observation | Status | Current-source conclusion |
|---|---|---|
| 1. Whole state is written as one journal record every 16 blocks | **NOT CONFIRMED** | A full state checkpoint is written every 16 blocks, but the journal also contains separate block and tip records. |
| 2. Storage has a 4 MiB `maxRecord` | **CONFIRMED** | `internal/storage/storage.go:18-25` defines `maxRecord = 4 * 1024 * 1024`. |
| 3. `SaveStateCheckpoint` rejects oversized payloads | **CONFIRMED** | `internal/storage/storage.go:240-245` rejects payloads above `maxRecord-64`. |
| 4. Checkpoint persistence failure affects `Accept` and `Open` | **CONFIRMED** | `Accept` returns a storage error before advancing the durable tip; startup incremental-state initialization also persists checkpoints and propagates errors. |
| 5. State decoding permits up to 1,000,000 accounts | **CONFIRMED** | `internal/statecommitment/state.go:166-173` enforces `maxCheckpointAccounts = 1_000_000`. |
| 6. Historical 67-byte/account sizing and ~62,600 ceiling | **CONFIRMED — source-derived arithmetic; NOT A RUNTIME MEASUREMENT** | Current valid addresses are 43 bytes. The checkpoint encoding is 67 bytes/account plus 106 fixed bytes; the 4 MiB record payload permits 62,599 such accounts before fixed overhead. |
| 7. Current state root re-sorts and re-hashes all accounts every block | **CONFIRMED** | `Snapshot.Root()` collects all balance/nonce keys, sorts them, and hashes every entry; `Accept` recomputes the root after each best-chain state transition. |
| 8. Every block retains a full state snapshot in RAM | **CONFIRMED** | `stateSnapshots` stores `Clone()` results for reconstructed/accepted blocks. |

## I1 — State persistence

### Storage
- **File:** `internal/storage/storage.go`
- **Function/type:** `Store`, `Open`, `replay`, `appendRecord`, `SaveBlock`, `SetTip`, `SaveStateCheckpoint`
- **Lines:** 27-34, 36-51, 67-141, 156-173, 174-199, 240-266.
- **Quote:** `file, ... os.OpenFile(... "ledger.journal" ...)` and `s.appendRecord(...)`.
- **Finding:** The persistent store is one append-only journal file. Record types are block, tip, and state-checkpoint records. Each complete append calls `file.Sync()`.

### Checkpoint serialization
- **File:** `internal/statecommitment/state.go`
- **Function:** `Encode`
- **Lines:** 91-128.
- **Quote:** `writeU64(&b, uint64(len(keys)))` followed by each address, balance and nonce, then `b.WriteString(s.Root())`.
- **Finding:** A checkpoint is a complete `Snapshot` serialization, not a delta.

### When persistence occurs
- **File:** `internal/chain/chain.go`
- **Function:** `initializeIncrementalState`
- **Lines:** 850-890.
- **Quote:** `if b.Index%stateCheckpointInterval == 0 { ... SaveStateCheckpoint(...) }`.
- **Finding:** The checkpoint interval is 16 blocks (`line 593`).

### Accept ordering/error propagation
- **File:** `internal/chain/chain.go`
- **Function:** `Accept`
- **Lines:** 968-1011.
- **Quote:** `SaveBlock(b)` occurs before the winning-state checkpoint; `SaveStateCheckpoint` failure returns `"storage"`; `SetTip` occurs only after required persistence succeeds.
- **Finding:** Block, checkpoint, and tip persistence are separate synchronous journal appends. There is no single atomic multi-record commit marker in the current store.

**Status: CONFIRMED.**

## I2 — 4 MiB limit

- **File:** `internal/storage/storage.go`
- **Constant:** `maxRecord`
- **Lines:** 18-25.
- **Quote:** `maxRecord = 4 * 1024 * 1024`.

### Enforcement points
- **Replay:** lines 100-105 reject an on-disk record whose declared length exceeds `maxRecord`.
- **Generic append:** lines 156-159 reject payloads over `maxRecord`.
- **Checkpoint:** lines 240-245 reject checkpoint payloads over `maxRecord-64` because 64 bytes are prepended for the block hash.

### Callers
- `SaveStateCheckpoint` is called by `initializeIncrementalState` at lines 882-886 and by `Accept` at lines 1000-1005.
- `Accept` returns `"storage"` on checkpoint persistence failure at line 1003.
- `openWithProtocolAndClock` calls `initializeIncrementalState` at lines 156-170, so startup can fail if that persistence step fails.

### Replay behavior
- `Store.replay` treats an incomplete final header/payload as a recoverable tail and truncates it; complete-record checksum/magic/version/type failures remain fatal.

**Status: CONFIRMED.**

## I3 — State changes per block

### V2 incremental mutation
- **File:** `internal/chain/chain.go`
- **Function:** `applyIncrementalV2`
- **Lines:** 687-733.
- **Quote:** `sDebit(s, tx.Sender, tx.AmountBaseUnits)`, `sCredit(s, tx.Receiver, tx.AmountBaseUnits)`, `s.Nonces[tx.Sender] = expected + 1`.
- **Finding:** A normal V2 transaction debits the sender, credits the receiver, and increments the sender nonce. Coinbase credits the receiver and increases issuance/supply without changing a nonce.

### Account creation/deletion semantics in current code
- **File:** `internal/chain/chain.go`
- **Functions:** `sCredit`, `sDebit`
- **Lines:** 619-633.
- **Finding:** `sCredit` assigns the balance map entry and therefore creates a balance entry when absent. `sDebit` reduces the value but does not delete a zero-balance entry. There is no account-deletion operation in this mutation path.

### State cloning
- **File:** `internal/chain/chain.go`
- **Function:** `validateIncrementalBlock`
- **Lines:** 800-847.
- **Quote:** `candidate := parent.Clone()`.
- **Finding:** Each candidate block transition clones the complete snapshot before mutation.

### Legacy V2 replay
- **File:** `internal/chain/v2.go`
- **Function:** `replayV2State`
- **Lines:** 297-317.
- **Finding:** Restart replay reconstructs balances, nonces, issuance and confirmed transactions from the active block sequence.

**Status: CONFIRMED.**

## I4 — Reusable incremental state code

| Component | Location | Reuse assessment |
|---|---|---|
| `statecommitment.Snapshot` | `internal/statecommitment/state.go:19-49` | **Potential reuse:** economic field model and clone semantics. **Limitation:** full-map snapshot is exactly the scaling problem C-2 is intended to remove. |
| `applyIncrementalV2` | `internal/chain/chain.go:687-733` | **Potential reuse:** transaction-to-state mutation semantics. **Limitation:** writes directly into full maps; future delta layer needs an adapter. |
| `validateIncrementalBlock` | `internal/chain/chain.go:800-847` | **Potential reuse:** validation ordering and economic transition contract. **Limitation:** currently clones full state and returns a full snapshot. |
| `stateForParent` | `internal/chain/chain.go:893-925` | **Design reference:** nearest-checkpoint reconstruction is a useful recovery pattern. **Limitation:** still materializes full snapshots and has no delta journal. |
| `Store.SaveStateCheckpoint/GetStateCheckpoint` | `internal/storage/storage.go:240-266` | **Reference only:** checkpoint API shape is relevant. **Not recommended unchanged:** current single-record full-state limit is the C-2 finding. |

**Recommendation:** reuse economic transition semantics and test fixtures, not the current full-snapshot persistence/root implementation.

## I5 — Startup / restart / replay / reorg

### Startup path
- **File:** `internal/chain/chain.go`
- **Function:** `openWithProtocolAndClock`
- **Lines:** 105-171.
- **Path:** `Store.Open -> AllBlocks -> buildChain(tip) -> validateChain -> replayState -> initializeIncrementalState`.
- **Quote:** `if err := c.replayState(candidate); err != nil { ... }` followed by `initializeIncrementalState(candidate)`.
- **Finding:** Startup performs legacy full state replay and then separately reconstructs incremental state from the nearest persisted checkpoint when one exists.

### Incremental restart reconstruction
- **Function:** `initializeIncrementalState`
- **Lines:** 850-890.
- **Finding:** It scans backward for a checkpoint, decodes it, then validates subsequent blocks from that checkpoint to the active tip.

### Reorg
- **File:** `internal/chain/chain.go`
- **Function:** `Accept`
- **Lines:** 976-1074.
- **Finding:** A higher-work candidate causes the winning state to be rebuilt through `stateForParent` and `validateIncrementalBlock`; the active tip and in-memory state are published only after persistence succeeds. No separate undo-log mechanism is present.

### Explicit reorg tests
- **File:** `internal/chain/chain_test.go`
- **Tests:** `TestHigherWorkForkReorganizes`, `TestConfirmedTransactionIndexRebuildsAcrossReorg`.
- **File:** `internal/chain/phase9_5_state_test.go`
- **Test:** `TestPhase9_5V2ReorgIncrementalStateMatchesLegacyReplay`.

**Status: CONFIRMED.** Reorg exists and reconstructs the winning state; there is no persistent delta/undo system in the frozen source.

## I6 — Current state root

- **File:** `internal/statecommitment/state.go`
- **Function:** `Snapshot.Root`
- **Lines:** 52-89.
- **Quote:** `sort.Strings(keys)` followed by per-account hashing.
- **Algorithm:**
  1. SHA-256 hash starts with `SYJ-STATE-ROOT-V1\x00` and the state commitment version.
  2. Genesis supply, mining issued, and total supply are encoded as big-endian uint64 values.
  3. The union of balance and nonce addresses is built.
  4. Addresses are sorted lexicographically as strings.
  5. Each address uses `SYJ-STATE-ACCOUNT-V1\x00`, address bytes, balance uint64 and nonce uint64.
  6. The account count is included.
  7. The final SHA-256 digest is rendered as lowercase hex.

The valid address format is `SYJ` plus 40 hexadecimal characters (`pkg/protocol/constants.go:13-14`; `internal/wallet/address.go:13-18`).

`Accept` recomputes the current root after each winning state transition at `internal/chain/chain.go:1041-1045`.

**Status: CONFIRMED.** The current root is a full-account-set traversal and is not an authenticated sparse Merkle root.

## I7 — Reusable V3 code

- **File:** `internal/chain/chain.go:52-70` — `OpenV3WithGenesisState` selects protocol version 3 but calls the existing `openWithProtocol` machinery.
- **File:** `internal/block/merkle_v3.go:9-85` — `MerkleRootV3` is an explicit V3 transaction-Merkle construction. It is **not** a state SMT.
- **File:** `pkg/protocol/v3params.go:5-13` — current V3 parameters are centralized here.
- **File:** `internal/clock/clock.go` — injected clock abstraction is available for deterministic tests; unrelated to C-2 state design.

`docs/protocol/V3_DECISIONS.md:82-88` records the approved future direction as per-block deltas, chunked checkpoints, and authenticated SMT, while explicitly saying it was not implemented in Build 1.

**Status: CONFIRMED for reuse candidates; no V3 state SMT is active in the baseline.**

## I8 — Delta journal interfaces

No interfaces are modified in Build 2A. The current source indicates future responsibilities would need seams around:

1. state transition output (`validateIncrementalBlock` / `applyIncrementalV2`),
2. delta encoding and journal record typing (`internal/storage/storage.go:156-173`),
3. checkpoint chunk persistence (`SaveStateCheckpoint` currently full-state),
4. state reconstruction (`initializeIncrementalState`, `stateForParent`),
5. root calculation (`Snapshot.Root`),
6. reorg state selection (`stateForParent` / `Accept`).

These are design targets only.

**Status: CONFIRMED as design-analysis targets; interfaces remain unchanged.**

## I9 — Crash consistency

### Proven current behavior
- Every completed `appendRecord` calls `file.Sync()` (`internal/storage/storage.go:156-173`).
- `Store.Close` also calls `file.Sync()` before close (`storage.go:53-65`).
- Incomplete final headers/payloads are truncated during replay (`storage.go:79-113`, `143-153`).
- Complete-record checksum corruption is fatal (`storage.go:114-116`).

### Not present in source
- No directory fsync is present.
- No atomic rename/manifest/commit-marker protocol is present.
- Block, checkpoint, and tip records are separately appended.

**Status: CONFIRMED current guarantees and gaps.** This inspection does not claim stronger crash safety than the source demonstrates.

## I10 — Test infrastructure

### Persistence/storage
`internal/storage/storage_test.go`:
- `TestJournalPersistsAndRecovers`
- `TestJournalCorruptionDetected`
- `TestJournalRecoversTruncatedFinalHeader`
- `TestJournalRecoversTruncatedFinalPayload`
- `TestJournalCRCcorruptionRemainsFatal`
- `TestJournalMultipleRecordsRecoverDeterministically`

### State commitment
`internal/statecommitment/state_test.go`:
- `TestRootDeterministicIndependentOfMapOrder`
- `TestCheckpointRoundTripAndCorruptionDetection`
- `TestRootChangesForConsensusState`

`internal/statecommitment/state_decode_hardening_test.go`:
- truncated checkpoint rejection
- duplicate-account rejection

### Restart/reorg/state
`internal/chain/phase9_5_state_test.go`:
- incremental-vs-legacy replay
- checkpoint restart
- deterministic snapshot root
- V2 reorg incremental-vs-legacy replay

`internal/chain/chain_test.go`:
- higher-work reorg
- transaction-index rebuild across reorg
- restart transaction-index survival

### Missing from frozen source
No test was located that injects corruption at every byte offset of every checkpoint record type, no 70k/200k state persistence test exists, and no crash-injection test was located that exercises a future chunked checkpoint/manifest protocol.

**Status: CONFIRMED test inventory; missing future C-2 coverage is design work, not an implementation claim.**

## Source-derived 4 MiB arithmetic

The current address format is exactly 43 bytes (`SYJ` + 40 hex characters). The current checkpoint account entry is:

- 8 bytes address length
- 43 bytes address
- 8 bytes balance
- 8 bytes nonce
- **67 bytes/account**

The fixed checkpoint encoding is 106 bytes before account entries (magic, version, three uint64 supply fields, account count, and 64-byte root string). `SaveStateCheckpoint` permits at most `4 MiB - 64 = 4,194,240` bytes of checkpoint payload because 64 bytes are reserved for the block hash in the journal record.

Therefore the source-derived maximum for uniform 43-byte valid addresses is:

`floor((4,194,240 - 106) / 67) = 62,599 accounts`

This is an analytical bound from current encoding, **not a runtime measurement**. 62,600 accounts exceeds the payload ceiling by 66 bytes before any other variability.

At 200,000 accounts, the same uniform-address checkpoint would require approximately 13,400,106 bytes before the journal's 64-byte block-hash prefix and is therefore far beyond the current single-record limit.

## Inspection conclusion

The frozen source confirms the core C-2 scaling shape: full-state checkpoint records, a 4 MiB record ceiling, full-set root traversal, and full per-block snapshot retention. It also confirms useful incremental transition/reconstruction code and existing persistence/reorg tests that can inform Build 2B without activating C-2 in Build 2A.
