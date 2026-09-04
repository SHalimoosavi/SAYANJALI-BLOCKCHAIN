# SAYANJALI BLOCKCHAIN --- Protocol Baseline

**Source of truth:** audited Python implementation at
`ead5330986debbc6420bdd1d392d81051eac8f22`.

This document records behavior discovered in code. It does not invent
missing protocol rules.

## 1. Version identifiers

  -------------------------------------------------------------------------
  Identifier              Current value             Status
  ----------------------- ------------------------- -----------------------
  Python package version  `0.1.0-mvp`               implementation
                                                    metadata; stale
                                                    relative to current
                                                    development checkpoint

  API version             `0.2.0-mvp`               implementation metadata

  P2P protocol name       `sayanjali-p2p`           implemented

  P2P protocol version    `1.0`                     implemented

  Network name default    `sayanjali-mainnet-mvp`   configurable

  Chain ID default        `1`                       configurable

  Serialization version   **UNDEFINED**             no explicit protocol
                                                    field
  -------------------------------------------------------------------------

## 2. Genesis

Configured defaults: - index: `0` - previous hash: 64 zero hex
characters - timestamp: `1735689600.0` (`2025-01-01T00:00:00Z`) - nonce:
`0` - message:
`SAYANJALI BLOCKCHAIN GENESIS BLOCK - SYJ TOKEN NETWORK` - genesis block
difficulty: `0` - genesis contains one informational transaction with
zero amount - genesis transaction hash is SHA-256 of the configured
message

Using the current deterministic serialization implementation, the
default genesis transaction hash is:

`f7c09e086769bcc45992c133e49f373edbe4653447ad6e1eec9bd60cfbf9a59d`

The resulting default genesis block hash is:

`5c95d2d7b63bde94fbfcfe4c2a95850ca96e0fc649e93e545b8b32f3a9c7cc1b`

These values are derived from the current implementation and should be
confirmed by an executable reference vector before being declared
immutable protocol constants.

Initial genesis allocation: **UNDEFINED / none specified**.

## 3. Block header and hash

The block hash covers: - `index` - `previous_hash` - `timestamp` -
`nonce` - `difficulty` - `merkle_root`

Serialization is deterministic JSON: - sorted keys - compact
separators - `default=str`

Hash algorithm: SHA-256 of the UTF-8 serialized header.

Important compatibility requirement: Go must reproduce the exact JSON
representation and numeric formatting or the hash changes.

## 4. Merkle root

Transaction hashes are combined pairwise with SHA-256 of concatenated
child hashes.

If a level has an odd number of hashes, the final hash is duplicated.

For an empty list, the root is SHA-256 of the empty string.

No domain-separation tag is currently defined.

## 5. Transactions

Current fields: - `sender` - `receiver` - `amount_base_units` -
`timestamp` - `sender_public_key` - `signature` - `tx_hash`

Transaction signing payload excludes public key/signature and
contains: - sender - receiver - amount_base_units - timestamp

Transaction hash then includes the signing payload plus public key and
signature.

Signature algorithm: - ECDSA - curve: SECP256k1 - SHA-256 message
hashing - hex-encoded signature/public key

Transaction nonce: **UNDEFINED / not present**.

## 6. Addresses

Wallet address derivation is:

`"SYJ" + first 40 hexadecimal characters of SHA-256(public_key_hex)`

The public key is the library's raw SECP256k1 verifying-key
serialization.

Address validation checks prefix and hexadecimal length only; it does
not prove ownership.

## 7. Monetary protocol

Maximum supply:

`720,000,000 SYJ`

Base unit:

`1 SYJ = 100,000,000 base units`

Maximum supply in base units:

`72,000,000,000,000,000`

Monetary protocol state uses integer base units.

Default block reward:

`50 SYJ = 5,000,000,000 base units`

The configured halving interval is `210,000` blocks but is **not
currently enforced**.

Supply invariant:

`0 <= total_supply <= 72,000,000,000,000,000`

Mining caps a block reward to remaining supply and stops issuance when
no supply remains.

## 8. State transition

Current authoritative balance model: - confirmed balances are
derived/cached from block transaction effects; - coinbase adds to
receiver balance and total issuance; - normal transactions subtract from
sender and add to receiver; - a transaction cannot spend more than
confirmed balance during chain validation; - mempool admission
additionally accounts for pending sender spend.

No explicit account nonce is used.

No state root is part of consensus.

Snapshots are **UNDEFINED**.

## 9. Coinbase

Every non-genesis block must contain exactly one coinbase.

The expected reward is: `min(configured_block_reward, remaining_supply)`

A block whose coinbase does not exactly match the protocol-required
amount is rejected.

Coinbase transactions are not wallet-signed and use the reserved sender
identifier:

`SYJ-COINBASE-0000000000000000000000000000`

## 10. Proof of Work

PoW validity: - recompute block hash; - require recorded difficulty to
equal the protocol-required difficulty; - require the hash to begin with
`difficulty` hexadecimal zero characters.

Work accounting:

`work(block) = 16 ** difficulty`

Chain selection: - valid chain - same genesis - strictly greater
accumulated work than local chain

Raw length is not the chain-selection rule.

## 11. Difficulty

Defaults: - initial configured difficulty: `4` - target block time: `30`
seconds - adjustment interval setting: `10` - minimum: `1` - maximum:
`32` - maximum adjustment factor: `4`

The implementation currently takes the last
`difficulty_adjustment_interval` blocks and calculates `len(window)-1`
intervals. Therefore, with the default setting of `10`, the measured
window contains 9 timestamp intervals.

Retargeting: - actual timespan is measured from chain timestamps; -
expected timespan = measured interval count × target block time; -
actual timespan is clamped by the maximum adjustment factor; - current
work basis is `16 ** previous_difficulty`; - rational arithmetic is
used; - geometric midpoint threshold is used to move difficulty by
integer levels.

This exact algorithm must be vectorized before Go implementation.

## 12. Timestamp rules

Current rule: each non-genesis block timestamp must be strictly greater
than its predecessor.

Future timestamp bound: **UNDEFINED / not enforced**.

Median-time-past equivalent: **UNDEFINED / not enforced**.

## 13. Mempool

The mempool is in-memory only.

Rules: - transaction must pass transaction validation; - duplicate
transaction hashes are rejected; - sender pending spend is counted
against confirmed balance; - mining selects oldest pending
transactions; - block transaction count is capped at 500 non-coinbase
transactions; - mempool is not persisted across process restart.

## 14. Persistence

SQLAlchemy Core is used.

Tables include: - blocks - transactions - wallets - peers - node
identity - P2P identity - peer credentials

Monetary fields are stored as integer base units in the current schema.

Reorganization reverses discarded balance effects and applies
replacement-chain effects atomically.

## 15. P2P protocol

Current HTTP endpoints include: - `/network/status` - `/network/peers` -
`/network/peers/register` - `/network/peers/challenge` -
`/network/peers/authenticate` - `/network/chain` - `/network/sync` -
`/network/blocks/receive` - `/network/transactions/receive`

Current P2P message enum includes: `HELLO`, `PEER_LIST`, `PING`, `PONG`,
`GET_CHAIN`, `GET_BLOCK`, `GET_BLOCKS`, `NEW_TRANSACTION`, `NEW_BLOCK`,
`SYNC_REQUEST`, `SYNC_RESPONSE`.

The current implementation does not yet implement the final production
wire protocol requested for: `HELLO_ACK`, `GET_PEERS`, `PEERS`,
`GET_HEADERS`, `HEADERS`, `BLOCKS`, `REJECT`.

Those production semantics remain to be specified.

## 16. Authentication

P2P identity: - separate from wallet identity; - ECDSA SECP256k1; -
persisted private/public key pair; - private key is not transmitted.

Trust establishment: - server-issued random challenge; - peer signs the
challenge; - challenge is single-use and expires.

Ongoing authenticated traffic: - signed envelope; - protocol
name/version; - node ID; - public key; - network name; - genesis hash; -
advertised address; - capabilities; - timestamp; - nonce; - payload
hash; - signature.

Replay protection uses a bounded in-memory cache.

## 17. Synchronization and propagation

Synchronization: - obtains a peer's full chain; - enforces block-count
and response-size limits; - parses blocks; - validates complete chain; -
compares accumulated work; - adopts only a valid higher-work chain.

Propagation: - only trusted peers are used for outbound
block/transaction propagation; - duplicate hashes are bounded by seen
caches; - inbound state changes occur only after authentication and
validation.

The current full-chain model is a prototype/reference behavior, not the
final production P2P protocol.

## 18. Reorganization

A candidate chain must: 1. share the local genesis; 2. be fully valid;
3. have strictly more accumulated work.

The storage layer atomically reverses balances from discarded blocks and
applies balances for the replacement chain.

Mempool reinsertion of transactions orphaned by a reorg is not
implemented.

## 19. Explicitly undefined / reserved

The following are not defined by the current implementation and must not
be invented during Go migration:

-   final token distribution
-   genesis allocation
-   vesting
-   staking economics
-   burn economics
-   governance economics
-   transaction nonce semantics
-   state root
-   block size consensus limit
-   fee market / transaction fees
-   future timestamp bound
-   median-time-past
-   production transport framing
-   final production P2P message grammar
-   serialization version field
-   network-version negotiation semantics
-   snapshot format
-   mainnet validator/consensus upgrade rules
