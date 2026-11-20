# SAYANJALI BLOCKCHAIN — C-2 Frozen SMT Test Vectors

## Freeze status

**Status:** FROZEN FOR BUILD 2A REVIEW — owner-approved amendment required for any later modification.

**Specification version:** `state-smt-v3-vector-0.1`

**Vector file:** `protocol/test-vectors/state-smt-v3.json`

**Independent generator:** `scripts/c2/generate_state_smt_v3_vectors.py`

**Go verifier:** `scripts/c2/state_smt_v3_vectors_test.go`

**Generator language:** Python standard library only.

**Generation timestamp:** `2026-10-02T02:54:56Z`

**Vector count:** 10 (`V1` through `V10`)

**V8 account count:** exactly 1,000

**Vector SHA-256:** `3941cc06c641557f896a6f6235e9a94abe91ed66eb99949e0069860e2d9d7fc2`

## Generator command

```text
python3 scripts/c2/generate_state_smt_v3_vectors.py
```

The generator was executed twice. Both generated files produced the same SHA-256:

```text
3941cc06c641557f896a6f6235e9a94abe91ed66eb99949e0069860e2d9d7fc2
```

## Cryptographic encoding

This is an independent Build 2A candidate/reference encoding. It is not production activation.

### Address and key

- Address encoding: UTF-8 bytes of the exact address string.
- Key: `SHA256(address_utf8)`.
- Key width: exactly 256 bits / 32 bytes.
- Tree direction: key bits consumed most-significant-bit first.
- Address ordering is not used to form the SMT root.

### Leaf

Canonical leaf bytes:

```text
version_u8
|| key_32
|| address_len_u32_be
|| address_utf8
|| balance_u64_be
|| nonce_u64_be
```

Leaf hash:

```text
SHA256(LEAF_TAG || canonical_leaf_bytes)
```

The leaf domain tag is explicitly stored in the vector specification.

### Internal node

```text
SHA256(NODE_TAG || left_hash || right_hash)
```

Left/right ordering is determined by the consumed key bit: `0 = left`, `1 = right`.

### Empty subtrees

Level convention:

- level `256` = leaf level
- level `0` = root level

Empty leaf:

```text
empty[256] = SHA256(EMPTY_TAG || uint16_be(256))
```

Recursive rule:

```text
empty[level] = SHA256(NODE_TAG || empty[level+1] || empty[level+1])
```

The vector file contains all **257** hashes from level 256 through level 0.

### Duplicates

- Different addresses hashing to the same key are rejected.
- A sequential operation on the same address is an update.
- Static account sets must not contain duplicate addresses.

### Delete

Deletion removes the keyed leaf. Its path is reconstructed with the corresponding empty-subtree hashes.

### Root

The root is the 32-byte level-0 hash, rendered as lowercase hexadecimal in the JSON vector file.

### Proofs

Proof encoding contains:

- queried address,
- queried 256-bit key,
- leaf hash,
- 256 sibling hashes from root to leaf,
- 256 direction bits from root to leaf,
- expected root,
- verification result.

Inclusion requires the account leaf to reconstruct the expected root.

Non-inclusion is proven by an **empty leaf** at the queried key plus its sibling path. No neighboring leaf is required by this candidate proof rule.

### JSON transport

JSON is transport only and is never used as cryptographic input. The file is UTF-8, two-space indented, sorted by object key, and newline terminated.

## Required cases

| Vector | Coverage |
|---|---|
| V1 | empty tree |
| V2 | one account |
| V3 | two accounts sharing a long key prefix |
| V4 | two accounts at opposite keyspace extremes (`0x00` and `0xFF` first key byte) |
| V5 | insert sequence |
| V6 | update sequence |
| V7 | delete sequence |
| V8 | exactly 1,000 deterministic accounts |
| V9 | inclusion proofs, including long-prefix and extreme-key cases |
| V10 | non-inclusion proof using empty-leaf semantics |

## V8 deterministic generation

Seed bytes:

```text
SYJ-C2-V3-1000-ACCOUNTS-SEED-20261002
```

For index `i` from `0` through `999`:

```text
address = "SYJ" + SHA256(seed || uint32_be(i)).hexdigest()[:40]
balance = 1,000,000 + ((i * 1,000,003) mod 9,000,000)
nonce = i mod 17
```

No random source is used.

## Independence rule

The Python generator does not import the repository, call Go, shell out to Go, or use third-party Python packages. The Go verifier independently implements the frozen byte encoding and SHA-256 rules and reads the committed vector file as expected-value input.

The vectors are now frozen for Build 2A. Any later modification requires an explicit owner-approved amendment and a new recorded checksum.
