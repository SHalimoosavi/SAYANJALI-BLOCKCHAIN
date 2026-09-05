# SAYANJALI BLOCKCHAIN — Production P2P Wire Protocol v1.0

**Status:** Frozen compatibility specification
**Protocol identity:** `sayanjali-p2p` / `1.0`
**Scope:** Production peer wire layer only. This document does not alter consensus, transaction, block, genesis, monetary, Merkle, PoW, difficulty, or chain-work rules.

## 1. Scope and architecture

The production network stack is layered as:

`TCP transport → fixed frame → envelope/header semantics → message payload → message validation → node handler`

TCP is a byte stream; it does not provide message boundaries. The wire layer provides those boundaries. Consensus serialization remains a separate concern. Consensus objects carried by this protocol are opaque consensus-encoded byte sequences whose contents MUST be produced/consumed by the frozen consensus implementation.

The legacy Python HTTP network remains reference/prototype behavior. It is not wire-compatible with this protocol.

## 2. Transport

The v1.0 wire profile uses a long-lived TCP connection. TLS is **not** part of this wire version; confidentiality is not provided by the wire protocol. Peer identity is authenticated at the message layer during the handshake. A future secure-transport profile MUST NOT silently change the v1.0 frame grammar.

Each connection has one ordered inbound and outbound byte stream. Implementations MUST bound concurrent connections and MUST apply handshake and idle timeouts at the session layer.

## 3. Frame

Every message is exactly one frame:

| Offset | Size | Field | Encoding |
|---:|---:|---|---|
| 0 | 4 | magic | ASCII `SYJP` |
| 4 | 1 | version_major | unsigned 8-bit, `1` |
| 5 | 1 | version_minor | unsigned 8-bit, `0` |
| 6 | 2 | message_type | unsigned 16-bit big-endian |
| 8 | 8 | request_id | unsigned 64-bit big-endian |
| 16 | 4 | payload_length | unsigned 32-bit big-endian |
| 20 | N | payload | exactly `payload_length` bytes |

Frame header size is **20 bytes**. Maximum complete frame size is **4 MiB (4,194,304 bytes)**, therefore `payload_length <= 4,194,284`.

Message type values are fixed:

`1 HELLO, 2 HELLO_ACK, 3 GET_PEERS, 4 PEERS, 5 GET_HEADERS, 6 HEADERS, 7 GET_BLOCKS, 8 BLOCKS, 9 NEW_BLOCK, 10 NEW_TRANSACTION, 11 REJECT`.

Unknown types are rejected. Unsupported versions are rejected. Version 1.0 is the only supported version in this specification.

Requests and responses MUST have a non-zero request ID. Unsolicited propagation (`NEW_BLOCK`, `NEW_TRANSACTION`) uses request ID zero. `REJECT` uses the request ID of the rejected request when one exists, otherwise zero.

A decoder MUST read the 20-byte header before allocating the payload, reject a declared payload above the maximum, and then read exactly the declared number of bytes. Truncated input is an error. Extra bytes after one frame belong to the next frame and MUST NOT be consumed as part of the current frame.

## 4. Primitive encoding

All integers are unsigned unless stated otherwise and use big-endian byte order. Strings are UTF-8 with a 16-bit byte length prefix and a maximum of 256 bytes. Variable byte strings use a 32-bit byte length prefix and an explicit per-field maximum. Fixed byte strings have no length prefix.

No implicit padding, compression, JSON framing, or trailing bytes are permitted.

Capabilities are a 32-bit bitset. Defined bits are `1 = blocks`, `2 = transactions`, `4 = sync`. All other bits are invalid in v1.0.

Hashes are exactly 32 raw bytes (SHA-256 values). P2P public keys are exactly 64 raw bytes in the frozen SYJ raw `X || Y` representation. P2P signatures are exactly 64 raw bytes in frozen raw `R || S` representation.

## 5. Envelope and correlation

The frame header is the complete v1.0 network envelope. It contains version, message type, request ID and payload length. There is no second generic envelope object and no flags field.

Request IDs are connection-local, non-zero, unsigned 64-bit values. A requester MUST NOT reuse an ID while the request is outstanding. A node MUST cap outstanding requests at **64 per connection**. Duplicate IDs for outstanding requests are invalid and MAY result in `REJECT(invalid_request)` followed by connection close.

Responses MUST echo the request ID of the request they answer. `HELLO_ACK`, `PEERS`, `HEADERS`, and `BLOCKS` are responses. `REJECT` correlates through its frame request ID. Unsolicited propagation uses ID zero.

Requests expire after **30 seconds**. A timeout is a local session event; it is not itself a wire message. Cancellation is not supported in v1.0.

## 6. Handshake: HELLO / HELLO_ACK

Handshake timeout is **10 seconds**. No non-handshake message is legal before the handshake is complete.

### HELLO payload

Fields, in order:

1. `protocol_name`: string; MUST equal `sayanjali-p2p`.
2. `version_major`: u8; MUST equal 1.
3. `version_minor`: u8; MUST equal 0.
4. `network_name`: string; MUST equal the locally configured network name.
5. `genesis_hash`: fixed 32 bytes; MUST equal the local genesis hash.
6. `node_id`: string; non-empty, max 256 bytes.
7. `public_key`: variable bytes, exactly 64 bytes.
8. `advertised_address`: string; non-empty, max 256 bytes.
9. `capabilities`: u32 bitset.
10. `challenge`: fixed 32 random bytes generated by the sender for this connection.
11. `signature`: fixed 64 bytes.

The signature covers a domain-separated deterministic byte transcript of fields 1–10 in their wire encoding, prefixed with ASCII `SYJ-P2P-HELLO-v1`. The exact signing primitive is ECDSA secp256k1 with SHA-256 using the existing P2P identity key domain. Signature verification MUST use the existing SYJ raw X||Y / raw R||S representation and MUST NOT involve wallet keys.

The challenge proves freshness of the handshake transcript; it is not a persistent credential and MUST NOT be reused.

### HELLO_ACK payload

Fields, in order, are the same identity fields 1–9, followed by:

10. `echo_challenge`: fixed 32 bytes; MUST equal the HELLO challenge.
11. `challenge`: fixed 32 random bytes generated by the responder.
12. `signature`: fixed 64 bytes.

The ACK signature covers fields 1–11, prefixed with ASCII `SYJ-P2P-HELLO-ACK-v1`. The initiator MUST verify the echoed challenge and responder signature before entering ESTABLISHED state.

Handshake failure reasons map to the rejection codes in section 15. Wrong network or genesis MUST NOT be treated as a successful session.

### Versioning

Only 1.0 is supported. A peer advertising another version is rejected with `unsupported_version`. There is no downgrade negotiation. A future incompatible protocol uses a new major version; a future compatible minor version requires an explicit compatibility specification before deployment.

## 7. GET_PEERS / PEERS

`GET_PEERS` payload:

1. `start_after`: string, empty for first page; max 256 bytes.
2. `limit`: u16; 1..256.

`PEERS` payload:

1. `count`: u16; 0..256.
2. Repeated peer records, each:
   - `node_id`: string, non-empty, max 256 bytes.
   - `host`: string, non-empty, max 256 bytes.
   - `port`: u16, 1..65535.
   - `capabilities`: u32 with only defined bits.
3. `next_start_after`: string; empty means no more pages; max 256 bytes.

The responder MUST return entries in deterministic lexical `node_id` order and MUST apply `start_after` as a strict exclusive cursor. A page MUST contain at most the requested limit. A peer implementation MUST cap stored/processed advertisements at 256 per response.

Host/address policy is separate from wire parsing. Implementations MUST validate advertised endpoints before dialing or persisting them, including private/local-address policy and DNS rebinding defenses. The wire codec itself only enforces syntax and bounds.

## 8. GET_HEADERS / HEADERS

`GET_HEADERS` payload:

1. `locator_count`: u8; 1..32.
2. `locator_hashes`: that many 32-byte block hashes, ordered **newest known hash to oldest**.
3. `stop_hash`: 32 bytes; all-zero means no stop hash.
4. `max_count`: u16; 1..2048.

The locator is a bounded ancestry locator, not a height claim. The responder searches the locator list from first to last and selects the first hash it knows that belongs to its active chain. It returns headers strictly after that common ancestor, oldest-to-newest. If no locator is known, the responder starts at the genesis successor. If `stop_hash` is non-zero, response generation stops after including that hash when it occurs on the selected active chain. The response is also bounded by `max_count` and 2048 headers.

`HEADERS` payload:

1. `count`: u16; 0..2048.
2. Repeated consensus-header byte strings, each length-prefixed u32 and max 4096 bytes.

An empty `HEADERS` response is valid and means there are no additional headers satisfying the request. Header continuity, hash linkage, ordering, and consensus validity are checked by the chain/sync layer, not trusted from the network codec.

## 9. GET_BLOCKS / BLOCKS

`GET_BLOCKS` payload:

1. `count`: u16; 1..128.
2. `count` block hashes, each 32 bytes, in requester order.

`BLOCKS` payload:

1. `count`: u16; 0..32.
2. Repeated consensus block byte strings, each u32-length-prefixed and max 512 KiB.

The responder returns blocks in the same order as the requested hashes for blocks it has. Missing blocks may be omitted; the response is allowed to be partial. The node layer correlates returned block hashes with the request and treats missing identifiers as `not_found` only when its synchronization algorithm requires an explicit error. Duplicate returned blocks are invalid at the message-consumer layer.

## 10. NEW_BLOCK

Payload is one consensus block byte string, u32 length-prefixed, max 512 KiB. Request ID MUST be zero.

Receipt is not acceptance. The receiver MUST deduplicate by block hash and MUST pass the decoded block through normal consensus validation before any chain-state mutation or relay. Invalid blocks are rejected with `invalid_block`.

## 11. NEW_TRANSACTION

Payload is one consensus transaction byte string, u32 length-prefixed, max 64 KiB. Request ID MUST be zero.

Receipt is not acceptance. The receiver MUST deduplicate by transaction hash and MUST pass the transaction through normal transaction/mempool validation. This protocol introduces no fee, priority, or economic policy.

## 12. REJECT

Payload:

1. `code`: u16.
2. `retryable`: u8, exactly 0 or 1.
3. `close`: u8, exactly 0 or 1.
4. `reason`: UTF-8 string, 1..256 bytes.

The frame request ID identifies the rejected request. For unsolicited input, request ID is zero.

Stable v1.0 codes:

| Code | Name | Retryable | Close | Reputation effect |
|---:|---|---|---|---|
| 1 | malformed_message | no | yes | protocol-failure score MAY increase |
| 2 | unsupported_version | no | yes | no score |
| 3 | wrong_network | no | yes | no score |
| 4 | wrong_genesis | no | yes | no score |
| 5 | invalid_request | no | yes | protocol-failure score MAY increase |
| 6 | invalid_block | no | no | consensus-invalid score MAY increase |
| 7 | invalid_transaction | no | no | transaction-invalid score MAY increase |
| 8 | not_found | yes | no | no score |
| 9 | too_large | no | yes | protocol-failure score MAY increase |
| 10 | rate_limited | yes | no | no score |
| 11 | invalid_state | no | no | no score |
| 12 | unauthorized | no | yes | auth-failure score MAY increase |

The codec does not assign reputation scores. “MAY increase” is the only normative wire-level effect; exact scoring/ban thresholds remain a node-policy concern.

The `retryable` and `close` bits MUST match the table for locally generated standard errors. Receivers MUST reject a REJECT payload whose code is unknown or whose booleans are not 0/1.

## 13. Resource limits

Hard v1.0 limits:

- frame: 4 MiB
- string: 256 bytes
- peer entries: 256
- outstanding requests/connection: 64
- request timeout: 30 s
- handshake timeout: 10 s
- locator hashes: 32
- headers/request or response: 2048
- block hashes/request: 128
- blocks/response: 32
- block payload: 512 KiB each
- transaction payload: 64 KiB
- header payload: 4 KiB each
- capability bits: 3 defined bits

A decoder MUST reject invalid counts before allocating a collection based on the count. Implementations MUST avoid spawning unbounded work for a single frame.

## 14. Peer/session state machine

States:

`DISCONNECTED → CONNECTING → HANDSHAKING → ESTABLISHED → CLOSING → DISCONNECTED`

`ESTABLISHED` has two logical activities, synchronization and active propagation; these are not separate wire states.

Legal messages:

- CONNECTING: no application messages.
- HANDSHAKING: HELLO, then HELLO_ACK only as defined by the handshake role.
- ESTABLISHED: GET_PEERS, PEERS, GET_HEADERS, HEADERS, GET_BLOCKS, BLOCKS, NEW_BLOCK, NEW_TRANSACTION, REJECT.
- CLOSING: no new application messages.

A message in an illegal state is a malformed protocol interaction and maps to `malformed_message`; the connection closes.

## 15. Error handling and malformed input

Malformed magic, unsupported version, unknown type, zero request IDs on requests/responses, invalid lengths, invalid counts, invalid fixed-field lengths, invalid UTF-8 where strings are decoded, trailing bytes in a message payload, and invalid capability bits MUST be rejected without panic.

An implementation MUST NOT allocate based on a remote length before checking it against the applicable limit. A malformed frame MUST consume no more than the bytes needed to classify the error; implementations MAY close immediately.

## 16. Consensus boundary

The P2P layer is not a consensus engine. Network-delivered headers, blocks, and transactions are untrusted input. Their canonical consensus encoding and semantic validity remain governed by the frozen Phase 4 specification and the Go Phase 5.1 implementation. Network framing MUST NOT change consensus serialization.

## 17. Compatibility and freeze rules

This v1.0 grammar is frozen. Any change to field order, field width, type number, encoding, limit, correlation rule, handshake transcript, synchronization semantics, or rejection meaning requires a protocol-version decision, ADR, updated specification, new wire vectors, and compatibility testing.

The legacy HTTP reference implementation remains unchanged. No Phase 4 consensus vector is regenerated by this specification.

## 18. Deterministic test vectors

Machine-readable wire fixtures are stored under `tests/p2p/fixtures/`. Each valid vector contains a complete frame in hexadecimal and the decoded semantic fields. Negative vectors contain the mutation and expected decoder error class. The fixture set is generated by `tests/p2p/reference_codec.py` and verified by the Go codec tests.

P2P wire vectors are intentionally separate from `protocol/test-vectors/*.json`.

## 19. Examples

A minimal `GET_PEERS` request is a 20-byte frame header followed by a two-byte string length of zero and a two-byte limit. A `NEW_TRANSACTION` request is a 20-byte frame header followed by a u32 byte length and the consensus transaction bytes.

The exact byte examples are machine-generated and frozen in the fixture set; implementations MUST test against those bytes rather than relying on prose examples.

## 20. Explicit non-goals

This document does not define node storage, chain selection implementation, peer reputation algorithms, mempool economics, mining, RPC, TLS, NAT traversal, discovery bootstrapping, persistent peer databases, or the Phase 6 node lifecycle.
