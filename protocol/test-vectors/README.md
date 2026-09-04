# SYJ Protocol Compatibility Vectors

This directory freezes the **observed Python reference behavior** for protocol-critical deterministic operations. The vectors are generated from the implementation; expected values are not manually authored.

## Scope

- genesis construction and identity
- SECP256k1 public-key representation and SYJ address derivation
- transaction signing payload, canonical signing message, fixed signature verification, and transaction hash
- Merkle roots
- block-header serialization and hashing
- hexadecimal-leading-zero PoW checks
- difficulty retargeting, including the 9-interval behavior of a 10-block window and the existing integer clamp/midpoint rules
- accumulated chain work
- native SYJ monetary conversion, issuance, supply boundary, and insufficient-balance rejection

## Canonical serialization currently observed

The Python reference function is `blockchain.utils.deterministic_json`:

```python
json.dumps(payload, sort_keys=True, separators=(",", ":"), default=str)
```

Therefore the frozen behavior is:

- UTF-8 encoding of the resulting JSON string for hashing/signing.
- lexicographically sorted object keys (`sort_keys=True`).
- no optional whitespace (`separators=(",", ":")`).
- Python's standard `json.dumps` string escaping behavior (`ensure_ascii=True` by default).
- integers remain JSON integers; floats use Python's JSON numeric rendering.
- booleans/null follow Python JSON semantics; the current serializer also retains Python's default `allow_nan=True` behavior for non-finite floats, although such values are not valid protocol inputs in the frozen fixtures.
- `default=str` is the current fallback for otherwise non-JSON-serializable values; this is an observed implementation detail, not a new protocol rule.
- field presence is determined by the actual payload-building function; omitted fields are not implicitly added by the serializer.

The current protocol-critical payloads use ordinary strings, integers, floats, booleans/null where applicable. The serializer's generic `default=str` fallback is documented rather than redesigned.

## Cryptographic fixture rule

`Wallet.sign()` currently uses the Python ECDSA library's non-deterministic `sign()` operation. The transaction vector therefore freezes one valid signature fixture instead of requiring every implementation to reproduce the same ECDSA nonce. Implementations must reproduce the signing payload/message, verify the fixed signature, and reproduce the transaction hash from that fixed signature.

The wallet public-key representation is the raw `VerifyingKey.to_string()` bytes encoded as lowercase hexadecimal. It is **not** SEC1 `04 || X || Y` encoding.

## P2P identity note

The current authenticated P2P handshake carries protocol version, network name, genesis hash, identity, advertised address, capabilities and authentication data. `chain_id` is **not** in the current handshake wire payload. The current genesis hash is the strong chain-identity boundary used by handshake verification. This vector package does not add `chain_id` to the wire protocol.

## Running

From the repository root:

```bash
python scripts/protocol_vectors/generate.py
python scripts/protocol_vectors/verify.py
pytest -q
```

Generation is reproducible because all fixture inputs and the one intentionally frozen non-deterministic signature are fixed in the generator. Difficulty fixtures intentionally use integer timestamps: the current reference implementation passes the clamped timespan directly to `Fraction(numerator, denominator)`, so fractional timestamp windows currently raise a `TypeError` rather than defining a cross-language rule. This behavior is documented as a freeze issue and is not modified here. The verifier reconstructs protocol objects using the Python reference implementation and fails loudly on drift.

## Vector files

- `genesis.json`
- `address.json`
- `transaction.json`
- `merkle.json`
- `block.json`
- `pow.json`
- `difficulty.json`
- `chain_work.json`
- `monetary.json`

This package is a compatibility freeze, not a new consensus design. It does not define genesis allocation, a chain-wide protocol version, transaction nonces, production P2P framing, or new token economics.
