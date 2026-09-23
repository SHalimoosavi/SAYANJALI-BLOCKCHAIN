# Block Specification

**Status:** Draft
**Target protocol version:** Phase 9 / target protocol version 2.x
**Scope:** Defines the legacy block boundary and the proposed production finalized-block contract.

> Phase 9 is a protocol-formalization phase. It specifies contracts and architecture; it does not implement PoS/BFT, staking, validator lifecycle, governance execution, a state tree, smart contracts, or a production network.


## 1. Current versus proposed

**CURRENT LEGACY MODEL:** the repository currently validates a PoW-oriented block chain with historical header fields, transaction integrity, difficulty, proof-of-work, and accumulated-work chain selection.

**PROPOSED PRODUCTION MODEL:** finalized blocks are proposed under CometBFT-compatible BFT consensus and carry authenticated transaction, state, receipt/event, and consensus evidence commitments.

The current PoW model is **Legacy / transitional / protocol-research track — not the production consensus target.**

## 2. Proposed production block structure

```text
version
height
chain_id / network_id
parent_hash
timestamp
proposer_validator_id
transaction_root
state_root
receipt_event_root
consensus_evidence
commit_certificate
application_metadata
```

Exact binary/JSON encoding is versioned and **TBD — requires protocol/security review**.

## 3. Timestamp

Block timestamp MUST be deterministic from consensus evidence rather than trusted local wall-clock time. Validators MAY use local clocks for networking and operational deadlines, but application validity MUST be based on consensus-defined time bounds and evidence.

## 4. Transaction root

The transaction root MUST commit to the ordered transaction identities or canonical transaction leaves defined by the finalized block version. The production tree algorithm, empty-root representation, and proof encoding are **TBD — requires protocol/security review**. Current V2 uses transaction identity for Merkle leaves; this behavior is documented as a compatibility fact rather than a final state-root design.

## 5. State and receipt roots

`state_root` MUST commit to the complete consensus-relevant application state after block execution. `receipt_event_root` MUST commit to deterministic execution receipts/events if that feature is retained. Neither root is implemented by the current repository.

## 6. Consensus evidence

The block MUST carry or be reconstructible with evidence sufficient to establish that the consensus engine finalized the block under the active validator set. A CometBFT commit certificate is the proposed model. The exact evidence schema and cryptographic encoding are **TBD — requires runtime integration review**.

## 7. Block size

A maximum block size MUST be enforced before execution. The numerical limit is **TBD — requires performance, security, and governance review**. The limit MUST account for consensus messages, transaction execution cost, receipt/event volume, and propagation time.

## 8. Canonical serialization and block hash

Consensus-critical serialization MUST be deterministic and versioned. The production block identifier MUST be derived from a canonical header/commitment representation that excludes mutable transport metadata. The exact production hash construction is **TBD — requires protocol and runtime review**.

## 9. Invalid block conditions

Reject a block for unsupported version; wrong chain identity; incorrect height; incorrect parent; invalid timestamp rules; malformed proposer identity; invalid transaction root; invalid state root; invalid receipt/event root; invalid consensus evidence; insufficient commit voting power; invalid validator-set transition; oversized encoding; duplicate/conflicting transactions; invalid state transition; or hash mismatch.

## 10. State-transition pseudocode

```text
require block.version == activeVersion
require block.height == parent.height + 1
require block.parent_hash == parent.hash
validateConsensusEvidence(block)
validateTimestamp(block, consensusTime)
validateTransactions(block)
S_next, receipts = Apply(S_parent, block.transactions)
require block.state_root == Root(S_next)
require block.receipt_event_root == Root(receipts)
require block.transaction_root == Root(block.transactions)
finalize(block)
```

## 11. Invariants

I-006, I-007, I-010, I-011, I-012, I-014, and I-017 apply directly.

## 12. Security assumptions

The production design assumes authenticated validators, deterministic execution, Byzantine-aware finality, secure consensus keys, bounded blocks, and correct state-root construction.

## 13. Failure modes

A state-root mismatch, invalid commit certificate, parent mismatch, invalid proposer, malformed transaction root, or nondeterministic application result MUST prevent finalization. A validator experiencing a local clock anomaly MUST NOT cause a valid block to become locally invalid solely because of that clock.

## 14. Compatibility and migration

Current PoW blocks remain historical/reference data. Migration to a production block model requires a new versioned genesis or explicit migration boundary. No automatic reinterpretation of historical PoW headers as BFT commit certificates is permitted.

## 15. Required vectors

Block header serialization, transaction root, state root, receipt root, parent linkage, validator-set transition, commit certificate, invalid evidence, timestamp boundaries, and block-size boundaries.

## 16. Open questions

Exact root algorithms, proof formats, block-size limit, hash domain, evidence encoding, metadata fields, and state snapshot format are **TBD — requires protocol/security/runtime review**.

## 17. Review checklist

- [ ] Legacy PoW is clearly separated from production target.
- [ ] State root is specified but not claimed implemented.
- [ ] Commit certificate is proposed, not shipped.
- [ ] No numerical production block limit is silently chosen.
