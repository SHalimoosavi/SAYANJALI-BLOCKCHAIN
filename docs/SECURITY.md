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

Current limitation: `/wallet/create` returns a private key. This is
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

## Security language policy

Use: - Implemented - Mitigated - Partially mitigated - Planned -
Undefined

Do not use: - "perfectly secure" - "unhackable" - "fully decentralized"
unless the architecture actually supports the claim - "BFT consensus"
merely because nodes synchronize


## Production transport and secret boundary

The Go production-track API must use TLS whenever it listens on a non-loopback address. Bearer authentication is not considered a substitute for encrypted transport. P2P supports a TLS transport wrapper around the frozen Phase 5.2 application protocol; enabling TLS does not alter frame IDs, HELLO encoding, or application payload grammar.

Node identity private keys are not stored as plaintext by the production-track identity loader. `SYJ_IDENTITY_ENCRYPTION_KEY` must be supplied out of band as 32 random bytes encoded as 64 hexadecimal characters. Do not put this value in source, genesis configuration, logs, CI output, or committed files.
