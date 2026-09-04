# SAYANJALI BLOCKCHAIN --- Architecture Baseline

**Audit checkpoint:** Phase 3 testnet foundation\
**Audited Git commit:** `ead5330986debbc6420bdd1d392d81051eac8f22`\
**Branch:** `phase-3-testnet-foundation`\
**Phase 2 parent:** `d24e3b4612007be98680288e15e41d9565068acd`

## 1. Architecture verified

The repository is a Python/FastAPI Layer-1 reference/prototype
implementation.

### Layers

1.  **Client interfaces**
    -   `api/`
    -   `cli/`
2.  **Networking**
    -   `blockchain/network/`
    -   peer registry, identity, authentication, discovery, propagation,
        synchronization, rate limiting and lifecycle
3.  **Application orchestration**
    -   `blockchain/blockchain.py`
    -   single coordinator for chain state, consensus, mempool, mining
        and storage
4.  **Core engines**
    -   `blockchain/consensus.py`
    -   `blockchain/mempool.py`
    -   `blockchain/mining.py`
    -   `blockchain/storage.py`
    -   `blockchain/validators.py`
5.  **Protocol/data primitives**
    -   `blockchain/block.py`
    -   `blockchain/transaction.py`
    -   `blockchain/native_asset.py`
    -   `blockchain/wallet.py`
    -   `blockchain/utils.py`
6.  **Configuration**
    -   `config/settings.py`

## 2. Dependency direction

The implementation follows the intended dependency direction:

`API / CLI / P2P / Mining -> Blockchain facade -> Validation / Consensus / Mempool / State-in-storage`

P2P routes delegate validation and state mutation to the existing
blockchain layer. RPC/CLI do not contain independent consensus rules.

## 3. Runtime architecture

A node is one FastAPI process. REST and P2P HTTP endpoints share the
same listener and port.

`NetworkNode` owns: - persistent node identity - P2P identity keypair -
peer registry - replay cache - lifecycle state - propagation counters -
network maintenance task

SQLite is the default database, accessed through SQLAlchemy Core.

## 4. Python reference boundary

The following modules are designated as the Python protocol
reference/oracle until a formally approved protocol change supersedes
them:

-   `blockchain/block.py`
-   `blockchain/transaction.py`
-   `blockchain/validators.py`
-   `blockchain/consensus.py`
-   `blockchain/blockchain.py`
-   `blockchain/mining.py`
-   `blockchain/mempool.py`
-   `blockchain/native_asset.py`
-   `blockchain/wallet.py`
-   `blockchain/utils.py`
-   `blockchain/network/*`
-   `config/settings.py`

The reference designation does not imply production readiness. It means
the Go implementation must reproduce defined behavior before
deliberately changing it.

## 5. Phase 3 additions

Phase 3 added: - explicit node lifecycle - FastAPI lifecycle ownership -
peer failure backoff and health state - capability tracking - bootstrap
registration/authentication - transitive peer discovery - explicit P2P
message types - propagation counters - synchronization attempt
tracking - three-process local testnet coverage

## 6. Existing security boundary

Phase 6.5 security work remains part of the reference: - SECP256k1 P2P
identity keys - challenge-response trust establishment - signed
per-request envelopes - nonce/timestamp replay controls - SSRF-resistant
peer address validation - request-body size limits - bounded rate
limiting - trusted-peer gating for block/transaction propagation

Known limitation: DNS validation is best-effort and does not pin the
resolved IP to the subsequent connection.

## 7. Architectural risks

High priority: - protocol rules are distributed across code/config
rather than one normative specification; - no cross-language
deterministic vector contract exists yet; - no `.github/workflows/` CI
directory exists at this checkpoint; - P2P transport is HTTP rather than
a production framed transport; - synchronization is full-chain rather
than incremental header/block synchronization; - block timestamp
validation has no future-time/median-time rule; - transactions have no
protocol nonce/account-sequence field; - wallet creation API returns
private key material; - runtime peer health/capabilities are not
persisted; - implementation version strings are stale.

Medium priority: - `max_peer_failures` is configured but not used as a
hard failure-count ceiling; - rate limiting is process-local; - the
README/test badges are inconsistent with the verified Phase 3 test
baseline; - the current network protocol has only one supported P2P
version (`1.0`).

## 8. Target transition

`Protocol specification -> Python oracle -> deterministic vectors -> Go implementation -> compatibility gate -> private testnet -> adversarial testing -> public testnet -> mainnet readiness`

The Python implementation remains in the repository and is not to be
rewritten merely for stylistic reasons.
