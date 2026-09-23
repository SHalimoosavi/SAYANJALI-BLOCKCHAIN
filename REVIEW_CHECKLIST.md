# Phase 9 Review Checklist

## Deliverable integrity

- [ ] No GitHub-write claims.
- [ ] All expected files present.
- [ ] JSON schemas parse as JSON and conform structurally to Draft 2020-12 expectations.
- [ ] Cross-document consistency reviewed.
- [ ] No implementation code.
- [ ] No mainnet genesis.
- [ ] No token distribution data.
- [ ] No smart-contract runtime.
- [ ] No PoS/staking/validator/governance execution code.
- [ ] README does not overstate maturity.
- [ ] PoW is labeled **Legacy / transitional / protocol-research track — not the production consensus target.**
- [ ] Cosmos SDK + CometBFT is labeled **PROPOSED / NOT SHIPPED**.
- [ ] SYJ is explicitly **NOT PRODUCTION-READY** and **NOT MAINNET-READY**.
- [ ] Decentralization is explicitly not yet demonstrated.

## Required invariants

| ID | Normative invariant | Specification reference | Test class | Implementation status |
|---|---|---|---|---|
| I-001 | Every valid transaction has exactly one protocol version. | transaction-spec.md §5–6 | UNIT / PROPERTY | Specified; current V2 enforces version=2 |
| I-002 | Every transaction belongs to exactly one network identity. | transaction-spec.md §5; NETWORK identity in protocol-overview | UNIT / ADVERSARIAL | Specified; current V2 validates exact network ID |
| I-003 | Every normal account has exactly one next nonce. | state-transition-spec.md §5 | PROPERTY / LONG-RUN | Specified; current V2 chain tracks nonce state |
| I-004 | A transaction cannot consume a nonce twice. | transaction-spec.md §5; state-transition-spec.md §5 | UNIT / FUZZ / ADVERSARIAL | Specified; current V2 replay/nonce paths enforce |
| I-005 | Total supply cannot exceed the monetary-policy ceiling. | state-transition-spec.md §13; staking-spec.md §11 | PROPERTY / LONG-RUN | Specified; production issuance policy TBD |
| I-006 | State transition is deterministic. | state-transition-spec.md §4; fee-spec.md §10 | PROPERTY / DIFFERENTIAL / FUZZ | Specified; production execution not implemented |
| I-007 | Finalized blocks cannot be reverted under normal protocol rules. | consensus-spec.md §9 | INTEGRATION / NETWORK-PARTITION | Proposed BFT rule; not current PoW finality |
| I-008 | Validator voting power is derived from authenticated stake state. | validator-spec.md §4; staking-spec.md §3 | PROPERTY / ADVERSARIAL | Specified; staking not implemented |
| I-009 | Validator-set transitions are deterministic. | validator-spec.md §5; consensus-spec.md §7 | PROPERTY / DIFFERENTIAL | Specified; validator runtime not implemented |
| I-010 | Consensus cannot depend on local wall-clock behavior. | consensus-spec.md §6; block-spec.md §3 | ADVERSARIAL / NETWORK-PARTITION | Specified target rule |
| I-011 | Consensus-critical serialization is versioned and deterministic. | block-spec.md §8; transaction-spec.md §3 | UNIT / DIFFERENTIAL / FUZZ | V2 transaction serialization implemented; production block encoding TBD |
| I-012 | Genesis uniquely determines initial network state. | state-transition-spec.md §3 | DIFFERENTIAL / LONG-RUN | Specified; no production genesis exists |
| I-013 | Protocol upgrades require explicit version/governance rules. | consensus-spec.md §13; governance-spec.md §6 | INTEGRATION / ADVERSARIAL | Specified; governance not implemented |
| I-014 | Application state changes only through consensus-authorized execution. | state-transition-spec.md §4/9 | UNIT / ADVERSARIAL | Specified; production application not implemented |
| I-015 | No account, validator, module, governance proposal, or treasury control may mint SYJ outside the specified issuance rules. | staking-spec.md §11; state-transition-spec.md §13 | PROPERTY / ADVERSARIAL / LONG-RUN | Specified; final monetary policy TBD |
| I-016 | No single administrative key may unilaterally upgrade consensus, modify supply, or finalize arbitrary state changes. | governance-spec.md §10; validator-spec.md §10 | ADVERSARIAL / INTEGRATION | Specified; key governance runtime not implemented |
| I-017 | All consensus-critical messages must be authenticated, replay-protected, and bound to the correct chain/network identity. | consensus-spec.md §11; transaction-spec.md §5 | FUZZ / ADVERSARIAL / NETWORK-PARTITION | V2 transaction/network identity implemented; production consensus messaging proposed |

## Economic/governance parameter audit

The following MUST remain TBD — requires governance and economic review:

- Minimum validator stake
- Maximum validator count
- Unbonding period
- Inflation/emission rate
- Validator reward schedule
- Delegator reward formula
- Commission bounds
- Fee burn percentage
- Governance quorum
- Governance approval threshold
- Governance veto threshold
- Proposal deposit
- Upgrade timelock duration
- Emergency halt authority
- Fee-market parameters
- Maximum production transaction/block sizes where not yet benchmarked

No allocation percentages, final supply numbers, or mainnet distribution are selected in Phase 9.

## Security and maturity checks

- [ ] Threat model covers 18 required threats.
- [ ] Each threat has asset, attacker capability, attack path, defense, detection signal, launch gate, and status.
- [ ] Launch gates distinguish specification, private prototype, local multi-validator, public testnet, and mainnet readiness.
- [ ] Future gates are not marked passed.
- [ ] Independent audit is required before mainnet candidate.
- [ ] External validators and governance rehearsal are required before mainnet candidate.
