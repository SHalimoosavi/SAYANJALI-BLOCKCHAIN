# ADR-010: PoS/BFT Production Consensus Target

**Status:** Draft
**Target protocol version:** Phase 9 / target protocol version 2.x
**Scope:** Architectural decision to target validator-based BFT finality rather than productionizing the current PoW engine.

> Phase 9 is a protocol-formalization phase. It specifies contracts and architecture; it does not implement PoS/BFT, staking, validator lifecycle, governance execution, a state tree, smart contracts, or a production network.


## Context

The repository currently contains a functioning PoW-oriented chain and a hardened V2 Go foundation. Phase 9 must establish a production consensus direction without implementing it.

## Problem

A production SYJ network requires explicit validator accountability, deterministic finality, stake-based security, evidence handling, and a well-defined validator lifecycle. The current PoW track does not provide the desired production architecture.

## Decision

**PROPOSED:** target PoS/BFT consensus compatible with CometBFT, with authenticated voting power, deterministic validator sets, proposer selection, prevote/precommit rounds, commit certificates, and finalized blocks.

The current PoW system remains:

**Legacy / transitional / protocol-research track — not the production consensus target.**

## Security assumptions

The BFT target assumes less than one-third of active voting power is Byzantine for standard safety/liveness properties. Long-range attacks require checkpoint/genesis governance and key-history controls. Stake concentration remains a systemic risk.

## Incentives

Staking, rewards, commission, and slashing are specified separately and remain TBD where economic values are concerned.

## Alternatives

- Continue PoW: preserves current implementation but does not meet the proposed validator/finality target.
- Custom BFT: full control but significantly increases protocol, audit, and maintenance risk.
- PoS/BFT with CometBFT: leverages a mature consensus runtime while leaving SYJ application semantics explicit.
- Other ecosystems: evaluated in ADR-012.

## Migration

A production migration requires:
1. frozen Phase 9 specs;
2. deterministic state migration;
3. new production genesis/network identity;
4. validator key enrollment;
5. private multi-validator prototype;
6. adversarial testing;
7. independent audit;
8. public-testnet launch gates.

## Consequences

Positive: deterministic finality model, validator accountability, explicit stake security. Negative: validator operations, slashing complexity, key custody, stake concentration, and governance risks.

## Open questions

Consensus runtime version, timeout tuning, evidence format, validator-set activation, recovery policy, emergency halt, and economic security parameters are TBD.

## Review checklist

- [ ] PoW is not called the production target.
- [ ] Custom BFT is not implemented.
- [ ] CometBFT remains proposed.
- [ ] Migration requires a new approved production boundary.
