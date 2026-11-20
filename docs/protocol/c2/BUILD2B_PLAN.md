# SAYANJALI BLOCKCHAIN — Build 2B Implementation Plan

**Status:** planning only. No Build 2B implementation is included in Build 2A.

## B2-1. Interfaces likely to change

| Current location | Future responsibility | Compatibility impact |
|---|---|---|
| `internal/chain/chain.go:636-733` | Produce canonical state mutations/deltas from validated transactions | Preserve V1/V2 transition semantics; V3 adapter only. |
| `internal/chain/chain.go:800-847` | Validate block and apply delta to authenticated state | V1/V2 path must remain unchanged. |
| `internal/storage/storage.go:156-173` | Add bounded delta/checkpoint record types and commit ordering | Existing record decoding must remain backward-readable where applicable; no V1/V2 migration. |
| `internal/storage/storage.go:240-266` | Replace one full-state checkpoint with chunk/manifest checkpoint APIs | V3-only activation; exact recovery semantics must be frozen first. |
| `internal/chain/chain.go:850-925` | Load checkpoint root/chunks and replay deltas to reconstruct state | Must support clean restart and deep reorg. |
| `internal/statecommitment/state.go:52-89` | Replace full-set root for V3 with authenticated SMT root | V1/V2 root remains byte-for-byte unchanged. |
| `internal/chain/chain.go:893-925` | Select branch state using checkpoint + deltas/structural sharing | Must eliminate full per-block state cloning. |
| `internal/chain/chain.go:976-1074` | Publish durable state/tip atomically after state persistence | Must align with selected D4/D5 design. |

No interface is modified in Build 2A.

## B2-2. Ordered implementation/test plan

1. Preserve and freeze Build 2A SMT vectors.
2. Record owner decisions for D1-D7.
3. Define V3 state model and account lifecycle semantics.
4. Implement the selected authenticated SMT in an isolated verification layer.
5. Cross-check production SMT against the frozen Python/Go vectors.
6. Implement canonical per-block state delta creation.
7. Implement bounded journal records for deltas.
8. Implement chunked checkpoints and manifest/commit-marker protocol.
9. Implement startup recovery and incomplete-checkpoint handling.
10. Implement branch/reorg state selection using the selected D4 mechanism.
11. Activate the V3 authenticated state root only after the above suites pass.
12. Preserve V1/V2 legacy state-root bytes exactly.
13. Add the owner-selected minimum-account-balance rule.
14. Integrate the owner-approved H-2 fee mechanism separately; do not conflate it with account creation.
15. Run large-state persistence/restart tests.
16. Run corruption, deep-reorg, compatibility, and performance evidence suites.
17. Run Ubuntu CI with Go 1.27.1 as the authoritative implementation gate.

## B2-3. Required acceptance tests

### 70,000-account test

Construct exactly 70,000 deterministic accounts using a documented fixture. Execute:

`Accept -> checkpoint -> process restart -> Open`

Verify identical:
- state root
- active tip

The test must record elapsed time and peak memory. Termux may use a configurable smaller case for local development.

### 200,000-account test

Execute the same lifecycle with exactly 200,000 deterministic accounts.

The authoritative CI requirement is:
- Ubuntu runner
- Go 1.27.1
- full test enabled
- no production-code bypasses

Required equality after restart:
- state root
- active tip

## B2-4. Corruption testing

For **every byte offset** of every new record type, inject corruption/truncation and assert the defined recovery result.

Coverage must include:
- partial record,
- corrupted length,
- corrupted checksum/hash,
- corrupted payload,
- truncated journal,
- incomplete checkpoint,
- incomplete manifest,
- missing commit marker,
- commit marker present with invalid payload.

For each record type, record the first failing offset and the expected error/recovery classification. A single arbitrary corruption offset is insufficient.

## B2-5. Deep reorg

Select and document an exact reorg depth before execution.

Construct a losing and winning branch from a common ancestor. After reorg:

`state_root_after_reorg == state_root_from_clean_replay_of_winning_chain`

and:

`tip_after_reorg == winning_chain_tip`

Also verify account balances/nonces and confirmed-transaction index semantics where applicable.

## B2-6. V1/V2 compatibility

Require proof that:
- V1 replay remains unchanged.
- V2 replay remains unchanged.
- legacy state-root bytes remain byte-for-byte compatible.
- no V1/V2 state migration is introduced.
- V3 activation is gated independently.

The existing V1/V2 vector/regression suites must remain green.

## B2-7. Performance evidence

Every large-state/recovery run must record, at minimum:
- elapsed time,
- peak memory,
- journal size,
- checkpoint size,
- number of SMT nodes,
- number of SHA-256 hashes,
- recovery time,
- replay distance from checkpoint.

Until a run is executed, each metric is **NOT MEASURED**.

## Build 2B exit gate

Build 2B cannot be considered complete until owner decisions D1-D7 are recorded, the frozen vector suite remains unchanged, large-state restart tests pass, corruption coverage is exhaustive by byte offset, deep reorg equals clean replay, V1/V2 compatibility is byte-for-byte verified, and Ubuntu Go 1.27.1 CI supplies authoritative evidence.
