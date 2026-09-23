# Governance Specification

**Status:** Draft
**Target protocol version:** Phase 9 / target protocol version 2.x
**Scope:** Defines a proposed governance lifecycle and authority separation without implementing governance execution.

> Phase 9 is a protocol-formalization phase. It specifies contracts and architecture; it does not implement PoS/BFT, staking, validator lifecycle, governance execution, a state tree, smart contracts, or a production network.


## 1. Authority model

Governance authority and consensus authority MUST remain separate. Governance can authorize protocol changes under explicit rules; validators apply the active protocol and MUST NOT invent governance outcomes.

## 2. Proposal lifecycle

```text
Draft -> Submitted -> Deposit satisfied -> Voting -> Passed/Rejected/Vetoed
       -> Timelock -> Scheduled execution -> Executed
```

The state machine MUST record proposal identity, proposer, type, deposit, voting period, voting power snapshot/rules, outcome, and execution status.

## 3. Proposal types

Supported conceptual types:
- parameter change;
- software/protocol upgrade;
- treasury action;
- emergency action;
- validator/economic policy change where authorized.

Smart-contract governance is not assumed.

## 4. Numerical parameters

The following are intentionally not finalized:
- governance quorum;
- approval threshold;
- veto threshold;
- proposal deposit;
- voting period;
- deposit refund/burn;
- upgrade timelock duration.

Each is **TBD — requires governance and economic review.**

## 5. Parameter changes

A passed parameter proposal MUST identify the exact parameter, old value, new value, activation rule, and compatibility impact. Parameters affecting consensus MUST NOT change retroactively unless the protocol explicitly defines a migration.

## 6. Software upgrades

An upgrade proposal MUST identify software/protocol version, activation height/epoch, migration requirements, rollback/recovery plan, and validator/operator communication. Upgrade execution MUST be versioned and deterministic.

## 7. Emergency halt

Emergency halt authority MUST be narrowly scoped, auditable, time-limited where possible, and unable to silently rewrite historical state. The exact authority model is:

**TBD — requires governance, security, and legal review.**

## 8. Timelocks and multisig

Sensitive treasury or upgrade actions SHOULD use explicit timelocks and multi-party authorization. The number of signers, threshold, and timelock duration are TBD.

## 9. Capture defenses

Governance SHOULD model stake concentration, vote buying, validator-cartel coordination, treasury concentration, proposal spam, low-turnout attacks, and emergency-authority abuse. Mitigations require measurable simulation.

## 10. Key separation

No single administrative key may unilaterally upgrade consensus, modify supply, or finalize arbitrary state (I-016). Governance keys are not consensus keys.

## 11. State-transition pseudocode

```text
submit(proposal):
    authenticate(proposer)
    require depositRule(proposal)
    store(proposal)

vote(voter, proposal, choice):
    authenticate(voter)
    require proposal.votingOpen
    recordVote()

tally(proposal):
    require votingClosed
    computeOutcomeDeterministically()

execute(proposal):
    require outcome == Passed
    require timelockSatisfied
    applyAuthorizedChange()
```

## 12. Failure modes

Proposal replay, double voting, vote-weight mismatch, deposit accounting errors, malicious upgrade, governance capture, emergency-key compromise, and non-deterministic execution MUST be rejected or halted.

## 13. Compatibility and migration

Governance is not implemented as a production execution layer in V2. A future migration MUST establish governance state and authority boundaries explicitly.

## 14. Required test vectors

Deposit; proposal identity; duplicate proposal; vote authorization; quorum boundary; approval boundary; veto boundary; timelock; parameter change; upgrade; emergency halt; key separation; capture simulation.

## 15. Open questions

All numeric thresholds and emergency authority details are TBD.

## 16. Review checklist

- [ ] Governance execution is not implemented.
- [ ] Consensus/governance separation is explicit.
- [ ] Numeric thresholds remain TBD.
- [ ] Upgrade execution is versioned.
