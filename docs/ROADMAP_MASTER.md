# SAYANJALI BLOCKCHAIN --- Master Roadmap

This document preserves the historical roadmap and adds the Python → Go
production transition without deleting or renumbering the historical
phases.

## Historical roadmap

### Phase 1 --- Blockchain Core

**Status:** COMPLETED

Block model, hashing, Merkle root, genesis, persistence and chain
validation.

### Phase 2 --- Wallets

**Status:** COMPLETED

SECP256k1 wallets, address derivation and key import/export.

### Phase 3 --- Transactions

**Status:** COMPLETED

Signed transactions and coinbase transactions.

### Phase 4 --- Mining

**Status:** COMPLETED

Proof of Work, mempool, block assembly and rewards.

### Phase 5 --- REST API & CLI

**Status:** COMPLETED

FastAPI, Typer CLI and persistent storage.

### Phase 6 --- Networking

**Status:** COMPLETED

HTTP P2P registration, discovery, propagation and accumulated-work
synchronization.

### Phase 6.5 --- Networking Hardening

**Status:** COMPLETED

Challenge-response peer authentication, replay protection,
SSRF-resistant address validation, body limits and bounded rate
limiting.

### Phase 7 --- Consensus Improvements

**Status:** COMPLETED for ratio-based difficulty retarget

PoS/DPoS design work remains unimplemented.

### Phase 8 --- Explorer

**Status:** PLANNED

### Phase 9 --- Smart Contracts

**Status:** PLANNED

### Phase 10 --- Mainnet

**Status:** HISTORICAL ROADMAP TARGET / NOT READY

The historical Phase 10 remains preserved; the production transition
below is now the active strategic path.

## Production transition

### Phase 11 --- Protocol Formalization

**Status:** CURRENT / IN PROGRESS

-   architecture baseline
-   protocol baseline
-   protocol gaps
-   authoritative protocol specification
-   ADR foundation
-   versioning model

### Phase 12 --- Go Production Core

**Status:** PLANNED

Production Go implementation of protocol primitives through node
lifecycle.

### Phase 13 --- Cross-Language Compatibility

**Status:** PLANNED

Python/Go deterministic vectors and compatibility harness.

### Phase 14 --- Go Private Testnet

**Status:** PLANNED

Multi-node private SYJ testnet using the Go production core.

### Phase 15 --- Adversarial Security Testing

**Status:** PLANNED

Fault injection, malformed inputs, resource exhaustion, replay, peer
attacks and consensus abuse.

### Phase 16 --- Public SYJ Testnet

**Status:** PLANNED

Public testnet after private-testnet and adversarial gates pass.

### Phase 17 --- Mainnet Readiness

**Status:** PLANNED

External review, operational readiness, upgrade policy, security
evidence and final genesis governance.

### Phase 18 --- SYJ Mainnet

**Status:** PLANNED

Mainnet only after all formal readiness gates pass.

## State labels

-   COMPLETED
-   CURRENT
-   IN PROGRESS
-   PLANNED
-   DEFERRED
-   ARCHITECTED FOR

## Rule

The historical `ROADMAP.md` is preserved. This file is the master
strategic roadmap for the Python-reference → Go-production transition.
