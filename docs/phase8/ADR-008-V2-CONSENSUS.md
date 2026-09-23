# ADR-008 — V2 Consensus Foundation

## Decision

Introduce SYJ Protocol V2 as a separate consensus track. Preserve Protocol V1 as a frozen compatibility track.

## 1. V1 frozen

Historical V1 transaction serialization/hashing/signing, historical genesis, Merkle vectors, difficulty/work vectors, address vectors, and P2P wire `sayanjali-p2p / 1.0` remain unchanged.

## 2. V2 introduced

V2 adds explicit transaction versioning, network identity, sender nonce state, signature-independent transaction IDs, and domain-separated cryptographic operations.

## 3. Version separation

Implicit coexistence is unsafe because V1 does not cryptographically bind protocol version, network identity, or nonce. V2 therefore rejects V1 transactions and V1 nodes cannot interpret V2 transaction objects as V1 transactions.

## 4. Network identity

The EffectiveNetworkID binds protocol version, GenesisState commitment, historical genesis hash, and network name. The audited private-testnet identity is `237a...aaf3` and its P2P representation is `syjnet-v2-237a...aaf3`.

## 5. GenesisState binding

V2 startup validates the GenesisState commitment and independently derives the network identity. It never silently regenerates or repairs consensus identity.

## 6. Transaction version

V2 transactions carry `version: 2`. Version is included in the signed payload.

## 7. Nonce

Each normal account begins at nonce `0`. Consensus requires exact equality with `next_nonce` and increments the state only after successful application.

## 8. Replay protection

Network ID, version, and nonce are cryptographically bound. `tx_id` is the authoritative transaction identity and duplicate IDs are rejected during block validation and full replay.

## 9. Signature-independent tx_id

`tx_id = SHA256("SYJ-TX-ID-V2\\0" || canonical_signing_payload)` and does not include the signature.

## 10. P2P compatibility

The application wire remains version 1.0. V2 network identity is carried by the existing authenticated HELLO `NetworkName` field, avoiding a wire-format change.

## 11. V1/V2 incompatibility

V1 and V2 are distinct consensus tracks. A node must not silently translate or migrate one into the other.

## 12. Reorg semantics

Winning-chain state is reconstructed from the common ancestor and winning branch. Balances, nonces, and confirmed IDs are rebuilt from consensus replay.

## 13. Mempool semantics

V2 mempool admission requires contiguous sender nonces, no gaps, no replacement, no fees, bounded capacity, duplicate-ID rejection, and duplicate sender+nonce rejection. Persistence is not required in this phase.

## 14. Coinbase

Coinbase is a separate special V2 object. It is unsigned, has no normal account nonce consumption, and has a deterministic V2 transaction ID.

## 15. Compatibility rules

The V1 fixtures are never regenerated. V2 vectors live under `protocol/test-vectors/v2/`.

## 16. Rollback rules

If V2 validation reveals a protocol discrepancy, do not weaken validation or mutate V1 vectors. Revert the Phase 8.1 implementation as a coherent local diff and re-derive vectors from the corrected specification.

## 17. Security assumptions

The implementation relies on the existing audited secp256k1 dependency for ECDSA, SHA-256, deterministic canonical JSON, authenticated P2P identity, and deterministic PoW. This ADR is not an independent cryptographic audit.

## 18. Future unresolved items

Public/mainnet GenesisState governance, validator operations, fees, transaction replacement policy, long-term state/index persistence, formal property testing at larger scale, external cryptographic review, and public-network operational controls remain future work.
