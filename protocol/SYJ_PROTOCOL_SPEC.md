# SYJ Protocol Specification --- Initial Frozen Baseline

**Protocol status:** Draft for formalization\
**Reference implementation:** Python Phase 3 commit
`ead5330986debbc6420bdd1d392d81051eac8f22`

> This document is authoritative only after review and acceptance.
> Values marked UNDEFINED are deliberately not invented.

## 1. Protocol version

Current P2P protocol version: `1.0`.

A chain-wide formal protocol version field: **UNDEFINED**.

## 2. Network identifier

Default network name: `sayanjali-mainnet-mvp`.

Network name is configurable in the current implementation.

## 3. Chain ID

Default chain ID: `1`.

Chain ID is configurable. The genesis hash and network name are
currently stronger network-identity anchors in P2P authentication.

## 4. Genesis block

Default: - index `0` - previous hash `00...00` (64 zeros) - timestamp
`1735689600.0` - nonce `0` - difficulty `0` - message
`SAYANJALI BLOCKCHAIN GENESIS BLOCK - SYJ TOKEN NETWORK`

Derived default genesis transaction hash:

`f7c09e086769bcc45992c133e49f373edbe4653447ad6e1eec9bd60cfbf9a59d`

Derived default genesis block hash:

`5c95d2d7b63bde94fbfcfe4c2a95850ca96e0fc649e93e545b8b32f3a9c7cc1b`

Genesis allocation: **UNDEFINED**.

## 5. Block header

The hashed header fields are:

`index, previous_hash, timestamp, nonce, difficulty, merkle_root`

Serialization is deterministic JSON with sorted keys and compact
separators.

## 6. Block serialization

Current wire/storage representation is JSON-compatible dictionary data
containing: - header fields - block hash - transaction list

Canonical byte-level serialization for cross-language consensus: **TO BE
FROZEN FROM VECTORS**.

## 7. Block hash

SHA-256 of deterministic serialized header.

No double-SHA-256 is used for block hashing.

## 8. Transaction format

Fields: - sender - receiver - amount_base_units - timestamp -
sender_public_key - signature - tx_hash

Transaction nonce: **UNDEFINED**.

## 9. Transaction serialization

Current Python implementation serializes transaction data as
deterministic JSON for hashing and JSON dictionaries for
persistence/API.

Canonical byte-level format for Go compatibility: **TO BE FROZEN FROM
VECTORS**.

## 10. Transaction hash

SHA-256 of the deterministic transaction payload including: - sender -
receiver - amount_base_units - timestamp - sender_public_key - signature

## 11. Signature algorithm

ECDSA on SECP256k1 with SHA-256.

The current Python library signs messages with its own ECDSA signing
behavior. Cross-language deterministic signature vectors require a
specified nonce/signature encoding policy before signatures can be used
as byte-for-byte vectors.

## 12. Public key format

Current wallet/P2P public keys are the raw verifying-key byte
representation, hex encoded.

Exact canonical byte format must be frozen in compatibility vectors.

## 13. Address derivation

`SYJ` + first 40 hex characters of SHA-256(public_key_hex).

This is a truncated hash of the hex-encoded public key string, not the
raw key bytes.

## 14. Merkle root

-   leaf values are transaction hashes;
-   concatenate two child hash strings;
-   SHA-256 the concatenation;
-   duplicate the last hash on odd-width levels;
-   empty tree root is SHA-256(empty string).

## 15. Coinbase rules

Every non-genesis block contains exactly one coinbase.

Coinbase is not signed.

Reserved sender: `SYJ-COINBASE-0000000000000000000000000000`

## 16. Block reward

Current default configured reward:

`50 SYJ`.

Final emission schedule: **UNDEFINED**.

The configured `210,000` halving interval is reserved but not enforced.

## 17. Maximum supply

`720,000,000 SYJ`

## 18. SYJ base unit

`1 SYJ = 100,000,000 base units`

Maximum supply: `72,000,000,000,000,000 base units`

## 19. Monetary representation

All authoritative monetary state uses integer base units.

Human-facing decimal strings are presentation only.

## 20. Supply accounting

Every accepted non-genesis coinbase increases total issuance.

Required invariant:

`0 <= total_supply <= 72,000,000,000,000,000`

## 21. Reward exhaustion

When remaining supply is less than the configured reward, the reward is
capped to the exact remaining supply.

When remaining supply is zero, further issuance is rejected.

## 22. Difficulty algorithm

Defaults: - configured initial difficulty `4` - target `30` seconds -
adjustment interval `10` - minimum `1` - maximum `32` - adjustment
factor `4`

Current algorithm measures timestamps across the configured recent-block
window, clamps the timespan, derives target work using exact rational
arithmetic and moves integer difficulty using geometric-midpoint
thresholds.

The exact algorithm must be captured in executable vectors before Go
implementation.

## 23. Difficulty encoding

Difficulty is the count of leading hexadecimal zero characters required
in the SHA-256 block hash.

## 24. Difficulty retarget

Current window behavior is part of the reference implementation and must
not be "corrected" during Go migration without a protocol-change
decision.

Notably, a 10-block recent window contains 9 timestamp intervals.

## 25. Chain work

`work(block) = 16 ** difficulty`

Chain work is the sum of block work.

## 26. Chain selection

A candidate chain must: - have the same genesis; - pass full
validation; - have strictly greater accumulated work.

## 27. Timestamp rules

A non-genesis block timestamp must be strictly greater than the previous
block timestamp.

Future-time bound: **UNDEFINED**.

Median-time-past: **UNDEFINED**.

## 28. Mempool rules

-   validate before admission;
-   reject duplicate tx hash;
-   enforce confirmed balance minus pending spend;
-   select oldest transactions for mining;
-   cap selected non-coinbase transactions at 500;
-   do not persist mempool across restart.

## 29. State transition

For each block: 1. validate exactly one coinbase; 2. apply coinbase
issuance; 3. validate every normal transaction; 4. debit sender; 5.
credit receiver; 6. enforce supply ceiling.

State root: **UNDEFINED**.

## 30. Block validation

Validation includes: - sequential index; - previous hash; - increasing
timestamp; - Merkle root; - block hash; - exact protocol-required
difficulty; - valid PoW; - exact coinbase reward; - transaction
validity; - deterministic monetary replay.

## 31. Transaction validation

Validation includes: - positive integer base-unit amount; -
maximum-supply bound on individual amount; - valid receiver address; -
valid sender address for normal transfers; - sender/public-key
correspondence; - signature verification; - transaction-hash
integrity; - sender != receiver for normal transfers.

## 32. Genesis rules

Genesis is built deterministically from `GenesisConfig`.

No genesis allocation is defined.

## 33. P2P protocol semantics

Current implementation uses HTTP JSON endpoints and an internal message
enum.

The future production protocol must formally define: - HELLO -
HELLO_ACK - GET_PEERS - PEERS - GET_HEADERS - HEADERS - GET_BLOCKS -
BLOCKS - NEW_BLOCK - NEW_TRANSACTION - REJECT

These production messages are not yet a frozen wire grammar.

## 34. Peer authentication

P2P identity uses separate SECP256k1 keys.

Initial trust uses a server-issued challenge.

Ongoing requests use signed envelopes with freshness, nonce and payload
binding.

## 35. Synchronization

Current synchronization is full-chain: - retrieve peer chain; -
resource-limit it; - parse; - fully validate; - compare work; - replace
only when strictly superior.

## 36. Propagation

Only trusted peers are used for outbound block/transaction propagation.

Seen-hash caches prevent repeated processing and immediate relay loops.

## 37. Reorganization

A higher-work valid chain can replace the local chain.

Storage reverses discarded balance effects and applies replacement-chain
effects atomically.

Orphaned transaction reinsertion is not implemented.

## 38. Error/rejection rules

Current implementation rejects malformed, invalid, unauthorized,
oversized, duplicate or inferior data at the relevant layer.

A normative machine-readable rejection-code registry: **UNDEFINED**.

## 39. Version negotiation

Current P2P implementation supports only `1.0`.

A production compatibility negotiation mechanism is **TO BE DEFINED**.

## 40. Future compatibility

Breaking changes require: - explicit protocol version transition; -
ADR; - updated specification; - compatibility vectors; - migration
strategy; - testnet validation.

No programming-language migration alone may change protocol semantics.
