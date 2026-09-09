# SYJ Phase 7A Genesis-State Integration

Phase 7 economic genesis is an immutable initial-state overlay on top of the frozen historical block genesis.

- Historical `block.Genesis()` and its hash remain unchanged.
- Phase 7 genesis state is not a block and is not a transaction.
- Phase 7 initial supply is exactly `28,800,000,000,000,000` base units.
- Mining issuance is separately capped at `43,200,000,000,000,000` base units.
- Absolute maximum remains `72,000,000,000,000,000` base units.
- The normal mining reward remains exactly `5,000,000,000` base units (50 SYJ).
- A partial final mining reward is not permitted; the 432M mining allocation divides exactly by the 50 SYJ reward.

## Network configuration

The reserved Phase 7 network name is:

`sayanjali-syj-phase7-v1`


A Phase 7 node must provide both:

- a genesis-state file path; and
- the exact commitment calculated from that file.

The node fails closed when either is missing or the commitment does not match. No placeholder commitment is embedded as a production default.

## Commitment

The commitment is SHA-256 over repository canonical-JSON bytes. Allocation entries are sorted by category before hashing, so input array ordering cannot change the commitment. This is a commitment to the state representation only; it does not change the frozen block serialization or P2P wire grammar.

The final production commitment must not be created until the five intentional production addresses are supplied and independently reviewed.
