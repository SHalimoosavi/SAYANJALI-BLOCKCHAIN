# SAYANJALI BLOCKCHAIN — C-2 Owner Decision Register

**Audit target:** Build 2A / PR #16  
**HEAD:** `233ef88550ba3b50cd336bf48244c5c0c365985d`  
**Baseline:** `v0.9.6-build1.1-c1` / `9502979638e48a43519360a3215eca2cb84e6ce9`  
**Rule:** No selected decision may be inferred from implementation convenience.

A decision is **NOT DECIDED** unless an explicit owner decision is present in the repository or in the owner's instructions.

## D1 — Account / nonce lifecycle

**Question:** What happens when an account reaches zero or below minimum balance?

**Options:** A never-delete; B nonce/lifecycle tombstone; C expiry-bound semantics.

**Selected decision:** NOT DECIDED.

**Evidence:** DESIGN.md explicitly leaves D1 OWNER DECISION REQUIRED. Current V2 does not delete zero-balance balance entries (`internal/chain/chain.go:627-633`) and increments sender nonce after successful transfer (`687-725`), but this is historical behavior, not a V3 lifecycle decision.

**Consensus consequence:** account existence, nonce reuse, deletion, recreation and leaf semantics remain undefined.

**Security consequence:** replay protection and account-spam semantics remain undefined.

**Storage consequence:** retention/pruning/tombstone behavior remains undefined.

**Test consequence:** delete/recreate/nonce-after-delete tests have no unique expected result.

**Implementation consequence:** Build 2B must not implement lifecycle behavior until D1 is selected.

**STATUS: BLOCKING**

## D2 — Account existence / minimum balance

**Question:** How does minimum balance interact with account existence?

**Explicit owner direction:** minimum account balance + ordinary transaction fees, with no separate account-creation fee.

**Selected decision:** PARTIAL ONLY — economic direction selected; lifecycle/existence semantics NOT DECIDED.

**Evidence:** `docs/protocol/V3_DECISIONS.md` records the minimum-balance + ordinary-fee direction but says numeric values require spam-cost analysis. DESIGN.md leaves absent/present-zero/present-nonce/minimum-balance semantics unresolved.

**Consensus consequence:** creation, receiving, below-minimum spend, zero-balance and deletion behavior remain undefined.

**Security consequence:** spam cost cannot be proven without M/F and closure semantics.

**Storage consequence:** leaf/tombstone retention remains undefined.

**Test consequence:** D2 acceptance cases cannot be finalized.

**Implementation consequence:** no minimum-balance enforcement may be implemented until D1/D2/H-2 are frozen.

**STATUS: BLOCKING**

## D3 — SMT shape

**Question:** Plain 256-level SMT or path-compressed authenticated tree?

**Selected decision:** NOT DECIDED.

**Evidence:** Build 2A vectors use the plain 256-level reference construction, but DESIGN.md explicitly says this does not permanently select production tree shape.

**Consensus consequence:** root/proof/node addressing semantics cannot be finalized.

**Security consequence:** proof canonicality depends on the selected shape.

**Storage consequence:** node persistence differs.

**Test consequence:** current vectors remain reference-only until D3 is selected.

**Implementation consequence:** Build 2B must not choose based on convenience.

**STATUS: BLOCKING**

## D4 — Reorg strategy

**Question:** How is state for a winning fork reconstructed or rolled back?

**Selected decision:** NOT DECIDED.

**Options considered:** persistent copy-on-write nodes; forward deltas plus undo; other owner-approved architecture.

**Evidence:** current source reconstructs from checkpoint/common ancestor plus replay (`chain.go:893-925`), but this is the pre-C-2 scaling architecture.

**STATUS: BLOCKING**

## D5 — Journal atomicity / commit protocol

**Question:** What is the durable commit unit for block, delta, checkpoint and tip?

**Selected decision:** NOT DECIDED.

**Evidence:** current storage appends block, checkpoint and tip separately (`storage.go:174-199`, `240-256`), each synced; C-2 has no selected multi-record commit protocol.

**Consensus consequence:** crash/restart result is not uniquely specified.

**Security consequence:** incomplete/corrupt state interpretation is undefined.

**Storage consequence:** record ordering and recovery scanner are undefined.

**Test consequence:** exhaustive corruption matrix cannot be finalized.

**Implementation consequence:** no C-2 persistence implementation may begin.

**STATUS: BLOCKING**

## D6 — V3 activation / V1-V2 compatibility

**Question:** How and when does authenticated V3 state become authoritative?

**Explicit owner direction:** V1/V2 behavior remains unchanged; V3 Merkle activation is deferred until C-2 acceptance.

**Selected decision:** PARTIAL ONLY — compatibility direction selected; activation mechanism and state-root commitment NOT DECIDED.

**Evidence:** `internal/chain/chain.go:52-70` explicitly says V3 C-1 preserves V2 state architecture and does not enable V3 Merkle activation. `internal/block/header.go:3-9` has no state-root field.

**Consensus consequence:** no unique activation/commitment rule exists.

**Security consequence:** activation ambiguity can split nodes.

**Storage consequence:** migration/restart boundary is undefined.

**Test consequence:** activation tests cannot be frozen.

**Implementation consequence:** Build 2B must stop before activation wiring.

**STATUS: BLOCKING**

## D7 — Minimum balance / chunk / checkpoint parameters

**Question:** What numeric parameters are frozen?

**Selected decision:** NOT DECIDED.

**Known direction:** minimum balance and ordinary fee are owner-selected but numeric values are absent; checkpoint chunk size and interval are absent; replacement bounded resource limits are absent.

**Analytical population cases:**
- 62,600 accounts: 62,600 × (M + F) minimum capital plus one funding fee per account.
- 200,000 accounts: 200,000 × (M + F).

These are analytical formulas, not runtime measurements.

**STATUS: BLOCKING**

## Decision gate

D1 = NOT DECIDED — BLOCKING  
D2 = PARTIAL — BLOCKING  
D3 = NOT DECIDED — BLOCKING  
D4 = NOT DECIDED — BLOCKING  
D5 = NOT DECIDED — BLOCKING  
D6 = PARTIAL — BLOCKING  
D7 = NOT DECIDED — BLOCKING

**Owner decision gate: NOT SATISFIED.**
