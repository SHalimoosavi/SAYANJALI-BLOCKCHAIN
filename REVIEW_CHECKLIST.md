# Phase 9.2 Review Checklist

**Baseline:** `75571c2a6fcbdbbf506d703ea379a3d9b8a4b2d8`
**Phase 9 specification commit:** `aef1a4944674e77c8745d5a20bdccf8a5d7ca11a`
**Scope:** documentation and schemas only
**GitHub writes:** none

## 1. Deliverable integrity

- [ ] No GitHub-write claims.
- [ ] All Phase 9.2 package files are present.
- [ ] JSON schemas parse successfully.
- [ ] All schemas validate against the JSON Schema Draft 2020-12 metaschema.
- [ ] Cross-document consistency is reviewed.
- [ ] No runtime implementation is introduced.
- [ ] No mainnet genesis is created.
- [ ] No token distribution is created.
- [ ] No smart-contract runtime is introduced.
- [ ] No PoS/staking/validator/governance execution code is introduced.
- [ ] No secrets/private keys/wallet artifacts/.env files/binaries/generated artifacts are included.
- [ ] SYJ remains **NOT PRODUCTION-READY** and **NOT MAINNET-READY**.
- [ ] PoW is labeled **Legacy / transitional / protocol-research track — not the production consensus target.**
- [ ] Cosmos SDK + CometBFT is labeled **PROPOSED / NOT SHIPPED / NOT IMPLEMENTED**.
- [ ] Decentralization is not claimed as demonstrated.

## 2. Phase 9.1 condition closure

- [ ] Condition 1: every I-001 through I-017 reference points to the actual normative section.
- [ ] Condition 2: `network_id` is canonical; Cosmos/CometBFT `chain-id` is a deterministic compatibility mapping only.
- [ ] Condition 3: `genesis-spec.md` covers every top-level field of `genesis-v2.json` and the required semantic validation contract.
- [ ] Condition 4: `phase10-prototype-contract.md` freezes the Phase 10 implementation boundary.

## 3. Required invariant traceability

| ID | Normative invariant | Specification reference | Test class | Status |
|---|---|---|---|---|
| I-001 | Every valid transaction has exactly one protocol version. | `transaction-spec.md` §5 | UNIT / PROPERTY | Specified; current V2 enforces version=2 |
| I-002 | Every transaction belongs to exactly one network identity. | `transaction-spec.md` §5; `ADR-013-network-identifier.md` §6 | UNIT / ADVERSARIAL | Specified; current V2 validates exact network ID |
| I-003 | Every normal account has exactly one next nonce. | `state-transition-spec.md` §5 | PROPERTY / LONG-RUN | Specified; current V2 tracks nonce state |
| I-004 | A transaction cannot consume a nonce twice. | `transaction-spec.md` §7; `state-transition-spec.md` §5 | UNIT / FUZZ / ADVERSARIAL | Specified; current V2 replay/nonce paths enforce |
| I-005 | Total supply cannot exceed the monetary-policy ceiling. | `state-transition-spec.md` §13; `staking-spec.md` §11 | PROPERTY / LONG-RUN | Specified; production issuance policy TBD |
| I-006 | State transition is deterministic. | `state-transition-spec.md` §4 | PROPERTY / DIFFERENTIAL / FUZZ | Specified; production execution not implemented |
| I-007 | Finalized blocks cannot be reverted under normal protocol rules. | `consensus-spec.md` §9 | INTEGRATION / NETWORK-PARTITION | Proposed BFT rule; not current PoW finality |
| I-008 | Validator voting power is derived from authenticated stake state. | `validator-spec.md` §4; `staking-spec.md` §3 | PROPERTY / ADVERSARIAL | Specified; staking not implemented |
| I-009 | Validator-set transitions are deterministic. | `validator-spec.md` §5; `consensus-spec.md` §8 | PROPERTY / DIFFERENTIAL | Specified; validator runtime not implemented |
| I-010 | Consensus cannot depend on local wall-clock behavior. | `consensus-spec.md` §6; `block-spec.md` §3 | ADVERSARIAL / NETWORK-PARTITION | Specified target rule |
| I-011 | Consensus-critical serialization is versioned and deterministic. | `block-spec.md` §8; `transaction-spec.md` §3 | UNIT / DIFFERENTIAL / FUZZ | V2 transaction serialization implemented; production block encoding TBD |
| I-012 | Genesis uniquely determines initial network state. | `genesis-spec.md` §§3–9; `state-transition-spec.md` §3 | DIFFERENTIAL / LONG-RUN | Specified; no production genesis exists |
| I-013 | Protocol upgrades require explicit version/governance rules. | `governance-spec.md` §6; `consensus-spec.md` §13 | INTEGRATION / ADVERSARIAL | Specified; governance not implemented |
| I-014 | Application state changes only through consensus-authorized execution. | `state-transition-spec.md` §§1,4,9 | UNIT / ADVERSARIAL | Specified; production application not implemented |
| I-015 | No account, validator, module, governance proposal, or treasury control may mint SYJ outside the specified issuance rules. | `staking-spec.md` §11; `state-transition-spec.md` §13 | PROPERTY / ADVERSARIAL / LONG-RUN | Specified; final monetary policy TBD |
| I-016 | No single administrative key may unilaterally upgrade consensus, modify supply, or finalize arbitrary state changes. | `governance-spec.md` §10; `validator-spec.md` §10 | ADVERSARIAL / INTEGRATION | Specified; key governance runtime not implemented |
| I-017 | All consensus-critical messages must be authenticated, replay-protected, and bound to the correct `network_id`. | `consensus-spec.md` §11; `transaction-spec.md` §5; `ADR-013-network-identifier.md` §6 | FUZZ / ADVERSARIAL / NETWORK-PARTITION | V2 transaction/network identity implemented; production consensus messaging proposed |

## 4. Genesis coverage

Every required top-level field MUST have a normative field-level section in `docs/spec/genesis-spec.md` and a matching schema property:

| Schema field | Genesis spec section | Required |
|---|---|---|
| `protocol_version` | §6.1 | MUST |
| `network_name` | §6.2 | MUST |
| `network_id` | §6.3 | MUST |
| `historical_genesis_hash` | §6.4 | MUST |
| `genesis_state_commitment` | §6.5 | MUST |
| `accounts` | §6.6 | MUST |
| `validators` | §6.7 | MUST |
| `staking` | §6.8 | MUST |
| `governance` | §6.9 | MUST |
| `monetary_policy` | §6.10 | MUST |

Required negative vectors G-NEG-001 through G-NEG-017 are specified in `genesis-spec.md` §11.

## 5. Canonical network identifier checks

- [ ] `network_id` is the only canonical SYJ protocol identity.
- [ ] Transaction, block, genesis, P2P, and consensus contracts bind to `network_id`.
- [ ] Cosmos/CometBFT `chain-id` is derived from `network_id` under ADR-013.
- [ ] The mapping is deterministic and reversible.
- [ ] Legacy V1 numeric `chain_id` is classified as historical compatibility material, not a Phase 9 identity.
- [ ] Existing V2 `network_id` vectors are not silently changed.

## 6. Economic/governance parameter audit

The following MUST remain **TBD — requires governance, economic, security, or legal review** unless separately approved by a later protocol decision:

- minimum validator stake;
- maximum validator count;
- unbonding period;
- inflation/emission rate;
- validator/delegator rewards;
- commission bounds;
- slashing/downtime parameters;
- fee burn percentage;
- minimum gas price;
- block gas limit;
- governance quorum;
- approval threshold;
- veto threshold;
- proposal deposit/refund;
- upgrade timelock;
- emergency halt authority;
- treasury controls;
- genesis allocation/vesting/distribution;
- final SYJ supply/issuance policy.

No Phase 9.2 document selects a final production economic or distribution value.

## 7. Phase 10 boundary

- [ ] Scope is a CometBFT + Cosmos SDK prototype on a local 4–7 validator network.
- [ ] `prototype/cosmos/` is the intended implementation boundary.
- [ ] Key separation is explicit.
- [ ] Deterministic genesis/network identity is required.
- [ ] Required unit/integration/property/fuzz/differential/chaos/partition tests are specified.
- [ ] Observability minimums are specified.
- [ ] Security-review gates are specified.
- [ ] Public network, mainnet, token sale/distribution, smart contracts, production treasury/validators, incentivized testnet, and final economic/governance values are forbidden.
- [ ] Phase 10 is explicitly a prototype, not a launch.

## 8. Forbidden-content audit

- [ ] No runtime implementation source added by Phase 9.2.
- [ ] No PoS/staking/validator/slashing/governance execution code added.
- [ ] No smart-contract runtime added.
- [ ] No mainnet genesis added.
- [ ] No token distribution added.
- [ ] No secrets/private keys/wallet files/.env files added.
- [ ] No binaries or generated build artifacts added.

## 9. Security and maturity

- [ ] Threat model covers all 18 required threats.
- [ ] Launch gates distinguish specification, private prototype, local multi-validator, public testnet, and mainnet readiness.
- [ ] Future gates are not marked passed.
- [ ] Independent security review remains required before any production candidate.
- [ ] External validators and governance rehearsal remain required before mainnet consideration.

## 10. Human approval

- [ ] Protocol owner review complete.
- [ ] Security review complete or explicitly deferred to the next gate.
- [ ] Economic/governance/legal review requirements recorded.
- [ ] Phase 10 entry authorized separately.
