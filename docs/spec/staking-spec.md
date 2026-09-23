# Staking Specification

**Status:** Draft
**Target protocol version:** Phase 9 / target protocol version 2.x
**Scope:** Defines the proposed staking state machine and economic-security interfaces without implementing staking.

> Phase 9 is a protocol-formalization phase. It specifies contracts and architecture; it does not implement PoS/BFT, staking, validator lifecycle, governance execution, a state tree, smart contracts, or a production network.


## 1. Scope

Staking is a consensus-security mechanism, not merely a rewards feature. It MUST establish authenticated voting power and deterministic validator-set membership.

## 2. Operations

Conceptual operations:
- self-delegation;
- delegation;
- undelegation;
- redelegation, if approved;
- validator registration;
- commission configuration;
- reward withdrawal;
- evidence/slashing handling.

Every operation MUST be authorized by the relevant account and validated against current state.

## 3. Self-delegation and delegation

A validator MAY self-delegate. Other accounts MAY delegate according to protocol rules. Voting power MUST derive from the authenticated stake state after applying activation rules.

## 4. Unbonding

Unbonding creates a delayed withdrawal state so stake does not immediately disappear from security accounting. The unbonding period is:

**TBD — requires governance and economic review.**

An account MUST NOT spend or redelegate stake that is locked in an active unbonding record.

## 5. Redelegation

If enabled, redelegation MUST preserve deterministic accounting and prevent rapid stake movement from bypassing penalties or unbonding safety. Limits and cooldowns are **TBD**.

## 6. Slashing and evidence

Evidence MUST identify the validator, chain/network, height/round/step, signed messages, and cryptographic proof of the violation. The state machine MUST verify evidence before applying a penalty. Slash percentages are **TBD — requires governance and economic review**.

## 7. Rewards

Validator and delegator rewards MUST be derived only from authorized issuance and fee rules. Validator reward schedule and delegator reward formula are:

**TBD — requires governance and economic review.**

No reward value is final in Phase 9.

## 8. Commission

Commission exists as a mechanism for validator operating costs and delegation economics. Commission bounds and change cadence are:

**TBD — requires governance and economic review.**

## 9. Withdrawal

Withdrawals MUST respect unbonding, reward accounting, and any applicable governance/penalty state. Withdrawal MUST NOT bypass supply accounting.

## 10. Anti-capture controls

The protocol SHOULD monitor:
- top-validator voting-power concentration;
- correlated operators;
- delegation concentration;
- rapid stake accumulation;
- exchange/custodian concentration;
- governance-vote concentration.

No threshold is final.

## 11. Economic invariants

I-005 and I-015 require all issuance and reward paths to stay within the monetary-policy ceiling and authorized issuance rules.

## 12. Simulation matrix

Required adversarial simulations:

| Scenario | Required stake/control | Evidence |
|---|---:|---|
| Stake attacker | 10% | Behavior and recovery report |
| Stake attacker | 25% | Safety/liveness report |
| Stake attacker | 33% | Boundary analysis |
| Stake attacker | 50% | Capture/censorship analysis |
| Stake attacker | 2/3 | Finality/capture analysis |
| Downtime | TBD | Jail/recovery behavior |
| Censorship | TBD | Inclusion/finality behavior |
| Equivocation | TBD | Evidence/slashing behavior |
| Cartel | TBD | Concentration/capture analysis |

These are test scenarios, not claims that the network currently resists them.

## 13. State-transition pseudocode

```text
delegate(owner, validator, amount):
    authorize(owner)
    require spendableBalance(owner) >= amount
    move amount to delegation(owner, validator)
    updateStakeState()
    scheduleVotingPowerActivation()

undelegate(owner, validator, amount):
    authorize(owner)
    require activeDelegation >= amount
    createUnbondingRecord()
    scheduleWithdrawal()
```

## 14. Failure modes

Unauthorized delegation, double withdrawal, stake accounting mismatch, evidence replay, reward over-issuance, commission manipulation, and unbonding bypass MUST fail deterministically.

## 15. Compatibility and migration

No staking state exists in the current V2 PoW chain as a production consensus mechanism. Any migration MUST create staking state deterministically and MUST NOT infer validator power from administrative assumptions.

## 16. Open questions

Minimum validator stake; maximum validator count; unbonding period; reward schedule; delegator formula; commission bounds; slashing rates; downtime threshold; redelegation rules are all TBD.

## 17. Review checklist

- [ ] No staking implementation included.
- [ ] All economic values are TBD.
- [ ] Evidence is authenticated and replay-protected.
- [ ] Simulation matrix is included.
