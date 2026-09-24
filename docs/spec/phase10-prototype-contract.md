# Phase 10 Prototype Contract

**Status:** Frozen implementation boundary candidate
**Depends on:** Accepted/frozen Phase 9 + Phase 9.2 contract
**Target:** CometBFT + Cosmos SDK prototype on a local 4–7 validator network
**Production status:** NOT A LAUNCH

> **Phase 10 is a prototype, not a launch.** It MUST NOT become a public network, mainnet, incentivized testnet, public devnet, token-sale system, or production validator network.

## 1. Scope statement

Phase 10 scope is exactly:

> **“CometBFT + Cosmos SDK prototype implementing the frozen Phase 9 contract on a local 4–7 validator network.”**

No implementation work outside this boundary is authorized by this contract.

## 2. Entry criteria

Phase 10 MUST NOT begin until all of the following are satisfied:

- Phase 9.2 invariant traceability is reviewed;
- `network_id` canonical decision is accepted;
- `genesis-v2.json` and `genesis-spec.md` are aligned;
- economic/governance/security/legal TBD items remain explicitly unresolved where not approved;
- the Phase 9 launch gates permit a private prototype;
- no mainnet genesis exists;
- no public token distribution or production economic launch is planned as part of the prototype.

## 3. Allowed scope

The following are allowed inside `prototype/cosmos/` or directly supporting the local prototype:

- deterministic genesis tooling;
- accounts;
- native SYJ bank module;
- transaction admission and execution flow;
- validator registration;
- staking;
- delegation;
- unbonding/redelegation;
- slashing evidence handling;
- rewards accounting using explicitly labeled prototype parameters;
- governance skeleton;
- state-root integration;
- local validator orchestration;
- metrics, health checks, and logging;
- automated startup and teardown;
- unit/integration/property/fuzz/differential/chaos/partition test infrastructure;
- V2 differential-test compatibility where applicable;
- deterministic local genesis and network identity verification.

Prototype economic values MUST be clearly marked as test configuration and MUST NOT be represented as final production economics.

## 4. Forbidden scope

Phase 10 MUST NOT include:

- public network;
- mainnet;
- mainnet genesis;
- token sale;
- public token distribution;
- exchange integration;
- smart-contract runtime;
- production economic launch;
- production treasury;
- production validator network;
- incentivized testnet;
- public devnet;
- final economic parameters;
- final governance thresholds;
- production emergency authority;
- claims of production readiness or mainnet readiness.

## 5. Repository layout

A compliant prototype SHOULD use a clearly isolated layout such as:

```text
prototype/cosmos/
├── app/
├── cmd/
├── modules/
│   ├── bank/
│   ├── staking/
│   ├── governance/
│   └── state-root/
├── genesis/
├── network/
├── test/
└── README.md
```

This is a contract for organization, not a requirement to create these files during Phase 9.2.

## 6. Required module boundaries

### Accounts and bank

The native SYJ bank module MUST own balances and ordinary account transfers. It MUST NOT expose an unrestricted mint operation.

### Staking

Staking MUST own delegation, unbonding, redelegation, stake accounting, and the derivation of validator voting power from authenticated stake state.

### Validator lifecycle

Validator registration, activation, jail/tombstone state, consensus-key rotation, and deterministic set transitions MUST be isolated from ordinary account transfer logic.

### Governance

Governance MUST own proposal state, deposits, voting, timelocks, and execution scheduling. Final numeric thresholds remain TBD until separately approved.

### State root

The state-root module MUST expose a deterministic commitment over all consensus-relevant application state. The concrete tree/proof algorithm remains subject to the Phase 9/ADR-009 contract and security review.

## 7. Key separation

The prototype MUST keep four logical key classes separate:

1. **Account/signing key** — signs ordinary user transactions.
2. **Node/P2P identity key** — authenticates node identity and peer sessions.
3. **Consensus key** — signs consensus votes/evidence.
4. **Governance/treasury key** — authorizes governance/treasury actions.

A prototype MAY use development-only key material, but it MUST NOT reuse a single private key across these roles. No production secret or wallet material belongs in the repository.

## 8. Deterministic local network

The local prototype MUST:

- run 4–7 validator processes;
- use one deterministic approved local genesis;
- derive and verify one `network_id` for every node;
- derive the external CometBFT `chain-id` from that same `network_id` using ADR-013;
- reject mismatched genesis/network identity;
- expose no public production endpoint by default;
- permit repeatable startup, shutdown, restart, and recovery.

The exact number of validators used in an individual test run MAY vary within 4–7.

## 9. Startup, shutdown, restart, recovery

Startup MUST validate schema, genesis commitment, network identity, module-state consistency, and key-role separation before joining consensus.

Shutdown MUST preserve the durable state required for safe restart. A restarted validator MUST NOT sign conflicting consensus messages merely because local state is incomplete or stale.

Recovery tests MUST include clean restart, crash/restart, delayed state restoration, validator replacement in the local topology, and recovery after a simulated partition.

## 10. Required testing contract

Phase 10 MUST provide evidence for:

- unit tests for module invariants;
- integration tests across bank/staking/governance/state-root boundaries;
- property tests for supply, nonce, stake, and deterministic execution invariants;
- fuzz tests for transaction/genesis/consensus message parsing;
- differential tests against frozen V2 vectors where compatibility is intended;
- chaos tests for validator/process/network failures;
- partition tests for safety and restart behavior;
- equivocation/slashing evidence tests;
- upgrade/migration rehearsal tests;
- reproducible genesis/state-root tests.

No test result may be fabricated or reported as executed unless actually run.

## 11. Observability minimums

The prototype MUST expose sufficient structured telemetry to diagnose:

- block height and finalization progress;
- proposer/validator participation;
- consensus round/step transitions;
- transaction admission/rejection reasons;
- state-root values;
- validator-set changes;
- staking and governance state transitions;
- peer connectivity and partition symptoms;
- startup/genesis validation failures;
- invariant/test failures.

Sensitive key material MUST NOT be logged.

## 12. Security-review gates

Before any movement beyond the local prototype, reviewers MUST verify:

- key separation;
- network identity binding;
- genesis reproducibility;
- consensus-message authentication and replay protection;
- validator-set transition safety;
- state-root determinism;
- supply/issuance invariants;
- governance authorization boundaries;
- RPC/P2P exposure controls;
- dependency/version pinning;
- secret handling;
- failure/recovery behavior.

External security review is required before any public or production launch consideration.

## 13. Rollback strategy

Prototype upgrades MUST be versioned and reproducible. A failed migration MUST stop activation rather than silently mutate state. Recovery MUST identify the last valid state commitment, the active protocol version, and the approved rollback/migration boundary.

No rollback mechanism may authorize an operator to rewrite finalized state outside the protocol's explicit governance/recovery rules.

## 14. V2 compatibility rules

- Existing V2 transaction vectors MUST remain available as differential/reference evidence.
- Phase 10 MUST NOT silently modify V2 signing domains, transaction-ID domains, or existing `network_id` vectors.
- Any intentional incompatibility MUST be introduced by a new versioned contract and migration rule.
- The audited private-testnet identity is not a production identity.

## 15. Exit criteria

Phase 10 is complete only when the local prototype demonstrates, with reproducible evidence:

- 4–7 validator startup and stable operation;
- deterministic genesis/network identity validation;
- transaction execution and bank invariants;
- validator/staking state transitions;
- governance skeleton behavior;
- deterministic state-root construction under the approved prototype algorithm;
- restart/recovery safety;
- partition behavior;
- equivocation evidence handling;
- differential compatibility results where applicable;
- required observability;
- documented unresolved risks and explicit next-gate decisions.

Passing these criteria does **not** mean production-ready or mainnet-ready.

## 16. Non-launch statement

Phase 10 is a private/local engineering prototype. It is not a mainnet, public devnet, public testnet, incentivized testnet, token launch, exchange integration, or production validator network. Any transition beyond the local prototype requires a separate governance/security/operations decision and updated launch-gate evidence.
