# Development Roadmap — SAYANJALI BLOCKCHAIN

This roadmap tracks SAYANJALI BLOCKCHAIN's path from the current MVP toward
a production-grade Layer-1 network powering the SYJ Token ecosystem.

## Phase 1 — Blockchain Core ✅ (MVP complete)
- Block model, hashing, Merkle root, genesis block
- Deliverable: `blockchain/block.py`

## Phase 2 — Wallets ✅ (MVP complete)
- ECDSA (SECP256k1) key generation, address derivation, import/export
- Deliverable: `blockchain/wallet.py`

## Phase 3 — Transactions ✅ (MVP complete)
- Signed transactions, verification, coinbase transactions
- Deliverable: `blockchain/transaction.py`

## Phase 4 — Mining ✅ (MVP complete)
- Proof-of-Work consensus, mempool, block assembly, reward distribution
- Deliverable: `blockchain/consensus.py`, `blockchain/mempool.py`, `blockchain/mining.py`

## Phase 5 — REST API & CLI ✅ (MVP complete)
- Full FastAPI surface, Typer CLI, persistent SQLite storage
- Deliverable: `api/`, `cli/`, `blockchain/storage.py`

## Phase 6 — Networking (P2P) ✅ (complete)
- HTTP-based peer registration and discovery, served under `/network/*`
  on the same FastAPI application as the REST API
- Node identity persisted across restarts (`blockchain/network/node.py`)
- Chain synchronization: fetch, validate, and adopt a peer's chain using
  **accumulated proof-of-work** (not raw length) as the adoption
  criterion, with reorg-safe persistence (`Blockchain.replace_chain`,
  `Storage.reorganize_from`)
- Block and transaction propagation with duplicate/invalid rejection and
  basic loop prevention
- Network-aware CLI commands: `network-status`, `peers`, `add-peer`, `sync`
- Milestone achieved: two independent node processes, started and
  connected over real HTTP, converge on the same chain in both directions
  (see `tests/test_multi_node_integration.py`)

## Phase 6.5 — Networking Hardening (deferred from Phase 6)
Phase 6 delivered a working HTTP-based P2P prototype, not a production
peer-to-peer protocol. Explicitly deferred:
- A real gossip/anti-entropy protocol (Phase 6 uses direct request/response
  propagation to known peers with a bounded seen-hash cache, not multi-hop
  gossip)
- Node handshake / version / protocol negotiation
- Peer authentication (any node can currently register as a peer)
- Client-side transaction signing (removing private-key transmission to
  `/transaction/sign` entirely)
- A rate limiter shared across a deployment, rather than per-process
  in-memory only

## Phase 7 — Consensus Improvements ✅ (difficulty retarget complete)
- Full Bitcoin-style difficulty retarget: ratio-based, using exact
  integer/rational arithmetic (`fractions.Fraction`, never floating
  point), with explicit configurable minimum/maximum difficulty bounds
  and an adjustment-factor clamp bounding how much any single retarget
  window can move difficulty in one step -- replacing the previous
  MVP's flat ±1 nudge, which adjusted relative to a static config value
  rather than the chain's actual current difficulty
- Proof-of-Stake / Delegated Proof-of-Stake design work: **not started**.
  This was explicitly marked optional in this phase, and was not
  undertaken -- `FutureConsensusOptions`' placeholder fields in
  `config/settings.py` remain exactly as they were, reserved for when
  that work begins
- Milestone achieved: `tests/test_consensus_retarget.py` demonstrates
  difficulty responding proportionally to sustained fast or slow mining
  across multiple retarget windows, converging toward the target block
  time rather than either staying fixed or overshooting via a fixed-size
  nudge

## Phase 8 — Explorer
- Public/local block and transaction explorer (web UI)
- Explorer-specific read API (address history, rich list, search by hash)
- Milestone: browse any block/transaction/address without the CLI

## Phase 9 — Smart Contracts
- WebAssembly or minimal custom VM for contract execution
- Contract deployment and call transactions
- Gas/fee metering model
- Milestone: deploy and call a simple contract (e.g. a counter or token)

## Phase 10 — Mainnet
- Security audit of consensus, wallet, and API layers
- Multi-signature wallet support
- Governance mechanism (parameter changes, upgrades)
- Production deployment topology (multiple validator/full nodes)
- Milestone: SAYANJALI BLOCKCHAIN mainnet genesis block, SYJ Token live

## Longer-term / architected-for-but-not-scheduled
These are explicitly designed for in the current architecture (modular
consensus interface, storage abstraction, versioned API) but not scheduled
into a specific phase yet:
- Cross-chain bridges
- Layer-2 scaling / rollups
- Sharding
- SDK for third-party developers
- Mobile and web wallet apps
- Enterprise integrations (SAYANJALI NEXUS product suite interop)
- NFT / token standards
- DAO governance tooling

## How phases map to this repository's history

Phases 1–5 shipped as v0.1.0-mvp: a single-node blockchain (37 passing
tests across wallets, transactions, blocks, mining, chain validation, and
the REST API). Phase 6 shipped next: HTTP-based P2P networking and
multi-node synchronization, bringing the suite to 92 passing tests,
including a real two-process integration test proving independent nodes
converge on the same chain over actual HTTP. Phase 6.5 hardened that
networking layer -- peer authentication, a challenge-response handshake,
SSRF-resistant address validation, bounded rate limiting, request body
size limits, replay protection, and removal of server-side private-key
handling -- bringing the suite to 166 passing tests, including an
authenticated real two-process integration run. Phase 7 replaced the
difficulty retarget algorithm with a Bitcoin-style ratio-based one,
bringing the suite to 195 passing tests. Proof-of-Stake/DPoS design work
(optional within Phase 7) and Phases 8–10 remain architected for but not
yet implemented, per the project's scope: build each layer as an
additive module on a clean core, not a rewrite.

**A note on terminology:** Phase 6 makes this a networked, multi-node
*prototype* -- nodes communicate, propagate, and converge. It is not
distributed consensus in the Byzantine-fault-tolerant sense, and it is not
a production peer-to-peer network. See ROADMAP.md's Phase 6.5 and the
README's Security section for exactly what that distinction means in
practice.
