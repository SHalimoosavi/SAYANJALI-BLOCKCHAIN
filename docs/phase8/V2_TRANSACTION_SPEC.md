# V2 Transaction Specification

## Wire/codec schema

```json
{
  "version": 2,
  "network_id": "<64 lowercase hex>",
  "sender": "...",
  "receiver": "...",
  "amount_base_units": 1,
  "nonce": 0,
  "timestamp": 1735689601.25,
  "sender_public_key": "<128 lowercase hex>",
  "signature": "<128 lowercase hex>",
  "tx_id": "<64 lowercase hex>"
}
```

`tx_hash` is a V1 field. V2 does not use it as the transaction identity boundary.

## Signing payload

The canonical payload is the sorted-key JSON object containing:

- `amount_base_units`
- `network_id`
- `nonce`
- `receiver`
- `sender`
- `sender_public_key`
- `timestamp`
- `version: 2`

Signing bytes:

`ASCII("SYJ-TX-SIGN-V2\\0") || canonical_signing_payload_UTF8`

The existing secp256k1 implementation hashes those signing bytes with SHA-256 before ECDSA signing, preserving the repository's raw `R||S` signature representation and low-S behavior.

## Transaction ID

`tx_id = SHA256(ASCII("SYJ-TX-ID-V2\\0") || canonical_signing_payload_UTF8)`

The signature is intentionally excluded. The public key remains included in the signed payload.

## Validation

A normal V2 transfer is accepted only when:

1. version is exactly `2`;
2. network ID is exactly 64 lowercase hexadecimal characters and equals the chain identity;
3. amount is positive and within the protocol maximum;
4. timestamp is finite;
5. sender and receiver are valid SYJ addresses and differ;
6. sender maps to the supplied public key;
7. signature verifies over the V2 signing bytes;
8. recomputed `tx_id` equals the supplied `tx_id`.

## Coinbase

Coinbase is a separate V2 special transaction. It has version `2`, the chain network ID, nonce `0`, the reserved coinbase sender, no public key, no signature, the exact consensus reward, and a deterministic `tx_id`. It does not consume a normal account nonce.
