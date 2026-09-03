# SAYANJALI BLOCKCHAIN — Productionization State

**This file is the persistent project state for the productionization
effort governed by the Master Directive. Update it after every completed
major phase. Do not let project state exist only in conversation memory.**

Last updated: Phase 1 completion

---

## Current Phase

**Phase 1 — Protocol Correctness & Security Hardening**
Status: **PASS**

## Completed Phases

- Phase 0 — Current-state audit
- Phase 1 — Protocol correctness & security hardening (three approved
  CRITICAL findings remediated)

## Current Phase Status

Phase 1 complete. All three approved CRITICAL findings from Phase 0 have
been fixed, tested, and verified. 231/231 tests pass (195 baseline + 36
new Phase 1 tests), including a real two-process multi-node integration
run. No PoS/DPoS, smart contracts, genesis changes, or unrelated features
were introduced.

## Approved Next Phase

None yet. Phase 2 of the Master Directive's sequence (P2P production
hardening) is not started and not approved. Awaiting explicit approval.

## Environment Limitation

Still applies: no `.git` directory exists in this environment. Git
status, diff --check, and history commands could not be run in their
literal form; manual equivalents (conflict-marker scan, trailing
whitespace scan on every touched file) were performed instead and came
back clean.

## Blocked Gates

- Phase 2 cannot begin without explicit approval.
- Git-derived evidence remains unavailable in this environment.

## Resolved CRITICAL Findings (Phase 1)

### 1. Consensus difficulty enforcement — RESOLVED

**Previous behavior:** every validation call site passed a floor of `1`
rather than the protocol-expected retargeted value; `next_difficulty()`
was used for mining but never for validation.

**Fix:** `blockchain/validators.py::expected_difficulty()` independently
re-derives the protocol-required difficulty from chain history using the
same windowing logic as mining. `validate_block_against_chain()` now
requires an exact match (`block.difficulty == expected_difficulty(...)`),
not a floor, and applies identically to locally-mined, propagated, and
synchronized blocks.

**Enforcement point:** `blockchain/validators.py::validate_block_against_chain`,
called from `Blockchain.mine_pending_transactions`, `Blockchain.replace_chain`
(via `validate_chain`), and `blockchain/network/propagation.py::receive_block`.

**Tests:** `tests/test_difficulty_enforcement.py` (12 tests) — exact-match
acceptance, artificially low/high rejection, propagated-block rejection,
synchronized-chain rejection, retarget-boundary manipulation, between-retarget
manipulation, competing chain with fabricated difficulty, determinism,
first-window behavior, multi-window progression, restart behavior.

### 2. Coinbase/reward enforcement — RESOLVED

**Previous behavior:** no code checked a coinbase transaction's amount
against `MiningConfig.block_reward`; only "at most one coinbase" was
enforced, not "exactly one."

**Fix:** `blockchain/validators.py::validate_block_structure()` now takes
an `expected_coinbase_reward` parameter and requires exact equality, plus
rejects zero coinbase transactions (not just more than one).

**Enforcement point:** `blockchain/validators.py::validate_block_structure`,
reached from the same three call sites as difficulty enforcement.

**Tests:** `tests/test_coinbase_enforcement.py` (11 tests) — exact reward
accepted, reward+1 rejected, reward far above rejected, reward below
rejected, zero coinbase rejected, duplicate coinbase rejected (regression),
malformed amount rejected, full-chain-validation rejection, manipulated
transaction list rejected, enforcement across retarget windows, enforcement
during reorg.

### 3. Mempool double-spend / pending-spend accounting — RESOLVED

**Previous behavior:** confirmed live in Phase 0 — two transactions each
individually affordable against confirmed balance were both accepted,
producing a confirmed balance of -30.0 that `is_chain_valid()` reported
as valid.

**Fix:** `Mempool.pending_spend_for(address)` sums a sender's currently-held
pending transaction amounts. `Blockchain.submit_transaction()` now checks
`already_pending + amount > confirmed_balance`, not confirmed balance
alone, with the check-then-add sequence made atomic under
`Blockchain.mutation_lock`.

**Accounting model:** confirmed chain state remains the sole balance
authority (per directive requirement); pending-spend accounting is a
purely additive, in-memory admission-control layer on top of it, not a
second consensus engine. Computed on demand from live mempool contents,
not a separately-maintained running total, avoiding state-drift risk.

**A genuine deadlock was found and fixed during this work:** wrapping
`submit_transaction` in `mutation_lock` caused a real deadlock, since
`blockchain/network/propagation.py::receive_transaction` already held
that same lock when calling `submit_transaction`, and the lock was a
plain non-reentrant `threading.Lock`. Fixed by switching to
`threading.RLock`, which preserves the same cross-thread mutual-exclusion
guarantee while permitting safe same-thread re-entry. This was caught by
the test suite genuinely hanging (timeout), not by inspection — reported
here in the interest of full honesty about how the fix was actually found.

**Tests:** `tests/test_mempool_pending_spend.py` (13 tests) — the exact
reproduced scenario, three-way overspend, exact-balance boundary,
pending-then-valid-remainder, duplicate rejection (regression), cross-sender
independence, confirmation releases reservation, rejection reserves
nothing, restart clears mempool state (documented as unchanged, pre-existing
behavior), concurrent submission race (proves the deadlock fix and the
race-closure both work), reorg preserves confirmed-balance authority.

## Test Status

```
python -m compileall -q .        -> exit 0
python -m pytest -q              -> 231 passed, 1 warning, 0 failed
python -m pytest -q tests/test_multi_node_integration.py -> 4 passed
```
The 1 warning is the same pre-existing, unrelated `httpx`/`starlette.testclient`
deprecation notice noted in the Phase 0 audit.

**Six pre-existing test files required updates**, all for the same
underlying reason: they manually constructed blocks via `Miner.mine_block()`
with a hardcoded literal difficulty (`1`, or a static config value)
rather than the live protocol-expected value, which the old, unenforced
validation never checked. Updated files: `tests/test_consensus_retarget.py`,
`tests/test_network_concurrency.py`, `tests/test_network_propagation.py`,
`tests/test_network_replace_chain.py`. One of these
(`test_concurrent_mining_and_propagation_do_not_corrupt_chain`) technically
still passed with the stale hardcoded value, but only because the block
would now always lose its race for an unrelated reason (difficulty
mismatch) rather than the genuine concurrency behavior it was meant to
test -- fixed anyway, since a test passing for the wrong reason is not
verified behavior. No test assertion was weakened to force a pass; every
change made the test data itself protocol-valid.

## Infrastructure Status

Unchanged from Phase 0 -- no infrastructure work performed or required by
this phase.

## Release Status

No releases, tags, or commits were created or attempted, per explicit
instruction.

## Known Limitations (carried forward from Phase 0, still unresolved)

- HIGH: consensus monetary values remain IEEE-754 floats (not approved
  for remediation in Phase 1; the coinbase-amount check added here uses
  exact float equality safely because the value is only ever copied
  verbatim from config, never computed via arithmetic -- this does not
  resolve the broader float-representation concern).
- HIGH: timestamp rules still lack a future-timestamp bound / median-time-past
  equivalent.
- HIGH: CORS still `allow_origins=["*"]` unconditionally.
- HIGH: P2P transport still plain HTTP, no TLS enforcement.
- HIGH: no CI/CD pipeline exists.
- HIGH: dependency pinning is `>=` minimums only, no lockfile.
- MEDIUM: `/wallet/create` still transmits a private key in its HTTP
  response (server-side generation flow, distinct from the already-removed
  `/transaction/sign` flow).
- MEDIUM: no peer reputation/quarantine/banning mechanism.
- MEDIUM: rate limiting remains process-local only (self-documented).
- Reorg re-injection of orphaned mempool transactions from a discarded
  fork remains out of scope, unchanged from prior behavior -- explicitly
  and deliberately not addressed in this minimal fix, per directive
  guidance not to build a sophisticated replacement mechanism.
- No fuzz testing, property-based testing, or crash/power-loss recovery
  testing exists yet for any component, including the newly-fixed ones.

## Architecture Decisions

- `ConsensusEngine`, `Blockchain`, `Storage`, `Mempool`, and `Transaction`
  were **not** rewritten -- all three fixes were additive/surgical changes
  to existing functions and one new helper function
  (`expected_difficulty`), consistent with the directive's "smallest safe
  change" requirement.
- `validate_block_against_chain`'s signature changed from taking a single
  `previous_block: Block` to `chain_so_far: list[Block]`, since deriving
  the correct retarget window requires more chain context than one block
  provides. All three call sites were updated accordingly.
- `Blockchain.mutation_lock` changed from `threading.Lock` to
  `threading.RLock` to fix a genuine deadlock discovered while
  implementing the mempool fix (see Finding #3 above).

## Pending Approvals

1. Approval for Phase 2 of the Master Directive's sequence (P2P
   production hardening), or clarification if a different next step is
   intended.
2. The float-to-integer monetary migration (HIGH severity, not CRITICAL)
   remains an open decision from Phase 0, not addressed in Phase 1 since
   it was outside the three explicitly-approved findings.

## Phase 2 native SYJ monetary hardening

The native monetary layer now uses integer base units throughout protocol state. One SYJ equals 100,000,000 base units, and the protocol maximum is 720,000,000 SYJ (72,000,000,000,000,000 base units).

Mining issuance is supply-aware: the coinbase reward is capped at the exact remaining supply, and mining stops once no issuance remains. Chain/block validation replays monetary state so peer-submitted blocks cannot create arbitrary supply or spend an unfunded balance.

The release does not define final SYJ tokenomics or genesis allocations. Those remain a separate future protocol/economic decision.
