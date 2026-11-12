# Decision note: address checksum encoding

**Status:** OWNER DECISION REQUIRED. No address-format change is authorized by this note.

## Current baseline

The current SYJ address is the `SYJ` prefix followed by a 40-character lowercase hexadecimal suffix derived from the public key. The format has no checksum and no network discriminator.

## Options

### Option A — retain current hexadecimal format

- Advantages: backward compatibility and simple implementation.
- Trade-offs: mistyped but syntactically valid addresses are not reliably detected; addresses do not identify a network.

### Option B — Base58Check-style encoding

- Advantages: familiar human-readable alphabet and checksum support.
- Trade-offs: new encoding and canonicalization rules; migration and compatibility handling required.

### Option C — Bech32/Bech32m-style encoding

- Advantages: strong transcription error detection, case rules and human-readable network prefix.
- Trade-offs: a new address format; versioning, HRP/network mapping and migration rules must be specified.

## Recommendation

Evaluate a versioned Bech32m-style format for a future address version, but do not change the current validator or accept a new address format until the owner approves a migration contract and compatibility vectors.
