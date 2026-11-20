# Decision note: deterministic confirmation policy

**Status:** OWNER DECISION REQUIRED. No consensus change is authorized by this note.

## Current baseline

The current chain is PoW and selects the valid chain with greater accumulated work. It does not provide deterministic BFT finality. A reorganization can remove previously active blocks.

## Options

### Option A — fixed confirmation count

Applications treat a transaction as settled after a configured number of descendant blocks.

- Advantages: easy to explain and implement.
- Trade-offs: a fixed count does not adapt to observed hash rate, difficulty, network partitions or attacker resources.

### Option B — work-based probabilistic policy

Applications use accumulated work after inclusion and a documented risk model.

- Advantages: reflects the PoW security model more directly.
- Trade-offs: requires defensible assumptions and clear user-facing risk communication; no deterministic finality is created.

### Option C — future BFT finality layer

A separately designed validator/finality protocol finalizes blocks under explicit quorum assumptions.

- Advantages: can provide deterministic finality within stated fault assumptions.
- Trade-offs: fundamentally changes consensus, validator requirements, networking and governance. It is not a maintenance change.

## Recommendation

For the current PoW network, document confirmations as probabilistic and define an application-level work/confirmation policy only after testnet data is available. Do not claim irreversible finality. Treat BFT finality as a separate consensus design requiring owner approval.
