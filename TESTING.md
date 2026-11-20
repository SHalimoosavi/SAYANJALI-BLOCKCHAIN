# Testing Guide — SAYANJALI BLOCKCHAIN

## Running the full suite

```bash
pytest -v
```

The authoritative current test matrix is maintained in [docs/TEST_MATRIX.md](docs/TEST_MATRIX.md). Historical test counts from earlier snapshots are not current results.


## Test isolation

`tests/conftest.py` provides an `isolated_settings` autouse fixture that:

- Points `SYJ_DB_FILE` at a fresh, randomly-named SQLite file per test.
- Sets `SYJ_DIFFICULTY=2` so Proof-of-Work mining stays fast in CI/local
  runs (the default production difficulty is higher).
- Clears the `get_settings()` `lru_cache` before and after each test so no
  configuration leaks between tests.

This means running the test suite **never touches or deletes**
`database/sayanjali_chain.db`, your real development chain.

## Running a subset

```bash
pytest tests/test_wallet.py -v
pytest tests/test_mining.py::test_coinbase_reward_credited_to_miner -v
pytest -k "transaction" -v
```

## Coverage (optional)

```bash
pip install pytest-cov --break-system-packages
pytest --cov=blockchain --cov=api --cov-report=term-missing
```

## What each test file covers

| File | Covers |
|---|---|
| `tests/test_wallet.py` | Key generation, address derivation, signing, signature verification |
| `tests/test_transaction.py` | Signing, verification, tamper detection, coinbase rules, serialization |
| `tests/test_block.py` | Hashing, Merkle root, difficulty check, serialization, genesis determinism |
| `tests/test_mining.py` | End-to-end mining, coinbase rewards, mempool inclusion |
| `tests/test_validation.py` | Chain validity, tamper detection at the chain level |
| `tests/test_api.py` | Every REST endpoint, including a full wallet → transaction → mine → balance flow |
| `tests/test_network_*.py` | Peer registry, rate limiting, SSRF/address validation, identity, handshake, propagation, concurrency, network API endpoints |
| `tests/test_multi_node_integration.py` | Real two-process HTTP convergence, authenticated handshake, adversarial rejection scenarios |
| `tests/test_consensus_retarget.py` | Ratio-based difficulty retarget: boundaries, determinism, multi-block progression, adversarial input |
| `tests/test_difficulty_enforcement.py` | Independent difficulty derivation and exact-match enforcement during validation (Phase 1) |
| `tests/test_coinbase_enforcement.py` | Exact coinbase reward enforcement, zero/duplicate coinbase rejection (Phase 1) |
| `tests/test_mempool_pending_spend.py` | Mempool cumulative pending-spend accounting and double-spend prevention (Phase 1) |

## Adding new tests

Any new module under `blockchain/` should get a matching `tests/test_*.py`
file. Favor testing through the public class/function interface (e.g.
`Blockchain.mine_pending_transactions`) over reaching into private state,
so tests stay valid as internals evolve.

## Manual smoke testing

Beyond automated tests, a quick manual pass before a release:

```bash
rm -f database/*.db   # start from a clean chain
python -m cli.main create-wallet
python -m cli.main mine <address-from-above>
python -m cli.main show-chain
python -m cli.main validate
python -m cli.main status
```

Then start the API and hit `http://127.0.0.1:8000/docs` to exercise
endpoints interactively via Swagger UI.

## Historical Python test counts

Earlier counts of 244 and other totals refer to prior snapshots and temporary test environments. They are not current baseline results. See [docs/TEST_MATRIX.md](docs/TEST_MATRIX.md) for the latest verified CI evidence.


## Phase 3 testnet acceptance

Phase 3 adds focused lifecycle/peer-health/protocol tests and a real three-process local testnet scenario. The three-node test verifies transitive discovery, authenticated peer links, transaction propagation through an intermediate node, block propagation, chain-tip/work convergence, balance convergence, and the 720,000,000 SYJ supply ceiling.

The local testnet uses isolated SQLite databases and independent HTTP ports, so it does not modify a developer's chain database.

## Go production-track validation

The exact Go 1.27.1 CI result, race status and coverage status are recorded in [docs/TEST_MATRIX.md](docs/TEST_MATRIX.md). The separate audit workflow generates a race-enabled coverage profile.


The production-track implementation is the Go node under `cmd/syjd` and `internal/`. Run:

```bash
go test ./...
go vet ./...
go build ./...
```

On a host that supports it, also run:

```bash
go test -race ./...
```

`go test -race` is a host capability check, not a consensus requirement. Android/arm64 toolchains may not support the race detector.

## Phase 7 three-node testnet

The authoritative local harness is:

```bash
scripts/testnet/run-3-node.sh
```

It uses `sayanjali-syj-phase7-v1`, the deterministic private-testnet GenesisState fixture, encrypted node identities, per-node API credentials, authenticated mutation endpoints, P2P discovery, propagation, transaction inclusion, convergence, restart, and cleanup. Runtime state is created under `.phase6-testnet/` and removed by the harness on exit.

## Python reference/oracle tests

The Python implementation remains the protocol reference/oracle. It is not the production node. Its test suite is still useful for compatibility and regression coverage, but the Go validation suite is the release gate for the production-track implementation.
