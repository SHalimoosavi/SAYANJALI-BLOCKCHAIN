# Validator Specification

**Status:** Draft
**Target protocol version:** Phase 9 / target protocol version 2.x
**Scope:** Defines validator identity, lifecycle, key separation, voting power, and deterministic set transitions.

> Phase 9 is a protocol-formalization phase. It specifies contracts and architecture; it does not implement PoS/BFT, staking, validator lifecycle, governance execution, a state tree, smart contracts, or a production network.


## 1. Validator identity

A validator is a protocol account/state object representing an entity authorized to participate in consensus. Its identity MUST be stable across node restarts and MUST be bound to authenticated stake state.

Production validator identity fields SHOULD include:
- validator ID/address;
- consensus public key;
- node/P2P identity reference;
- staking authorization;
- commission configuration;
- lifecycle status;
- activation/deactivation heights.

Exact encodings are **TBD — requires protocol/security review**.

## 2. Key separation

Four credential domains MUST remain distinct:
1. **Account/signing key** — signs ordinary transactions.
2. **Node/P2P identity key** — authenticates network identity and peer sessions.
3. **Consensus key** — signs prevotes, precommits, and related evidence.
4. **Governance/treasury key** — authorizes governance or treasury actions.

Operational tooling MAY support secure custody systems that coordinate these keys, but protocol authorization MUST NOT collapse them into one universal administrative credential.

## 3. Registration

Registration MUST create a deterministic validator state record after validating stake authorization, identity keys, commission configuration, and required metadata. Production minimum stake is **TBD — requires governance and economic review**.

## 4. Active-set membership

Voting power MUST be derived from authenticated stake state (I-008). The active set MUST be selected deterministically from eligible validators. Maximum validator count is **TBD — requires governance, decentralization, and performance review**.

## 5. Set transitions

Validator-set transitions MUST be deterministic (I-009). Changes SHOULD activate at a well-defined height/epoch so all validators compute the same set before consensus messages for the new set are accepted.

## 6. Lifecycle

```text
Candidate -> Active -> Jailed -> Unjailed -> Active
                    \-> Tombstoned -> Removed
Active -> Unbonding -> Removed
Candidate -> Rejected
```

The exact transition triggers and delays are **TBD**.

## 7. Jail and tombstone

Jailing is a temporary restriction triggered by defined faults such as downtime. Tombstoning is a stronger, normally irreversible consensus-state penalty for severe behavior such as cryptographic equivocation. Conditions, evidence thresholds, and recovery are **TBD — requires security/economic review**.

## 8. Key rotation

Consensus-key rotation MUST be explicitly authorized and activated at a deterministic height/epoch. The old key MUST remain recognized for evidence covering the prior validity window. Emergency rotation procedures MUST avoid enabling unilateral arbitrary finalization.

## 9. Anti-centralization requirements

The production system SHOULD expose validator concentration metrics, stake concentration warnings, geographic/ASN diversity metrics where ethically and legally appropriate, independent-operator targets, and delegation concentration analysis. These are launch controls, not proof of decentralization.

## 10. Governance and consensus boundaries

Governance MAY authorize parameter changes according to the governance specification, but a governance operator MUST NOT directly sign consensus commits. Consensus validators MUST NOT unilaterally change governance state outside authorized proposals.

## 11. Launch gates

Before a public testnet:
- separate key classes MUST be operational;
- validator registration and set transition tests MUST pass;
- evidence handling MUST be tested;
- secure P2P identity and rate limiting MUST exist;
- independent operators MUST participate.

Before mainnet candidate:
- independent security review;
- key compromise drills;
- validator diversity evidence;
- slashing/evidence simulations;
- deterministic recovery tests.

## 12. Invariants

I-008, I-009, I-016, I-017.

## 13. Failure modes

Invalid stake proof, duplicate identity, stale consensus key, conflicting set transition, compromised key, equivocation, downtime, corrupted validator state, and unsafe emergency removal.

## 14. Required simulations

Stake attacker scenarios at 10%, 25%, 33%, 50%, and 2/3; downtime; censorship; equivocation; cartel coordination; key rotation; mass validator outage; and recovery after partition.

## 15. Open questions

Minimum stake; maximum set size; activation epoch; commission bounds; downtime thresholds; jail duration; tombstone rules; emergency removal; geographic concentration limits are TBD.

## 16. Review checklist

- [ ] Four key classes are separated.
- [ ] Voting power comes from authenticated stake.
- [ ] Set changes are deterministic.
- [ ] Numerical validator parameters remain TBD.
