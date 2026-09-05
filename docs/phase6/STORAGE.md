# Phase 6 Storage Architecture

The Go node uses an append-only binary journal under `<data-dir>/chain/ledger.journal`.

Record types:

- `1`: block payload
- `2`: active-tip hash

Each record contains a fixed magic/version/type/length/CRC header followed by the bounded payload. Blocks are serialized using the existing JSON-compatible block/transaction dictionary representation; block hashes and all consensus calculations remain governed by the existing core implementation.

Startup replays every record, verifies CRCs and decodes every stored block. Any malformed record, invalid length, unsupported database version or checksum mismatch fails startup rather than silently repairing state.

The active tip is a separate durable record. This makes the acceptance operation crash-safe at the canonical-tip boundary: an interrupted write can leave an already-stored branch block without changing the previous active tip.

No runtime databases, identities or journals are included in source ZIP deliveries.
