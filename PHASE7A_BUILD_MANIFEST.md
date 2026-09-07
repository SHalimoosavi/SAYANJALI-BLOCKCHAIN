# SYJ Phase 7A Build Manifest

## Baseline

- Phase 7 address-independent baseline: `1e44046`
- Dedicated implementation target: `phase-7a-genesis-state-integration`
- No production allocation addresses are included.
- No production Phase 7 commitment is included.
- Frozen historical block genesis remains authoritative for block 0.

## Implemented

- Versioned `internal/tokenomics.GenesisState`.
- Deterministic canonical-JSON commitment with category-order normalization.
- Address validation delegated to the existing SYJ wallet address validator.
- `chain.OpenWithGenesisState` with deterministic 288M initial balances.
- Explicit `genesisSupply`, `miningIssued`, and `supply` state accounting.
- Phase 7 mining reward calculation against the dedicated 432M mining allocation.
- Miner uses the same Phase 7 consensus reward calculation as validation.
- Restart/replay and reorg rebuilds begin from the same immutable Phase 7 baseline.
- Genesis allocation is explicitly rejected if it appears as a normal block transaction.
- Phase 7 network name `sayanjali-syj-phase7-v1`.
- Node fail-closed configuration requiring a genesis-state file and matching commitment.
- Non-Phase-7 `chain.Open` path remains unchanged in economic behavior.
- Phase 7 chain/node/P2P-path regression tests and deterministic commitment tests.

## Intentionally not implemented

- Production address binding.
- Production commitment/hash publication.
- New transaction types.
- Nonce.
- Halving.
- On-chain vesting.
- Consensus multisig.
- P2P wire changes.
- Historical genesis changes.
