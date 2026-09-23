# V2 Replay Protection and Reorganization Rules

## Identity boundary

`tx_id` is the consensus transaction identity. It is independent of the signature and includes all fields in the V2 signing payload, including version, network ID, nonce, sender, receiver, amount, timestamp, and sender public key.

## Duplicate rejection

Consensus rejects:

- duplicate `tx_id` in one block;
- `tx_id` already confirmed on the active chain;
- duplicate IDs discovered during chain replay;
- duplicate IDs introduced through a candidate reorganization.

Validation uses an authoritative replay pass; the confirmed-ID map is an acceleration/index structure, not the sole security mechanism.

## Nonce rules

Each normal sender starts with `next_nonce = 0`.

A transaction must use exactly `sender.next_nonce`. A successful application increments the state by one. Gaps, reuse, and stale nonces are rejected. If `next_nonce == MaxUint64`, a further successful normal transaction cannot be represented because the required increment would overflow; the transaction is rejected fail-closed.

## Reorganization

The active-chain state is reconstructed from the common ancestor and the winning branch. The reconstruction includes balances, nonces, and confirmed transaction IDs. Losing-fork transactions are eligible for re-admission only if they remain valid against the winning state, including balance, nonce, signature, network identity, version, and duplicate-ID rules.

## Cross-network and cross-protocol replay

The network ID and protocol version are signed. A transaction signed for one V2 network cannot validate on another network, and a V1 transaction cannot be interpreted as a V2 transaction.
