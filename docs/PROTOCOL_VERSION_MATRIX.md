# Protocol version feature matrix

Baseline: `9502979638e48a43519360a3215eca2cb84e6ce9`

This matrix distinguishes implemented behavior from design documents and deferred work. A version label does not imply that every feature described by a later design is active.

| Capability | V1 | V2 | V3 |
|---|---|---|---|
| Go chain constructor | `chain.Open` / legacy path | `OpenV2WithGenesisState` | `OpenV3WithGenesisState` |
| Transaction envelope | Legacy transaction model | V2 transaction model | Reuses V2 economic/state model in current constructor |
| Network identity bound into signing payload | No V2-style network ID in V1 payload | Yes, explicit NetworkID | Yes, inherited V2 model |
| Account nonce | No V2 nonce semantics | Yes | Yes, inherited V2 model |
| Canonical transaction serialization | Historical compatibility path | V2 canonical signing payload | No separate V3 transaction wire/signing format activated by current V3 constructor |
| Low-S ECDSA | Historical compatibility path | Enforced in shared Go secp256k1 verifier | Inherited |
| PoW timestamp rules | Legacy parent-relative time policy | V2 parent-relative policy | C-1 V3 MTP lower bound and ingress future-time bound |
| V3 MTP + 1 lower bound | No | No | Implemented |
| V3 ingress future bound | No | No | Implemented |
| Historical replay independent of wall clock | Historical validation path | Historical validation path | Explicit C-1 replay tests |
| V3 solo/bootstrap mining | No | No | Operator-controlled, disabled by default |
| State commitment in block header | No | No | Not activated |
| State root/checkpoint sidecar | Not applicable to legacy state | Incremental state/checkpoint implementation | Inherits V2 state architecture |
| Merkle V3 activation | No | No | Not activated by `OpenV3WithGenesisState`; deferred by V3 decisions |
| Fee field / fee market | No | No production fee field in V2 | No V3 fee activation |
| Deterministic BFT finality | No | No | No; current PoW chain selection remains accumulated-work based |

## Implemented

- V1: historical compatibility and legacy chain validation.
- V2: explicit network identity, nonce-bearing transaction model and V2 mempool/state validation.
- V3: C-1 time semantics, V3 replay clock-independence tests and opt-in solo/bootstrap mining controls.

## Deferred or absent

- C-2 state architecture activation and authenticated sparse Merkle state.
- V3 Merkle activation.
- Production transaction fees and fee market.
- Deterministic BFT finality.
- A complete wallet seed/restore lifecycle.
- Formal V1 retirement/activation boundary.

## Compatibility warning

V1, V2 and V3 are not interchangeable labels. Operators must use the exact network configuration and protocol version expected by the chain they are joining. Do not infer mainnet readiness or feature activation from the presence of a constructor or a design specification.
