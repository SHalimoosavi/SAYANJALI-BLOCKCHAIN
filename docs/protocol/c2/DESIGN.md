# SAYANJALI BLOCKCHAIN — Build 2A C-2 Design Options

**Decision status:** all recommendations below are **OWNER DECISION REQUIRED**. Build 2A does not encode any selected option into production code.

## Design boundary

Approved direction: **per-block deltas + chunked checkpoints + authenticated Sparse Merkle Tree**.

Build 2A freezes only an independent candidate SMT cryptographic/reference vector specification. It does not activate a state SMT, change state persistence, change fees, or change consensus behavior.

## D1 — Nonce reset

### Problem
Current V2 mutation increments a sender nonce and does not delete zero-balance entries. Future minimum-balance enforcement could create an account-lifecycle rule that removes state. Removing the nonce would make the lifecycle semantics part of replay protection.

### Option A — Never delete account records
- **Storage:** monotonically retains account/nonce records.
- **Replay protection:** strongest continuity because nonce never disappears.
- **Tree:** leaf remains present even at zero balance.
- **Pruning/reorg:** simple semantics, higher long-term state.
- **Migration:** requires defining legacy absent/zero semantics.

### Option B — Tombstone accounts
- **Storage:** preserves a compact nonce/lifecycle marker while allowing balance data to be removed.
- **Replay protection:** nonce survives closure.
- **Tree:** tombstone is a distinct present leaf state.
- **Pruning/reorg:** requires tombstone retention rules.
- **Migration:** needs explicit V3-only semantics; no V1/V2 migration.

### Option C — Expiry-bound transaction semantics
- **Storage:** may permit lifecycle pruning after a defined horizon.
- **Replay protection:** depends on signed transaction validity including the expiry rule.
- **Tree:** account can become absent after expiry.
- **Complexity:** highest because transaction semantics and time become coupled.
- **Migration:** requires new V3 transaction semantics.

### Recommendation
**OWNER DECISION REQUIRED.** A nonce-preserving lifecycle is the principal replay-protection requirement; A and B preserve that property directly, while C adds new transaction-time semantics.

## D2 — Account semantics in the tree

### Candidate semantic model 1 — Absent / present-zero / present-nonce
Keep absence distinct from a zero-balance account and preserve nonce independently.

### Candidate semantic model 2 — Minimum-balance live accounts
A leaf is live only if balance meets the V3 minimum; below-minimum state follows the D1 lifecycle decision.

### Candidate semantic model 3 — Canonical tombstone state
Every historically created address has a canonical leaf state even after balance reaches zero.

### Required distinctions
- **Absent:** no leaf for the key.
- **Present, zero balance:** leaf exists with balance 0.
- **Present, nonce:** leaf exists with nonce state even if balance is zero.
- **Minimum-balance account:** live account satisfying the owner-approved minimum.

Creation, receiving, spending, below-minimum transitions, deletion, and nonce persistence must be specified together. No final semantics are selected here.

### Recommendation
**OWNER DECISION REQUIRED.** The vector specification treats an account as a leaf with explicit balance and nonce; lifecycle semantics remain outside the frozen vectors until D1/D2 are decided.

## D3 — Tree shape

### Option A — Plain 256-level SMT with cached defaults
- **Updates:** O(256) path recomputation.
- **Proof:** 256 sibling hashes for the uncompressed proof form.
- **Memory/disk:** predictable node addressing and simple deterministic rules; more empty-path structure.
- **Implementation:** lowest conceptual complexity.
- **Reorg:** path nodes can be versioned or copy-on-write.

### Option B — Path-compressed SMT
- **Updates:** potentially fewer materialized nodes for sparse populations.
- **Proof:** variable-length paths plus compression metadata.
- **Complexity:** higher proof/encoding complexity and more edge cases.
- **Determinism:** requires canonical compression/splitting rules.
- **Reorg:** structural sharing is attractive but implementation is more involved.

### Option C — Other authenticated key-value tree
Possible only if a measured requirement establishes that SMT-specific properties are insufficient. No third architecture is selected or implemented in Build 2A.

### Hash-count analysis
For a plain 256-level update, the cryptographic path contains up to 256 internal node hashes plus one leaf hash. This is **ANALYTICAL, NOT MEASURED**. The frozen vector generator records actual intermediate hashes for selected cases but is not a performance benchmark.

### Recommendation
**OWNER DECISION REQUIRED.** Option A is the simplest independently reproducible reference shape and is used for the Build 2A vectors; this does **not** activate or permanently select the production tree shape.

## D4 — Fork / reorg

### Option A — Persistent copy-on-write nodes
- **Storage:** shares unchanged subtrees between branches.
- **Memory:** branch state can remain compact if nodes are immutable.
- **Rollback:** tip selection changes the root pointer/version.
- **Deep reorg:** efficient when structural sharing is effective.
- **Crash recovery:** requires durable node/version metadata.

### Option B — Per-block deltas + undo records
- **Storage:** one forward delta plus inverse/undo information per block.
- **Rollback:** direct application of undo records.
- **Deep reorg:** cost grows with reorg depth.
- **Checkpoint interaction:** checkpoint plus delta replay is straightforward.
- **Crash recovery:** journal ordering and commit markers become critical.

### Recommendation
**OWNER DECISION REQUIRED.** The current full-state clone must not be carried into Build 2B. The final choice should be made together with D5 journal atomicity.

## D5 — Journal layout

### Option A — One atomic record per block
`block + delta + resulting state root`

- **Pros:** one logical commit unit; simple replay order.
- **Cons:** large variable records; harder partial recovery if record size grows.
- **Checkpoint:** separate chunk records plus manifest/commit marker are still needed for bounded state snapshots.

### Option B — Separate block and state-mutation records
- **Pros:** independent block and state streams; smaller records.
- **Cons:** requires transaction/commit correlation and replay ordering.
- **Crash recovery:** requires explicit incomplete-transaction semantics.

### Proposed checkpoint model for either option
- fixed-size or bounded chunks;
- deterministic chunk sequence numbers;
- checkpoint manifest listing chunks and expected hashes;
- commit marker written only after all chunks are durable;
- manifest/commit state written last;
- startup ignores incomplete uncommitted checkpoints;
- corruption of committed chunks fails closed.

### Recommendation
**OWNER DECISION REQUIRED.** Do not implement atomicity until the owner selects the record relationship and commit-marker semantics.

## D6 — Protocol gating

### V3 proposal
V3 state root becomes the authenticated SMT root.

### V1/V2 compatibility requirement
V1 and V2 state-root behavior remains byte-for-byte unchanged. No migration is proposed. The testnet reset means Build 2B can activate V3 on a new state without a historical state conversion path.

### Current baseline fact
`internal/statecommitment.Snapshot.Root` is currently the legacy full-set root. `internal/block/merkle_v3.go` is a transaction Merkle construction, not a state SMT. No Build 2A change activates a V3 state root.

### Recommendation
**OWNER DECISION REQUIRED.** Protocol activation should be a later Build 2B implementation gate after vectors and all state/recovery decisions are accepted.

## D7 — Parameters

### Minimum account balance
Let `M` be the owner-selected minimum account balance in base units and `F` the ordinary transaction fee for a funding transaction. No H-2 fee value exists in the frozen source, so fee-dependent results are **UNKNOWN / PARAMETERIZED**.

| Population | Minimum-balance capital | One funding fee assumption | Combined minimum spam capital + fee |
|---:|---:|---:|---:|
| 62,600 | `62,600 × M` | `62,600 × F` | `62,600 × (M + F)` |
| 200,000 | `200,000 × M` | `200,000 × F` | `200,000 × (M + F)` |

This analysis assumes one funding transaction per newly created account. Actual attack cost depends on whether minimum balance remains locked, can be reclaimed, or is converted into a tombstone/closure state.

### Storage-growth reference
For current 43-byte addresses, the frozen checkpoint encoding is 67 bytes/account plus 106 fixed bytes. This yields approximately:
- 62,600 accounts: 4,194,306 checkpoint bytes before the 64-byte journal block-hash prefix; this exceeds the current 4,194,240-byte checkpoint payload limit by 66 bytes.
- 200,000 accounts: 13,400,106 checkpoint bytes before the journal block-hash prefix; far beyond the current limit.

These are analytical calculations, not runtime measurements.

### Checkpoint chunk size / interval
Candidate ranges should be evaluated against:
- bounded record size,
- restart replay time,
- peak memory,
- checkpoint write amplification,
- deep-reorg replay distance.

No final numeric chunk size or interval is selected in Build 2A.

### Recommendation
**OWNER DECISION REQUIRED.** Minimum balance should be selected only after defining D1/D2 lifecycle semantics and the H-2 fee parameter. Chunk size and interval should be selected from measured Build 2B persistence/recovery results, not assumptions.

## Required decision gate

**Owner decision required before Build 2B implementation.**

Build 2A intentionally leaves D1-D7 unresolved at the production-architecture level.
