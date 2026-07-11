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

## Phase 6 — Networking (P2P)
- Peer discovery and gossip protocol for transactions and blocks
- Longest-valid-chain conflict resolution across peers (the `replace_chain`
  hook already exists in `blockchain/blockchain.py` for this)
- Node handshake / version negotiation
- Milestone: two independent nodes stay in sync over a LAN

## Phase 7 — Consensus Improvements
- Full Bitcoin-style difficulty retarget (ratio-based, not the current
  MVP's conservative ±1 nudge)
- Optional: begin Proof-of-Stake or Delegated Proof-of-Stake design work
  (config placeholders already exist in `config/settings.py`'s
  `FutureConsensusOptions`)
- Milestone: difficulty stabilizes block time under varying network hash rate

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

## How phases map to this MVP delivery

Phases 1–5 are the complete, tested MVP delivered in this repository (37
passing tests across wallets, transactions, blocks, mining, chain
validation, and the full REST API). Phases 6–10 are architected for but
intentionally not implemented yet, per the project's scope: build a clean,
modular MVP now; expand into a real network later without a rewrite.
