# Consensus Specification

**Status:** Draft
**Target protocol version:** Phase 9 / target protocol version 2.x
**Scope:** Proposed PoS/BFT target compatible with CometBFT assumptions; current PoW remains legacy/transitional.

> Phase 9 is a protocol-formalization phase. It specifies contracts and architecture; it does not implement PoS/BFT, staking, validator lifecycle, governance execution, a state tree, smart contracts, or a production network.


## 1. Status boundary

**CURRENT:** Go PoW chain implementation exists and is useful as a reference/research track.

**TARGET:** a validator-based BFT runtime in which finalized blocks are committed after sufficient authenticated voting power.

The current PoW architecture is **Legacy / transitional / protocol-research track — not the production consensus target.**

## 2. Proposed round flow

```text
Proposal
   |
Block validation
   |
Prevote
   |
Precommit
   |
>= 2/3 voting power for a valid commit
   |
Commit certificate
   |
Finalized block
```

The exact CometBFT version, ABCI contract, evidence format, timeout parameters, and validator-set update hooks are **TBD — requires runtime/security review**.

## 3. Proposal

The deterministic proposer selection mechanism MUST select exactly one proposer for each height/round from the active validator set. Selection MUST be derived from authenticated stake/voting power and the consensus schedule, not local preference.

## 4. Validation and voting

A validator MUST validate chain identity, block version, parent, transaction root, state transition, state root, receipt root, and proposer authorization before voting. Validators MUST NOT prevote or precommit a block that violates consensus rules.

## 5. Two-thirds rule

Finalization requires authenticated precommit evidence representing more than two-thirds of the active voting power under the active consensus rules. Exact rounding/weight arithmetic MUST be deterministic and specified before implementation.

## 6. Time and liveness

Consensus MUST NOT depend on local wall-clock values for state validity (I-010). Local timers MAY drive round progression, but blocks are accepted based on authenticated consensus messages and deterministic validity rules. A network partition may prevent progress; safety takes precedence over unsafe finalization.

## 7. Byzantine assumptions

The target assumes fewer than one-third of active voting power is Byzantine for standard BFT safety/liveness conditions. This is a security assumption, not a measured property of the current network.

## 8. Validator-set updates

Set changes MUST become active at deterministic heights/epochs. The block that schedules a change MUST commit the exact next set or a state commitment from which it can be deterministically derived.

## 9. Finality and reversion

Once a block has a valid commit certificate under the active protocol, it is finalized and MUST NOT be reverted under normal protocol rules (I-007). Recovery from catastrophic corruption is a governance/operational event and MUST NOT be silently treated as an ordinary reorg.

## 10. Equivocation evidence

A validator signing conflicting consensus messages for the same height/round/step constitutes equivocation. Evidence MUST be independently verifiable and bound to chain identity. Evidence processing MUST be deterministic and MUST lead to the specified penalty lifecycle.

## 11. Message authentication and replay protection

Every consensus-critical message MUST be authenticated, bound to chain/network identity, include height/round/step context, and be replay-protected (I-017). Transport encryption may provide confidentiality but is not a substitute for message-level authentication.

## 12. Partitions and restart

During a partition, validators MUST refuse to finalize conflicting histories. After restart, a validator MUST recover its durable consensus state and refuse to sign an unsafe conflicting vote. Evidence and commit state MUST be durable according to the runtime's safety requirements.

## 13. Upgrades and forks

Consensus upgrades require explicit versioning, activation rules, governance authorization, and a compatibility boundary (I-013). A validator MUST NOT infer an upgrade solely from an observed message or local configuration.

## 14. Security consequences

BFT finality reduces the normal need for accumulated-work chain selection but introduces validator-key, stake-concentration, evidence, and governance risks. These are addressed by the validator/staking/threat-model specifications.

## 15. Failure modes

Insufficient voting power, conflicting proposals, invalid evidence, validator-set mismatch, network partition, unavailable proposer, corrupted local consensus state, and incompatible upgrades MUST result in non-finalization or deterministic recovery—not ambiguous chain acceptance.

## 16. Compatibility

No current PoW block is automatically a BFT-finalized block. A migration boundary MUST explicitly define the last legacy block and first production-consensus genesis/transition state.

## 17. Required test vectors

Single-validator development case; 4–7 validator private network; proposer failure; one-third Byzantine; two-thirds commit; equivocation evidence; partition; restart; validator-set change; upgrade activation; replayed consensus message; wrong-network message; conflicting proposal.

## 18. Open questions

Exact runtime version, timeout configuration, evidence schema, voting-power arithmetic, epoch boundary, validator-set update timing, upgrade activation, and emergency halt behavior are TBD.

## 19. Review checklist

- [ ] Proposal → prevote → precommit → commit is explicit.
- [ ] >2/3 voting power is explicit.
- [ ] PoW remains legacy.
- [ ] Finality is specified, not claimed as current.
- [ ] Consensus implementation is absent.
