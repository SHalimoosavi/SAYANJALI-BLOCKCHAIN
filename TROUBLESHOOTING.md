# Troubleshooting Guide — SAYANJALI BLOCKCHAIN

## "Chain is INVALID" after pulling in an old database file

The `difficulty` field is part of every block's hashed header. If you have
a database file created before this field existed in your local build,
blocks mined under an older schema will fail hash verification against the
current code. Delete `database/*.db` and let the chain rebuild from genesis,
or keep a versioned backup of old databases alongside the code version that
produced them.

## `WalletError: Invalid private key`

The private key must be a 64-character hex string (32 raw bytes for
SECP256k1). Common causes: pasting a truncated key, extra whitespace/newline
characters copied from a terminal, or accidentally passing a public key
instead.

## Transaction rejected with "Insufficient balance"

`Blockchain.submit_transaction` checks the sender's *confirmed* balance
(from mined blocks), not pending mempool transactions. If you've just mined
a reward to that address, the balance should already reflect it — check
`GET /wallet/{address}` or `python -m cli.main status`. If you sent multiple
transactions from the same address before mining a block, only the first
ones your confirmed balance can cover will be accepted; the rest need a
mined block first to release new confirmed balance.

## Transaction rejected with signature verification failure

This usually means one of:
- The `timestamp` sent to `/transaction/sign` doesn't exactly match the one
  returned by `/transaction/create` (the signature covers the timestamp).
- The `sender` address doesn't match the address derived from the supplied
  `private_key`.
- The transaction was mutated after signing (e.g. amount changed) before
  submission.

## `sqlite3.OperationalError: database is locked`

SQLite allows only one writer at a time. This can happen if you run the CLI
and the API server against the same database file simultaneously while both
attempt to mine/write at once. For MVP use, prefer running one process at a
time against a given database file, or point them at different
`SYJ_DB_FILE` values.

## Mining seems to hang

Proof-of-work mining time grows exponentially with difficulty. If you set
`SYJ_DIFFICULTY` too high for your device (e.g. 6+ on a phone CPU), mining a
single block can take a very long time. Lower it via the environment
variable and restart:

```bash
export SYJ_DIFFICULTY=3
```

## Port already in use when starting the API

```bash
lsof -i :8000        # find the process (may not be available on Termux)
pkill -f "uvicorn api.main:app"
```

Or just run on a different port:

```bash
uvicorn api.main:app --host 0.0.0.0 --port 8001
```

## `ModuleNotFoundError: No module named 'blockchain'` / `'config'` / `'api'`

These are run as packages relative to the project root. Always run commands
from the project's top-level directory (`sayanjali-blockchain/`), and use
the `-m` flag:

```bash
python -m api.main
python -m cli.main status
python -m pytest tests/
```

Running `python api/main.py` directly (without `-m`) breaks the internal
`from config.settings import ...` imports.

## Tests fail with database-related errors

Confirm you're running `pytest` from the project root, and that
`tests/conftest.py` is present and being picked up (pytest auto-discovers
`conftest.py` — no import needed). If tests still touch your real database,
check that `SYJ_DB_FILE` isn't hardcoded/exported globally in your shell
profile, which would override the per-test isolation fixture.

## Genesis block hash differs between two machines/clones

The genesis block is deterministic (fixed timestamp, nonce, and message in
`config/settings.py`'s `GenesisConfig`), so two unmodified clones of this
repo should always produce an identical genesis hash. If they differ, check
whether `GenesisConfig` was edited locally, or whether an environment
variable is overriding a genesis-related setting.
