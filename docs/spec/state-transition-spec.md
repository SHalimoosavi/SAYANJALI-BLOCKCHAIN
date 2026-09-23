# State Transition Specification

**Status:** Draft
**Target protocol version:** Phase 9 / target protocol version 2.x
**Scope:** Defines a deterministic application state machine without implementing a state tree or production execution runtime.

> Phase 9 is a protocol-formalization phase. It specifies contracts and architecture; it does not implement PoS/BFT, staking, validator lifecycle, governance execution, a state tree, smart contracts, or a production network.


## 1. Model

The target state machine is:

`S(n+1) = Apply(S(n), Block(n))`

Only consensus-authorized execution may change state. Query, RPC, peer, wallet, or administrative interfaces MUST NOT directly mutate consensus state.

## 2. State domains

The conceptual state namespace contains:
- account identity, balance, and next nonce;
- validator identity and lifecycle state;
- delegation and unbonding records;
- reward/commission accounting;
- governance proposals, deposits, votes, and outcomes;
- module-specific state;
- protocol version and migration metadata;
- issuance and supply accounting.

A concrete state-tree technology is deliberately not implemented in Phase 9.

## 3. Genesis

Genesis MUST uniquely determine the initial network state, chain identity, initial protocol version, authorized initial validators if any, and all consensus-relevant initial module state. A production genesis MUST NOT be created in this phase.

Genesis material MUST be canonicalized before commitment. Any mismatch between declared network identity and independently derived identity MUST fail closed.

## 4. Deterministic execution order

For each finalized block:
1. verify consensus evidence;
2. verify parent and chain identity;
3. apply deterministic system-level transitions;
4. process transactions in canonical order;
5. apply fees and issuance under active rules;
6. apply staking/validator transitions at their defined activation height;
7. apply governance changes only when their execution conditions are met;
8. compute receipts/events;
9. compute the post-state root.

No local time, random number generator, map iteration order, network arrival order, or external API response may affect the result.

## 5. Accounts and nonces

Every normal account has exactly one next nonce (I-003). A successful transaction consumes the expected nonce exactly once (I-004). A transaction with a stale, duplicate, skipped, or conflicting nonce is rejected or remains pending according to the mempool policy; consensus execution MUST be deterministic.

## 6. State root

The state root MUST commit to every consensus-relevant state domain. Account, validator, staking, governance, module, and issuance state MUST be reachable from the authenticated root. The concrete authenticated tree and proof format are specified in ADR-009 but remain unimplemented.

## 7. Failed transactions and receipts

The production protocol MUST define deterministic failure semantics. A failed transaction MUST NOT apply unauthorized state effects. Whether fee consumption survives a failed execution is a protocol rule and is **TBD — requires governance/economic review**. Receipts MUST distinguish acceptance, execution failure, and consensus rejection.

## 8. Validator and staking transitions

Validator-set changes MUST be scheduled at deterministic boundaries. Voting power MUST derive from authenticated stake state. Unbonding, jailing, tombstoning, and redelegation state MUST not alter the currently active validator set until the specified activation point.

## 9. Governance transitions

Governance execution MUST be state-driven and deterministic. A proposal cannot directly mutate state merely because an operator claims approval. The state machine MUST verify proposal status, thresholds, timelocks, and applicable protocol version.

## 10. State pruning and archive nodes

Full validators MAY prune historical execution material only after preserving sufficient commitments and snapshot metadata to reconstruct active state. Archive nodes MUST preserve historical blocks, state commitments, receipts/events, and migration evidence required by the archival policy. Pruning rules are **TBD — requires operational review**.

## 11. Migration and versioning

State schema versions MUST be explicit. A migration MUST define:
- source version;
- destination version;
- deterministic transformation;
- activation height;
- validation checksum/root;
- rollback/failure behavior;
- compatibility window.

A migration MUST NOT depend on external mutable data.

## 12. Light-client proofs

A light client SHOULD be able to verify account/state claims using a finalized block commitment plus an authenticated proof path. Proof encoding, tree algorithm, and witness limits are **TBD — requires security/runtime review**.

## 13. Supply invariant

I-005 and I-015 require that total supply and all issuance paths are represented in authenticated state. No owner or administrative key may create supply outside active issuance rules.

## 14. Pseudocode

```text
Apply(state, block):
    verifyBlockContext(block)
    working = cloneOrTransactionalView(state)
    for tx in block.transactions:
        validateConsensusRules(tx, working)
        receipt = executeDeterministically(tx, working)
        appendReceipt(receipt)
    applyScheduledValidatorChanges(working)
    applyScheduledGovernance(working)
    enforceSupplyCeiling(working)
    root = StateRoot(working)
    return working, receipts, root
```

## 15. Security assumptions and failure modes

Nondeterministic execution, hidden mutable dependencies, inconsistent serialization, state-root bugs, migration errors, and unauthorized state mutation are critical risks. Any inability to derive the expected state root MUST halt block acceptance.

## 16. Compatibility and migration impact

Current V2 chain-state reconstruction is a reference mechanism. Migration to authenticated state roots is a protocol boundary, not an incremental database optimization. Historical V2 state MUST be mapped deterministically if retained.

## 17. Required vectors

Genesis-to-state-root; single transfer; multiple transactions; nonce conflict; fee failure; issuance; validator transition; delegation; unbonding; governance execution; migration; snapshot restore; proof verification; and supply ceiling.

## 18. Open questions

Tree algorithm, key encoding, versioned leaf format, proof limits, pruning schedule, failed-fee semantics, archive retention, module namespace rules, and migration activation rules remain TBD.

## 19. Review checklist

- [ ] `S(n+1) = Apply(S(n), Block(n))` is deterministic.
- [ ] State-root implementation is not included.
- [ ] Genesis is specified but no mainnet genesis exists.
- [ ] All state domains are authenticated.
- [ ] Migration is explicit and versioned.
