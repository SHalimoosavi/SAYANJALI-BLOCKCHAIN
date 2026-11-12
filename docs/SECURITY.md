# SAYANJALI BLOCKCHAIN --- Security Model

## Scope

This document describes the security posture observed at the Phase 3
checkpoint. It does not claim perfect security.

## Threat matrix

  --------------------------------------------------------------------------
  Threat                  Current status          Notes
  ----------------------- ----------------------- --------------------------
  Double spending         Mitigated               Confirmed-balance state
                                                  validation plus mempool
                                                  pending-spend admission

  Forged transactions     Mitigated               SECP256k1 signature
                                                  verification and
                                                  sender/public-key binding

  Forged blocks           Mitigated               Header hash, PoW and chain
                                                  validation

  Coinbase inflation      Mitigated               Exact reward and
                                                  maximum-supply enforcement

  Supply overflow         Mitigated               Integer base units and
                                                  replayed supply ceiling

  Replay                  Partially mitigated     P2P nonce/timestamp cache;
                                                  transaction nonce is
                                                  undefined

  Malformed messages      Partially mitigated     Pydantic, parsing and
                                                  body-size controls;
                                                  production wire grammar
                                                  not frozen

  Peer poisoning          Partially mitigated     Authenticated trust plus
                                                  genesis/network checks;
                                                  peer reputation is limited

  Sybil                   Planned / partially     Key authentication is not
                          mitigated               Sybil resistance

  Eclipse                 Planned                 No mature
                                                  peer-selection/diversity
                                                  strategy

  DoS                     Partially mitigated     Body limits, bounded
                                                  caches, rate limiting and
                                                  backoff

  Rate abuse              Partially mitigated     Process-local fixed-window
                                                  limiter

  SSRF                    Partially mitigated     Address validation; DNS
                                                  rebinding remains possible

  Address spoofing        Mitigated for signed    Public-key/address
                          transfers               correspondence is verified

  Reorg abuse             Partially mitigated     Higher-work valid chain
                                                  required; orphaned tx
                                                  reinsertion absent

  Invalid difficulty      Mitigated               Expected difficulty is
                                                  independently derived

  Timestamp manipulation  Partially mitigated     Strictly increasing
                                                  timestamps only

  Serialization attacks   Partially mitigated     Deterministic JSON exists;
                                                  canonical cross-language
                                                  format not frozen

  Key compromise          Partially mitigated     Keys are separated by
                                                  trust domain; secure key
                                                  custody remains
                                                  operational responsibility
  --------------------------------------------------------------------------

## Security boundaries

### Wallet keys

Wallet private keys authorize value movement and must remain
client-side.

Current status: `/wallet/create` does not return private-key material. This is
acceptable only as an MVP development flow and must not be treated as
production wallet custody architecture.

### P2P keys

P2P identity keys authenticate nodes and are separate from wallet keys.

### Consensus

P2P, API, CLI and mining code must not invent consensus rules.

## Production security gates

Before public testnet: - formal wire protocol; - secure transport; -
peer diversity/reputation strategy; - fuzz/property testing; -
crash/power-loss recovery; - dependency pinning; - CI security checks; -
adversarial consensus tests; - key-management review; - external
security review.

## Phase 9.5 Security Status

Phase 9.5 adds security hardening for deterministic state commitment, incremental state validation, checkpoint persistence/recovery, Merkle commitment semantics, and V2 chain-reorganization state handling.

### Implemented and locally validated

- Deterministic state snapshots and state-root computation.
- Deterministic account ordering.
- Bounded checkpoint decoding with account and field-size limits.
- Complete checkpoint reads using `io.ReadFull`.
- Rejection of truncated checkpoint data.
- Rejection of duplicate account entries.
- Deterministic state-root vectors.
- Merkle Protocol V3 with domain-separated leaf/node/root hashing and leaf-count commitment.
- Regression coverage for the historical duplicate-last Merkle ambiguity.
- Incremental state validation and periodic checkpoints.
- Checkpoint restart/recovery validation.
- Legacy replay versus incremental-state differential validation.
- V2 divergent-branch/reorganization state validation.
- State-root verification following V2 reorganization.
- Safer block → checkpoint → durable-tip persistence ordering.

### Current verification environment

The latest local validation environment is:

- Go `1.27.0 android/arm64`
- Python `3.14.6`

Validated locally:

- Go tests — **PASS**
- Go vet — **PASS**
- Go build — **PASS**
- gofmt verification — **PASS**
- Python tests — **262 passed, 1 warning**
- Python compileall — **PASS**
- pip-audit — **PASS / No known vulnerabilities**
- Bandit — **PASS / No issues identified**

The following security/validation gates remain **NOT EXECUTED** in the current environment:

- staticcheck
- govulncheck
- gosec
- Go race detector on Android/ARM64
- fuzzing
- empirical 400-block mining/difficulty regression
- live Phase 9.5 runtime verification harness
- external security audit

### Security interpretation

Phase 9.5 implementation and local validation do **not** establish production security approval.

The state root is currently a deterministic **sidecar/checkpoint commitment** and is not an active state-root field in the block-header consensus format.

Merkle Protocol V3 is implemented and tested, but activation and network compatibility remain deferred.

Historical V1 mempool/reorganization semantics remain an open issue and are not claimed as resolved by Phase 9.5.

The improved block/checkpoint/durable-tip ordering does not constitute crash/fault-injection proof of fully atomic persistence. That verification remains outstanding.

Cosmos SDK / CometBFT remains a future architecture direction and is not implemented in the current repository.

**Project security status: NOT PRODUCTION-READY / NOT MAINNET-READY.**


## Security language policy

Use: - Implemented - Mitigated - Partially mitigated - Planned -
Undefined

Do not use: - "perfectly secure" - "unhackable" - "fully decentralized"
unless the architecture actually supports the claim - "BFT consensus"
merely because nodes synchronize


## Production transport and secret boundary

The Go production-track API must use TLS whenever it listens on a non-loopback address. Bearer authentication is not considered a substitute for encrypted transport. P2P production transport now requires TLS 1.3 with mutual certificate verification on non-loopback listeners. Phase 9.3 adds a HELLO_FINISH authentication step and therefore requires the documented P2P protocol-minor amendment; old handshake peers are not assumed compatible.

Node identity private keys are not stored as plaintext by the production-track identity loader. `SYJ_IDENTITY_ENCRYPTION_KEY` must be supplied out of band as 32 random bytes encoded as 64 hexadecimal characters. Do not put this value in source, genesis configuration, logs, CI output, or committed files.
