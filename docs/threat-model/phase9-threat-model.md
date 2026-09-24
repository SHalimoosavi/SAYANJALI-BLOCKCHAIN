# Phase 9 Threat Model

**Status:** Draft
**Target protocol version:** Phase 9 / target protocol version 2.x
**Scope:** Threat-oriented security requirements for the proposed production protocol.

> Phase 9 is a protocol-formalization phase. It specifies contracts and architecture; it does not implement PoS/BFT, staking, validator lifecycle, governance execution, a state tree, smart contracts, or a production network.


## Method

The threat model distinguishes specification from implementation. A listed defense is not evidence that the current network already has that defense.

## Threat register

| ID | Threat | Asset at risk | Attacker capability | Attack path | Required defense | Detection signal | Launch gate | Status |
|---|---|---|---|---|---|---|---|---|
| TM-001 | Double spend | Balances and transaction validity | Attacker controls funded account or attempts conflicting transactions | Submit conflicting spends; exploit nonce/race/state inconsistency | Atomic nonce/balance transition; deterministic execution; finalized state root | Conflicting nonce, duplicate identity, balance mismatch, divergent state root | Consensus transaction/state tests and independent audit | SPECIFIED / current V2 has nonce/replay controls |
| TM-002 | Long-range attack | Chain history and validator trust | Attacker obtains old validator keys or creates an alternative history | Build alternate history from old stake keys | Finalized checkpoints, key-history/evidence policy, genesis governance, weak-subjectivity operational controls | Conflicting finalized checkpoints or old-key signatures | Mainnet security design and external audit | PROPOSED |
| TM-003 | Eclipse attack | Validator/node network view | Attacker controls or filters peer connections | Isolate a node from honest peers | Authenticated P2P identity, peer diversity, sentry topology, rate limits | Peer concentration, unusual peer churn, identical network routes | Private/public testnet network-resilience tests | Partially implemented networking; production controls PROPOSED |
| TM-004 | Validator cartel | Consensus influence and governance | Stake concentrated among coordinated validators | Coordinate voting/censorship | Stake concentration monitoring, delegation diversity, governance safeguards | Voting concentration and correlated operator telemetry | Decentralization launch gate | SPECIFIED |
| TM-005 | Censorship | Transaction inclusion | Validator subset refuses transactions | Suppress selected transactions | Inclusion monitoring, proposer rotation, governance/economic accountability | Pending transaction age and validator-specific inclusion skew | Censorship simulation | PROPOSED |
| TM-006 | Equivocation | Consensus safety | Validator signs conflicting messages | Double-sign at same height/round/step | Authenticated evidence, deterministic penalty lifecycle | Conflicting signatures | Evidence fuzzing and adversarial testnet | SPECIFIED / NOT IMPLEMENTED |
| TM-007 | Stake centralization | Consensus and governance | Large holder/custodian accumulates voting power | Concentrate delegation | Concentration telemetry, validator diversity policy, governance controls | Top-N voting power share | Economic/decentralization review | SPECIFIED |
| TM-008 | Key compromise | Funds, consensus, governance | Private key stolen | Impersonate account/validator/admin | Key separation, HSM/secure custody, rotation, evidence | Unexpected signatures/key use | Key-compromise drills | SPECIFIED / operationally incomplete |
| TM-009 | RPC abuse | Node resources and availability | Remote client floods or abuses endpoints | High-rate requests or expensive queries | Authentication, quotas, rate limits, bounded queries, read/write separation | Latency/error spikes and request-rate anomalies | Load/security testing | PROPOSED for production |
| TM-010 | P2P flooding | Network availability | Attacker sends excessive frames/connections | Exhaust sockets/CPU/bandwidth | Connection limits, frame limits, rate limiting, reputation, peer diversity | Connection/frame rejection metrics | Network flood testing | Partially implemented in current Go track |
| TM-011 | State bloat | Storage and execution | Attacker creates cheap persistent state | Spam state-creating transactions | Fees/resource costs, size limits, pruning policy | State growth per block/account | Long-run state-growth tests | SPECIFIED |
| TM-012 | Cross-network replay | Funds and consensus | Valid message/transaction copied to another network | Replay signed payload on different chain | `network_id` binding, domain separation, versioning | Wrong-network validation failures | Differential replay tests | IMPLEMENTED for current V2 transaction identity |
| TM-013 | Malicious genesis | Initial network identity | Operator or attacker supplies altered genesis | Launch nodes on unauthorized state | Genesis commitment, independent approval, derived network identity | Commitment mismatch and startup failure | Genesis reproducibility test | IMPLEMENTED for private-testnet V2 binding; production genesis NOT CREATED |
| TM-014 | Malicious upgrade | Protocol integrity | Authorized or compromised governance attempts unsafe upgrade | Activate incompatible code/rules | Versioned upgrade proposals, timelocks, multi-party authorization, migration checks | Unexpected version activation | Upgrade rehearsal | SPECIFIED |
| TM-015 | Governance capture | Protocol authority | Stake/coalition gains sufficient governance control | Pass harmful proposals | Quorum/thresholds, veto rules, timelocks, delegation diversity, emergency separation | Abnormal concentration or proposal patterns | Capture simulations | SPECIFIED / parameters TBD |
| TM-016 | Treasury key compromise | Treasury assets | Treasury credential stolen | Unauthorized transfer | Multisig, timelocks, spending policy, separate governance/consensus keys | Unexpected treasury proposal/signature | Key custody audit | SPECIFIED |
| TM-017 | Supply inflation attack | Monetary integrity | Bug/key/governance bypass creates unauthorized supply | Mint outside issuance rules | Authenticated supply state, issuance caps, deterministic accounting, no unilateral mint key | Supply delta mismatch and root mismatch | Supply-invariant/property tests | SPECIFIED |
| TM-018 | Fee-market manipulation | Transaction inclusion and user costs | Validators or users manipulate fee conditions | Spam, fee bidding attacks, underpriced execution | Resource pricing, minimum fee, bounded blocks, governance review | Fee volatility and utilization anomalies | Fee simulation/load tests | SPECIFIED / parameters TBD |

## Security assumptions

The target PoS/BFT design assumes authenticated validator keys, a bounded Byzantine voting-power fraction for standard BFT safety, deterministic application execution, reliable key custody, and adequate network diversity. These assumptions require operational evidence before production.

## Residual-risk principles

No protocol can eliminate all operational risk. Phase 9 therefore requires layered controls: deterministic state rules, cryptographic authentication, validator diversity, governance separation, secure custody, monitoring, incident response, and independent review.

## Required adversarial testing

At minimum, test:
- conflicting spends and nonce races;
- 10/25/33/50/2/3 stake control scenarios;
- partition and delayed-message behavior;
- equivocation evidence;
- long-range/checkpoint scenarios;
- peer flooding/eclipse attempts;
- state-growth workloads;
- governance capture and malicious upgrades;
- treasury and consensus-key compromise;
- fee-market manipulation.

## Review checklist

- [ ] Every threat has an asset, attacker capability, attack path, defense, signal, residual risk/launch gate, and status.
- [ ] No defense is represented as implemented unless verified.
- [ ] Operational and governance risks are included.
