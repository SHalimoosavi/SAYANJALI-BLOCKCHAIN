# Transaction Specification

**Status:** Draft
**Target protocol version:** Phase 9 / target protocol version 2.x
**Scope:** Formal transaction contract for the existing V2 compatibility boundary and the proposed production extension.

> Phase 9 is a protocol-formalization phase. It specifies contracts and architecture; it does not implement PoS/BFT, staking, validator lifecycle, governance execution, a state tree, smart contracts, or a production network.


## 1. Scope and terminology

**CURRENT V2** means the transaction implemented in the Phase 8.1 Go foundation at the authoritative baseline. **PROPOSED PRODUCTION** means the Phase 9 target contract and is not implemented.

## 2. Current V2 data structure

```text
version             uint8 = 2
network_id          64 lowercase hexadecimal characters
sender              SYJ address
receiver            SYJ address
amount_base_units   positive uint64
nonce               uint64
timestamp           finite non-negative numeric value
sender_public_key   128 lowercase hexadecimal characters
signature           128 lowercase hexadecimal characters
tx_id               64 lowercase hexadecimal characters
```

The current implementation also permits a coinbase-specific V2 transaction path. Coinbase is not a normal account transaction and does not consume an account nonce.

## 3. Canonical serialization

Current V2 signing payload keys are exactly:

`amount_base_units`, `network_id`, `nonce`, `receiver`, `sender`, `sender_public_key`, `timestamp`, `version`.

The repository canonical-JSON implementation determines key ordering and encoding. Signing bytes are:

`ASCII("SYJ-TX-SIGN-V2\0") || canonical_payload_UTF8`.

The transaction ID is:

`SHA256(ASCII("SYJ-TX-ID-V2\0") || canonical_payload_UTF8)`.

The signature is excluded from `tx_id`, while the public key remains included. This makes transaction identity stable when the same canonical payload is re-signed.

## 4. Proposed production extensions

The production protocol SHOULD add a versioned fee object and explicit expiry/deadline semantics rather than overloading the frozen V2 object. Suggested conceptual fields are:

```text
fee.amount
fee.gas_limit
expiry.height and/or expiry.time
```

Exact representation is **TBD — requires protocol/security review**. It MUST NOT be inserted into existing V2 vectors without a versioned compatibility decision.

Maximum transaction size is **TBD — requires protocol/security review**. A node MUST reject an encoded transaction exceeding the active consensus limit before expensive cryptographic or state processing.

## 5. Validation rules

A normal current V2 transaction MUST:
1. have version exactly 2;
2. carry the protocol's exact 64-hex `network_id`;
3. have a positive amount no greater than the repository's maximum-supply constant;
4. have a finite non-negative timestamp within the current implementation's uint64-compatible numeric range;
5. use valid, distinct sender and receiver addresses;
6. bind sender to the supplied public key;
7. contain valid 128-hex public-key and signature encodings;
8. verify the secp256k1 signature over the canonical V2 signing bytes;
9. have a `tx_id` equal to independent recomputation;
10. pass account-state nonce, balance, and confirmed-replay checks before inclusion.

The exact transaction-level amount maximum currently follows the repository maximum-supply constant; it is not a separate economic limit.

## 6. Proposed expiry and fee validation

Production validators MUST define deterministic expiry behavior. A transaction that is expired MUST be rejected from execution; whether expired transactions are retained for audit is an operational policy, not a state transition. Fee payment MUST be atomic with transaction execution and MUST NOT allow a failed transaction to create unbounded state changes.

Fee amount, gas schedule, maximum size, and minimum gas price are **TBD — requires governance and economic review**.

## 7. Rejection cases

Reject malformed encoding; unsupported version; wrong network identity; zero/overflow amount; invalid address; sender=receiver; public-key mismatch; invalid signature; malformed signature encoding; incorrect tx ID; duplicate identity; confirmed replay; nonce reuse; insufficient balance; invalid fee; expired transaction; excessive size; or any state-transition rule violation.

## 8. State-transition pseudocode

```text
validateEnvelope(tx)
validateNetwork(tx.network_id)
validateSignatureAndIdentity(tx)
validateExpiry(tx)
expected = state.nextNonce(tx.sender)
require tx.nonce == expected
require state.balance(tx.sender) >= tx.amount + fee(tx)
receipt = execute(tx)
state.nonce(tx.sender) = expected + 1
applyBalanceChanges()
chargeFee()
emitReceiptAndEvents()
commit deterministic state changes
```

For a failed executable transaction, the protocol MUST specify which effects are reverted and which fee/receipt effects remain. Final semantics are **TBD — requires protocol review**, but they MUST be deterministic.

## 9. Invariants

- **I-001:** one protocol version per valid transaction.
- **I-002:** one network identity per transaction.
- **I-004:** a nonce cannot be consumed twice.
- **I-011:** canonical transaction serialization is versioned and deterministic.
- **I-017:** authenticated messages are network-bound and replay-protected.

## 10. Security assumptions and failure modes

Security depends on correct key custody, canonical serialization, domain separation, nonce state, `network_id`, and deterministic validation. Key compromise, network-ID confusion, timestamp abuse, signature malleability, and oversized payloads are explicit threat surfaces.

## 11. Compatibility and migration

Existing V2 vectors MUST remain reproducible. Production fee/expiry fields require a new transaction schema/version or a separately approved extension mechanism. A production migration MUST provide a deterministic mapping from any carried-forward account state to the new state machine.

## 12. Required vectors

- canonical payload;
- signing bytes and digest;
- signature verification;
- signature-independent tx ID;
- wrong-network rejection;
- public-key mismatch;
- nonce replay;
- confirmed replay;
- expiry boundary;
- fee sufficiency;
- maximum-size boundary;
- malformed/unknown version.

## 13. Open questions

Maximum encoded transaction size; exact expiry representation; fee/gas accounting; minimum gas price; fee burn; failed-transaction fee semantics; address versioning; signature format upgrade path; and production transaction schema version are **TBD — requires governance, economic, and security review**.

## 14. Review checklist

- [ ] Current V2 wire contract preserved.
- [ ] Proposed fee/expiry fields are not presented as implemented.
- [ ] Network identity is mandatory.
- [ ] Nonce and replay checks are deterministic.
- [ ] No final economic parameter is selected.
