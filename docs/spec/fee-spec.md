# Fee Specification

**Status:** Draft
**Target protocol version:** Phase 9 / target protocol version 2.x
**Scope:** Defines the proposed transaction-fee model and anti-spam controls without finalizing economic parameters.

> Phase 9 is a protocol-formalization phase. It specifies contracts and architecture; it does not implement PoS/BFT, staking, validator lifecycle, governance execution, a state tree, smart contracts, or a production network.


## 1. Purpose

Fees allocate scarce execution and block space, discourage spam, and provide a deterministic accounting boundary between users, validators, and any approved burn/treasury mechanism.

## 2. Fee model

A production transaction SHOULD carry an explicit fee object containing an amount and, where execution requires it, a gas/resource limit. The exact fields are **TBD — requires protocol/security review**.

Minimum gas price is:

**TBD — requires governance and economic review.**

## 3. Admission and priority

Mempool admission SHOULD:
1. parse and authenticate the transaction;
2. verify network identity and nonce;
3. estimate required resources;
4. verify fee affordability;
5. reject malformed or expired transactions;
6. rank valid transactions deterministically by fee/resource policy.

The mempool MUST NOT be consensus state. Block proposers MAY choose among valid transactions subject to deterministic validity and resource limits.

## 4. Eviction

Nodes SHOULD evict stale, expired, underpriced, conflicting, or resource-exhausting transactions according to bounded local policy. Consensus MUST NOT depend on the exact contents of any node's mempool.

## 5. Fee market

A production fee market MAY use a base-fee plus priority mechanism. Any formula, adjustment speed, target utilization, and minimum fee are **TBD — requires governance/economic review**.

## 6. Fee burn

Fee burn is not selected as final. If burn is adopted, the burned amount MUST be removed from spendable supply deterministically and MUST be represented in issuance/supply accounting.

**Fee burn percentage: TBD — requires governance and economic review.**

## 7. Anti-spam and state bloat

Controls SHOULD include transaction size limits, gas/resource limits, bounded event output, per-account nonce ordering, mempool limits, peer rate limits, and minimum fee policy. Exact limits are TBD.

## 8. Example only

> **EXAMPLE ONLY — NOT A FINAL PROTOCOL PARAMETER.**
>
> If a future governance-approved policy charged 10 units for a transaction and burned 20%, 2 units would be removed from supply and 8 units would follow the approved destination. This example does not select any SYJ economic parameter.

## 9. State-transition pseudocode

```text
admit(tx):
    validateEnvelope(tx)
    require fee(tx) >= activeMinimum
    require resources(tx) <= activeLimits
    placeInMempool(tx)

execute(tx):
    reserve/charge fee according to active rules
    execute deterministic state transition
    account for burn/reward/treasury destination
```

## 10. Invariants

I-005, I-006, I-014, and I-015 apply.

## 11. Security assumptions and failure modes

Fee manipulation, mempool flooding, fee griefing, state bloat, underpriced computation, and fee-accounting bugs are primary risks. A fee schedule that permits unbounded computation without proportional cost is unsafe.

## 12. Compatibility and migration

The current V2 transaction does not contain a production fee field. Introducing fees requires a versioned production transaction contract or explicitly approved extension.

## 13. Required vectors

Fee minimum; insufficient fee; fee overflow; gas/resource limit; failed transaction; burn accounting; proposer priority; mempool eviction; expired transaction; and state-bloat limits.

## 14. Open questions

Fee fields, gas model, minimum gas price, fee-market algorithm, priority semantics, burn percentage, reward destination, treasury share, and parameter-change cadence are TBD.

## 15. Review checklist

- [ ] No final fee value.
- [ ] No final burn percentage.
- [ ] Current V2 absence of fees is documented.
- [ ] Mempool policy is separated from consensus validity.
