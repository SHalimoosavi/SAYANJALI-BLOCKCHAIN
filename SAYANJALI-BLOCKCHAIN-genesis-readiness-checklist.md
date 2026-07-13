# SAYANJALI BLOCKCHAIN — Genesis Readiness Checklist

Based on repo state as of `v0.1.0-mvp` (Jul 11, 2026): Python 3.13, FastAPI, SQLAlchemy Core, PoW consensus (SHA-256/ECDSA-secp256k1), 37/37 tests passing, **no P2P networking yet**.

The README already names four honest limitations. This checklist starts there and expands into everything needed before SYJ genesis is minted.

---

## Phase A — Close the README's own disclosed gaps

These are the project's *own* stated blockers. Nothing below should be considered "production-ready" until all four are resolved.

- [ ] **P2P networking built** — currently "each node is authoritative over its own local chain state." Genesis cannot happen on a single-node chain; that's not decentralization, it's a database.
- [ ] **`/transaction/sign` endpoint removed or reworked** — it currently accepts a private key over HTTP. This must become client-side signing (sign locally, submit only the signed tx) before any public-facing deployment.
- [ ] **Real difficulty retargeting algorithm** — replace the "conservative, bounded adjustment" with a full ratio-based algorithm (Bitcoin-style: `new_difficulty = old_difficulty * (target_time / actual_time)`, clamped to prevent oscillation).
- [ ] **API hardening** — add authentication, rate limiting, TLS termination, and lock down CORS (currently permissive by default).

---

## Phase B — P2P layer: what to test

### Node discovery & connectivity
- [ ] New node with zero prior peers can bootstrap and sync from genesis using only bootstrap node addresses
- [ ] Node survives peer disconnect/reconnect cycles without state corruption or silent forking
- [ ] Tested across real network conditions — different machines/providers, real latency, real NAT — not just localhost
- [ ] Minimum bar before genesis: **5–10 independently operated nodes**, different machines/operators, running continuously for **2–4 weeks**

### Network partition & sync
- [ ] Kill a node mid-sync, restart it — confirm it resumes correctly without corrupting local chain
- [ ] Simulate a network split (two node groups isolated) — on reconnection, chain reconciles to one canonical fork, not two permanent forks
- [ ] Fresh node syncing from genesis on a chain with thousands of blocks — measure time, memory usage, confirm no stalls/OOM

### Message propagation
- [ ] Measure actual block/tx propagation time across all nodes (don't estimate — log it)
- [ ] Duplicate message handling — a node must not re-process/re-broadcast the same block or tx repeatedly
- [ ] Malformed/truncated message handling — node drops the bad connection, does not crash

### Load & adversarial conditions
- [ ] Flood a node with junk tx/connection requests — confirm graceful degradation, not crash (basic DoS resistance, tested against your own nodes)
- [ ] Conflicting blocks submitted at the same height by different nodes — confirm the fork-choice rule resolves deterministically and identically on every node

---

## Phase C — Security review: what it must cover

### Consensus-level
- [ ] Assess realistic mining centralization risk — with genuinely few real miners at launch, how cheap would it be for one actor to dominate hashrate? State this risk plainly rather than assuming it away.
- [ ] Double-spend test — submit two conflicting transactions spending the same input; confirm only one ever confirms, across all nodes
- [ ] Explicit 51%-style attack cost estimate for your actual expected launch hashrate — for a brand-new low-hashrate PoW chain this is often trivially cheap, and that needs to be stated in your security docs, not hidden

### Cryptographic
- [ ] ECDSA/secp256k1 wallet implementation reviewed specifically for nonce reuse/predictability (classic private-key leak vector) — you're already using the audited `ecdsa` + `cryptography` libraries, which is the right call; confirm no custom crypto has crept in anywhere
- [ ] Confirm signing has moved fully client-side (ties back to Phase A item 2)

### Smart contract layer (once execution environment — Phase 9 on your roadmap — is built)
- [ ] Reentrancy, integer overflow/underflow, unchecked external calls — standard checklist
- [ ] Gas/fee metering cannot be bypassed to cause infinite loops or node resource exhaustion

### Operational
- [ ] Genesis parameters (100B SYJ supply, allocation split) reviewed by someone other than you before being hardcoded into `GenesisConfig` — this is irreversible once genesis mines, so get a second set of eyes first
- [ ] Any treasury/admin wallet keys moved to cold storage — not living on the Android/Termux device used for development
- [ ] `SYJ_DIFFICULTY`, `SYJ_TARGET_BLOCK_TIME`, and `SYJ_BLOCK_REWARD` values finalized and reviewed — these are currently dev defaults (difficulty `4`, block time `30s`, reward `50.0`) and need real economic modeling before mainnet, not just environment-variable defaults left as-is

---

## Phase D — Before flipping genesis

- [ ] Block explorer live (Roadmap Phase 8) — external parties need to verify chain state without trusting your word
- [ ] Full difficulty retargeting shipped (Roadmap Phase 7)
- [ ] Formal or informal security audit completed — even 1–2 paid independent developers reviewing P2P + consensus + fork-choice logic specifically, since that's where amateur chains fail first
- [ ] Genesis config reviewed and **locked** — no further tokenomics changes possible after this point
- [ ] Wallet software (create-wallet / signing flow) tested by someone other than you, from a clean environment, to confirm it's usable without your guidance

---

## Notes
- Your roadmap already sequences this correctly: Phase 6 (P2P) → Phase 7 (retargeting/PoS groundwork) → Phase 8 (explorer) → Phase 9 (contracts) → Phase 10 (governance/SDK) → Phase 11 (mainnet + SYJ launch). Don't skip ahead to Phase 11 items before 6–8 are genuinely solid — that's the order that turns "37/37 tests passing" into "network survives real adversarial conditions."
- The README's MIT license explicitly does **not** cover SAYANJALI/SYJ trademarks — worth keeping in mind for any future contributor agreements.
