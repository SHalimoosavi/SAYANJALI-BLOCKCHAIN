# ADR-012: Consensus Runtime

**Status:** Draft
**Target protocol version:** Phase 9 / target protocol version 2.x
**Scope:** Proposed production application/consensus runtime direction.

> Phase 9 is a protocol-formalization phase. It specifies contracts and architecture; it does not implement PoS/BFT, staking, validator lifecycle, governance execution, a state tree, smart contracts, or a production network.


## Decision

**PROPOSED, NOT IMPLEMENTED:** use **Cosmos SDK + CometBFT** as the target production architecture.

```text
Clients
  |
  v
RPC/API
  |
  v
Cosmos SDK application
  |-- accounts / bank / staking / governance modules (target)
  |-- deterministic state transition
  |-- authenticated state root
  |
  +---- ABCI ----> CometBFT (target)
                      | proposal
                      | prevote
                      | precommit
                      | commit
                      v
                  Validator P2P
```

The existing Go/Python implementations remain reference/research/compatibility tracks during migration.

## Security boundary

Application code determines transaction validity and deterministic state transitions. CometBFT determines consensus rounds and commit evidence. Cryptographic key custody and network infrastructure sit outside the deterministic application boundary but are security-critical.

## Why this direction

The proposed architecture separates application logic from BFT consensus, provides a well-defined validator runtime boundary, and allows the SYJ protocol contract to remain explicit. It also avoids writing a custom BFT engine before the protocol has been frozen.

## Alternatives

| Option | Tradeoffs |
|---|---|
| Cosmos SDK + CometBFT | Strong application/consensus separation; broad ecosystem; migration work remains substantial. |
| Polkadot SDK | Powerful shared-security and runtime model; different architecture and operational assumptions; migration would require adopting its framework. |
| Avalanche L1 | Flexible validator/network model; introduces a different consensus/runtime ecosystem and operational model. |

This is a comparison of architecture characteristics, not a claim that any option is universally superior.

## Migration plan

1. Freeze Phase 9 specifications.
2. Build invariant-to-test mapping.
3. Prototype a 4–7 validator local/private network.
4. Implement state/transaction adapters.
5. Differential-test against current V2 reference vectors where compatibility is intended.
6. Add state-root and migration tooling.
7. Conduct security review.
8. Run launch gates before any public testnet.

## Consequences

Positive: explicit application/consensus boundary, reusable consensus runtime, validator lifecycle hooks. Negative: new framework dependencies, migration complexity, learning curve, and the need to reconcile current V2 semantics with the target application model.

## Open questions

Exact Cosmos SDK/CometBFT versions, module selection, ABCI interface, state-tree implementation, key custody, telemetry, upgrade mechanism, and operational topology are TBD.

## Review checklist

- [ ] Cosmos SDK + CometBFT is labeled proposed.
- [ ] No integration code is included.
- [ ] Polkadot SDK and Avalanche L1 are documented as alternatives.
- [ ] Migration begins only after Phase 9 freeze.
