# SAYANJALI BLOCKCHAIN

**A native Layer-1 blockchain MVP powering the SYJ Token, built by
SAYANJALI NEXUS PRIVATE LIMITED.**

SAYANJALI BLOCKCHAIN is not an ERC-20/BEP-20 token deployed on an existing
chain — it is an independent blockchain implementation, currently at MVP
stage, architected to grow into a production-grade Layer-1 network. This
repository contains a clean, fully tested, modular foundation: blockchain
core, wallets, signed transactions, Proof-of-Work mining, persistent
storage, a REST API, and a CLI — all buildable and runnable entirely from
an Android phone via Termux.

## Status

**MVP — Phases 1 through 5 complete.** 37 automated tests passing across
wallets, transactions, blocks, mining, chain validation, and the full REST
API. See [ROADMAP.md](ROADMAP.md) for what's next.

## Project Overview

SAYANJALI BLOCKCHAIN implements the core primitives every Layer-1 network
needs — a chain of cryptographically linked blocks, wallets, signed
transactions, a Proof-of-Work consensus engine, a mempool, persistent
storage, and both a REST API and a CLI for operating a node — while keeping
every layer modular enough that P2P networking, alternate consensus
algorithms, and smart contracts can be added later without restructuring
what's already here.

## Features

| Capability | Status |
|---|---|
| Block model, hashing, Merkle root, genesis | ✅ |
| ECDSA (SECP256k1) wallets | ✅ |
| Signed, verifiable transactions | ✅ |
| Proof-of-Work consensus + mining rewards | ✅ |
| Mempool with duplicate prevention | ✅ |
| Persistent storage (SQLite via SQLAlchemy) | ✅ |
| REST API (FastAPI) | ✅ |
| CLI (Typer + Rich) | ✅ |
| Structured logging | ✅ |
| Test suite (pytest) | ✅ |
| P2P networking | 🔜 Phase 6 |
| Smart contracts | 🔜 Phase 9 |

## Architecture

```
┌─────────────┐     ┌─────────────┐
│   cli/      │     │   api/      │   ← application layer (CLI, REST API)
└──────┬──────┘     └──────┬──────┘
       │                   │
       └─────────┬─────────┘
                  │
          ┌───────▼────────┐
          │  Blockchain     │        ← orchestrator (blockchain/blockchain.py)
          │  (facade)       │
          └───────┬────────┘
                   │
   ┌───────────────┼───────────────┬────────────────┐
   │               │               │                │
┌──▼───┐      ┌────▼────┐    ┌─────▼─────┐    ┌─────▼─────┐
│Block │      │Consensus│    │  Mempool  │    │  Storage  │
│model │      │ (PoW)   │    │           │    │(SQLAlchemy)│
└──────┘      └─────────┘    └───────────┘    └───────────┘
```

### Key design decisions

- **Consensus is pluggable.** `blockchain/consensus.py` defines an abstract
  `ConsensusEngine` interface. Proof of Work is the only implementation
  today, but Proof of Stake / Delegated Proof of Stake can be added as new
  classes without touching `Blockchain` or any other module.
- **Storage is swappable.** `blockchain/storage.py` is built on SQLAlchemy
  Core, not raw `sqlite3`. Moving from SQLite to PostgreSQL later is a
  one-line change to `database_url` in `config/settings.py` — no code
  changes required anywhere else.
- **Block hashing is header-only.** A block's hash covers only its header
  fields (index, previous_hash, timestamp, nonce, difficulty, merkle_root),
  not the full transaction list. This keeps mining cheap regardless of
  block size, while the Merkle root still guarantees transaction integrity.
- **Difficulty is recorded per block, not assumed globally.** Each block
  stores the difficulty it was actually mined at (and that value is part of
  the hashed header, so it can't be tampered with independently). This
  allows the network's difficulty to legitimately retarget over time
  without invalidating historical blocks.
- **API and CLI share one core.** Both `api/routes.py` and `cli/main.py`
  call into the same `Blockchain` facade — there is exactly one source of
  truth for chain logic.

## Folder Structure

```
sayanjali-blockchain/
├── blockchain/
│   ├── block.py          # Block model, hashing, Merkle root, genesis
│   ├── blockchain.py     # Orchestrator: chain state, mining, validation
│   ├── consensus.py      # Pluggable consensus engine (PoW implemented)
│   ├── mempool.py        # Pending transaction pool
│   ├── mining.py         # Coinbase + block assembly for mining
│   ├── storage.py        # SQLAlchemy-backed persistence
│   ├── transaction.py    # Transaction model, signing, verification
│   ├── utils.py          # Hashing, logging, shared exceptions
│   ├── validators.py     # Block/chain/transaction validation rules
│   └── wallet.py         # ECDSA key generation, addresses, signing
├── api/
│   ├── main.py           # FastAPI app entrypoint
│   ├── routes.py         # All REST endpoints
│   └── schemas.py        # Pydantic request/response models
├── cli/
│   └── main.py           # Typer CLI commands
├── config/
│   └── settings.py       # Central configuration (env-var driven)
├── database/              # SQLite file lives here (gitignored)
├── logs/                  # Rotating log files (gitignored)
├── tests/                 # pytest suite (37 tests)
├── requirements.txt
├── README.md
├── LICENSE
├── .gitignore
├── SETUP_TERMUX.md
├── GITHUB_WORKFLOW.md
├── TESTING.md
├── TROUBLESHOOTING.md
└── ROADMAP.md
```

## Installation

```bash
git clone https://github.com/<your-username>/sayanjali-blockchain.git
cd sayanjali-blockchain
python -m venv venv
source venv/bin/activate      # Windows: venv\Scripts\activate
pip install -r requirements.txt
```

Building on Android/Termux? See [SETUP_TERMUX.md](SETUP_TERMUX.md) for the
complete phone-only setup guide.

## Termux Setup

The condensed version, for anyone building entirely on an Android phone:

```bash
pkg update -y && pkg upgrade -y
pkg install -y git python clang openssl-tool rust binutils

git clone https://github.com/<your-username>/sayanjali-blockchain.git
cd sayanjali-blockchain

python -m venv venv
source venv/bin/activate
pip install --upgrade pip
pip install -r requirements.txt

python -m cli.main status
```

If `cryptography` fails to build a wheel on your device, force a source
build: `pip install cryptography --no-binary :all:`. Full troubleshooting,
`termux-wake-lock` notes, and performance recommendations for low-end
devices live in [SETUP_TERMUX.md](SETUP_TERMUX.md).

## Quick Start

A minimal end-to-end flow — create a wallet, mine a block, check the
balance, validate the chain:

```bash
source venv/bin/activate

python -m cli.main create-wallet
# copy the printed Address

python -m cli.main mine <address-from-above>
python -m cli.main status
python -m cli.main validate
```

Or the same flow against a running API server:

```bash
python -m api.main &

curl -s -X POST http://127.0.0.1:8000/wallet/create
curl -s -X POST http://127.0.0.1:8000/mine \
  -H "Content-Type: application/json" \
  -d '{"miner_address": "<address-from-above>"}'
curl -s http://127.0.0.1:8000/status
```

## Usage

### Start the node (REST API)

```bash
python -m api.main
# or
uvicorn api.main:app --host 0.0.0.0 --port 8000
```

Interactive API docs: `http://127.0.0.1:8000/docs`

### Use the CLI

```bash
python -m cli.main create-wallet
python -m cli.main mine <your-address>
python -m cli.main show-chain
python -m cli.main status
python -m cli.main validate
```

## API Reference

| Method | Endpoint | Description |
|---|---|---|
| GET | `/health` | Liveness probe |
| GET | `/status` | Node/chain status summary |
| GET | `/chain` | Full blockchain |
| GET | `/block/{index}` | Single block by index |
| POST | `/wallet/create` | Generate a new wallet |
| GET | `/wallet/{address}` | Confirmed balance for an address |
| POST | `/transaction/create` | Build an unsigned transaction envelope |
| POST | `/transaction/sign` | Sign a transaction with a private key |
| POST | `/transaction/submit` | Submit a signed transaction to the mempool |
| GET | `/transactions/pending` | List pending mempool transactions |
| POST | `/mine` | Mine a new block, rewarding a miner address |

Full request/response schemas are documented automatically at `/docs`
(Swagger UI) and `/redoc`.

## CLI Reference

| Command | Description |
|---|---|
| `create-wallet` | Generate a new wallet and print its keys/address |
| `show-chain` | Print a summary table of every block |
| `mine <address>` | Mine a new block, rewarding `<address>` |
| `status` | Print node/chain status |
| `create-transaction` | Build, sign, and submit a transaction interactively |
| `validate` | Validate the entire chain |
| `start-node` | Start the FastAPI node via uvicorn |

## Screenshots

*(Add CLI and Swagger UI screenshots here once available — placeholders
below.)*

- `docs/screenshots/cli-show-chain.png`
- `docs/screenshots/api-swagger-docs.png`
- `docs/screenshots/cli-mining.png`

## Testing

```bash
pytest -v
```

37 tests cover wallets, transactions, blocks, mining, chain validation, and
the full REST API (via FastAPI's `TestClient`), all running against
isolated temporary SQLite databases so your real chain in `database/` is
never touched. See [TESTING.md](TESTING.md) for what each test file
covers, coverage reporting, and the manual smoke-test checklist.

## GitHub Workflow

Branching (`main` / `dev` / `feature/*`), commit message conventions,
release tagging, and Termux-specific push/pull/PAT authentication notes are
all documented in [GITHUB_WORKFLOW.md](GITHUB_WORKFLOW.md).

## Roadmap

See [ROADMAP.md](ROADMAP.md) for the full 10-phase plan from this MVP
through mainnet.

## Security

This is an MVP. Before treating any deployment as production-grade or
holding real value, be aware of its current limitations:

- **No P2P networking yet (Phase 6).** Each node is authoritative over its
  own local chain; there is no peer consensus, so this MVP should not be
  run as a multi-party trust-minimized network yet. `Blockchain.replace_chain`
  already implements the longest-valid-chain rule and is ready for a future
  networking layer to call.
- **Private keys pass through `/transaction/sign`.** This endpoint exists
  for MVP/CLI convenience when the node and wallet are operated by the same
  person on the same device (e.g. a single Termux instance). Do not expose
  this endpoint on a shared or public-facing deployment; sign transactions
  client-side instead once building a real wallet client.
- **Difficulty retargeting is a simplified ±1 nudge**, not a full
  ratio-based Bitcoin-style adjustment — adequate for MVP block-time
  stability, not for adversarial hash-rate conditions (tracked in
  [ROADMAP.md](ROADMAP.md) Phase 7).
- **No rate limiting, authentication, or TLS** on the REST API by default.
  CORS is wide open (`allow_origins=["*"]`) for local development
  convenience — tighten this in `api/main.py` before any non-local
  deployment.
- **Cryptography:** wallets use ECDSA on SECP256k1 (the same curve used by
  Bitcoin and Ethereum) via the well-audited `ecdsa` and `cryptography`
  Python libraries. Addresses are one-way SHA-256 derivations of the public
  key and cannot be reversed to recover it.
- **Responsible disclosure:** if you find a vulnerability in this MVP,
  please report it privately rather than opening a public issue, given the
  project's early stage and real-token ambitions.

## Contribution Guide

This is currently a solo-developer project under SAYANJALI NEXUS PRIVATE
LIMITED. If external contributions open up in the future:

1. Fork the repository.
2. Create a feature branch (`feature/<short-name>`).
3. Ensure `pytest` passes locally (`pytest -v`) before opening a PR.
4. Follow the existing code style: type hints everywhere, PEP 8, complete
   docstrings on public classes/functions.
5. Keep PRs scoped to one logical change.

See [GITHUB_WORKFLOW.md](GITHUB_WORKFLOW.md) for the full Git workflow used
on this project, including Termux-specific push/pull/auth notes.

## License

MIT — see [LICENSE](LICENSE). Note: the license covers source code only,
not the SAYANJALI / SYJ Token / SAYANJALI NEXUS names or trademarks.

## Future Vision

SAYANJALI BLOCKCHAIN's MVP is deliberately scoped to a clean, testable core
so that later phases (P2P networking, smart contracts, staking, bridges,
mobile/web wallets, an explorer, and eventually mainnet) can be built as
additive modules rather than rewrites. Every architectural decision in this
MVP — the pluggable consensus interface, the storage abstraction, the
per-block difficulty recording, the API/CLI sharing one core — was made
with that expansion path in mind.
