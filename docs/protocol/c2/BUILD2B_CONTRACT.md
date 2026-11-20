# SAYANJALI BLOCKCHAIN — Build 2B Implementation Contract

**Contract status:** NOT AUTHORIZED  
**Reason:** consensus-critical Build 2A decisions remain unresolved.

This is a conditional implementation contract. It is not an implementation and deliberately contains no unselected consensus constants.

## 1. Authorization prerequisites

Build 2B may begin only when D1-D7 are explicitly decided, H-2 fee semantics are frozen, and every consensus-critical ambiguity in `PROTOCOL_AMBIGUITIES.md` is closed.

The frozen vector checksum must remain:

`3941cc06c641557f896a6f6235e9a94abe91ed66eb99949e0069860e2d9d7fc2`

unless an explicit owner-approved vector amendment is recorded.

## 2. State transition contract

The implementation must realize exactly one deterministic transition:

`State[n-1] + Block[n] -> State[n]`

Before coding, the final protocol must define transaction order, account creation, receiving into absent accounts, balance changes, nonce initialization/increment, minimum-balance checks, fee deduction/order, zero-value behavior, invalid transaction behavior, duplicate transaction behavior, duplicate account mutations, deletion/recreation, overflow/underflow and whole-block failure atomicity.

A failed block must not partially publish state. Storage representation must not alter transition semantics.

## 3. Authenticated state contract

The selected tree must freeze key derivation, leaf bytes/version, domain separators, empty recursion, bit ordering, level numbering, left/right ordering, duplicate-key collision handling, insert/update/delete, empty-tree root, state-root commitment location and coverage of all consensus state including global monetary fields if applicable.

The production root must be reproducible independently of storage layout.

## 4. Proof contract

If proofs are protocol-facing, Build 2B must implement exact inclusion/non-inclusion semantics, key binding, sibling ordering, path length, canonical encoding and rejection of malformed, truncated, excess, wrong-key, wrong-leaf and wrong-root proofs.

If proof serialization is non-consensus, that classification must be explicit rather than inferred.

## 5. Delta contract

The frozen delta schema must include parent state commitment, block linkage, deterministic ordered mutations, created/updated/deleted account effects, nonce effects, balance effects, fee effects, resulting state commitment, empty-delta representation and duplicate/conflicting mutation rules.

Deltas are derived from consensus state transitions, not arbitrary storage mutations.

## 6. Checkpoint contract

The selected checkpoint protocol must define trigger, interval, chunk size/bound, sequence numbering, chunk content/hash, manifest bytes/hash, target block/height, parent/result commitments, completeness/commit marker, publication order and recovery handling for stale/incomplete/corrupt checkpoints.

A valid state must not depend on a fixed 4 MiB single-record limit.

## 7. Journal/crash contract

For every new record type, recovery must be explicitly frozen for complete records, incomplete headers/payloads, invalid lengths, invalid checksum/hash, duplicate/out-of-order/missing/stale records, incomplete checkpoint/manifest, missing commit marker and committed-but-invalid payloads.

Each class must resolve to exactly one of ACCEPT / IGNORE / ROLLBACK / REBUILD / HALT.

## 8. Reorg contract

After a winning-chain reorganization:

`recovered_state(winning_chain) == clean_replay(winning_chain)`

must hold for state root, account contents, balances, nonces, global state, active tip and height.

## 9. Compatibility contract

V1/V2 behavior must remain byte-for-byte compatible wherever the owner-approved compatibility decision requires it. No historical migration may be introduced merely for C-2 convenience. V3 semantics may not activate before the selected activation point.

## 10. Resource contract

Build 2B must measure elapsed time, peak memory, journal bytes, checkpoint bytes, materialized SMT nodes, SHA-256 count, recovery time and replay distance. Required large-state cases are exactly 70,000 and 200,000 accounts.

Existing 62,600/200,000 checkpoint figures are analytical only.

## 11. Forbidden shortcuts

Build 2B must not use JSON as consensus serialization, rely on map iteration order, use floating point for consensus integers, silently select D1-D7, retain the old full-state checkpoint as the C-2 journal model, retain a fixed 4 MiB single-record ceiling for valid state, skip corruption offsets, claim restart/reorg correctness without clean-replay comparison, change V1/V2 behavior for convenience, or change frozen vectors merely to fit implementation.

## 12. Governance rule

If Build 2B discovers a protocol question not answered by the frozen package, implementation must stop and return the question to the owner decision register.

**Build 2B is NOT AUTHORIZED by this audit.**
