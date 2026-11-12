# Decision note: transaction fees

**Status:** OWNER DECISION REQUIRED. No implementation is authorized by this note.

## Current baseline

The current V2 transaction format has no production fee field. The fee specification in `docs/spec/fee-spec.md` is draft material. The mempool does not implement a fee market.

## Options

### Option A — no fees

Transactions remain fee-free, with admission controlled by fixed local limits and protocol size/resource limits.

- Advantages: simplest transaction and monetary accounting; no fee distribution ambiguity.
- Trade-offs: weak economic spam pricing; block-space allocation lacks a fee signal; node operators bear resource costs.

### Option B — fixed protocol fee

Each transaction pays a fixed base-unit amount.

- Advantages: simpler than a dynamic market; predictable accounting.
- Trade-offs: fixed fees do not respond to demand; fee changes require carefully governed parameter activation; fee destination remains a decision.

### Option C — base fee plus priority fee

A deterministic base fee and optional priority component are charged.

- Advantages: congestion pricing and a proposer incentive signal.
- Trade-offs: more complex transaction format, signing, mempool policy, block validation, accounting and cross-version migration.

## Fee destination choices

- Pay all fees to the block producer.
- Burn all fees.
- Split between producer, burn and an explicitly governed treasury.

Each choice changes supply accounting and requires an explicit protocol decision.

## Recommendation

Do not add fees until the owner selects the fee model, fee recipient policy, minimum/maximum constraints, overflow rules, failed-transaction behavior and activation/migration plan. Preserve the current V2 format pending approval.
