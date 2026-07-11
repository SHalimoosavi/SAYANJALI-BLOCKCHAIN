# Termux Deployment Guide — SAYANJALI BLOCKCHAIN

This guide covers building and running SAYANJALI BLOCKCHAIN entirely on an
Android phone using Termux, matching the same Termux-first workflow used for
SYJ Mail Intelligence AI and the other SAYANJALI NEXUS Termux tools.

## 1. Install Termux packages

```bash
pkg update -y && pkg upgrade -y

pkg install -y git python clang openssl-tool rust binutils
```

Notes:
- `clang` and `rust` are needed because `cryptography` and `ecdsa` sometimes
  build native wheels on ARM/Android if a prebuilt wheel isn't available for
  your Termux Python version. If pip installs cleanly without them, you can
  skip `clang`/`rust`, but keeping them installed avoids surprises on
  Termux updates.
- `openssl-tool` provides the OpenSSL headers `cryptography` links against.

## 2. Grant storage access (optional, only if you want files under /sdcard)

```bash
termux-setup-storage
```

This project keeps its database and logs inside its own project folder
(`database/`, `logs/`), so this step is optional unless you plan to export
wallet backups to shared storage.

## 3. Clone or copy the project

```bash
cd ~
git clone https://github.com/<your-username>/sayanjali-blockchain.git
cd sayanjali-blockchain
```

(Or `termux-setup-storage` + copy the folder in via a file manager if you
built it elsewhere and are transferring it.)

## 4. Create and activate a virtual environment

```bash
python -m venv venv
source venv/bin/activate
```

You'll need to run `source venv/bin/activate` at the start of every new
Termux session before using the CLI or API.

## 5. Install requirements

```bash
pip install --upgrade pip
pip install -r requirements.txt
```

If `cryptography` fails to build a wheel:

```bash
pip install cryptography --no-binary :all:
```

This forces a source build using the `clang`/`rust`/`openssl-tool` packages
installed in step 1. It's slower (a few minutes) but reliable on Termux.

## 6. Run the API server

```bash
python -m api.main
```

Or via uvicorn directly, with auto-reload disabled (reload is unstable on
Termux's filesystem watchers):

```bash
uvicorn api.main:app --host 0.0.0.0 --port 8000
```

Visit `http://127.0.0.1:8000/docs` in a mobile browser for interactive
Swagger API docs, or `http://<phone-lan-ip>:8000/docs` from another device
on the same Wi-Fi network.

## 7. Run the CLI

```bash
python -m cli.main --help
python -m cli.main create-wallet
python -m cli.main status
```

## 8. Run tests

```bash
pytest -v
```

## 9. Common Termux issues

**`ModuleNotFoundError` after activating venv**
Confirm the venv is actually active — the prompt should show `(venv)`.
Re-run `source venv/bin/activate`.

**`error: command 'clang' failed` while installing `cryptography`**
Run `pkg install clang openssl-tool` and retry. Ensure `pkg update` was run
recently — Termux's package index can be stale after long gaps between uses.

**`sqlite3.OperationalError: unable to open database file`**
The `database/` folder must exist and be writable. It's created
automatically by `config/settings.py` on first run, but if you've moved the
project, confirm you have write permission in Termux's home directory
(`~/sayanjali-blockchain`), not a read-only shared storage path.

**Port already in use**
Termux keeps background processes alive across app switches. Find and stop
a stale server with:

```bash
pkill -f "uvicorn api.main:app"
```

**Battery optimization killing the server**
If running the node long-term (e.g. as a background miner), disable battery
optimization for Termux in Android Settings → Apps → Termux → Battery, and
consider running via `termux-wake-lock` to prevent Android from suspending
the process:

```bash
termux-wake-lock
python -m api.main
```

## 10. Performance recommendations

- Keep `SYJ_DIFFICULTY` low (2–4) for a phone-only MVP node — Proof of Work
  mining is CPU-bound and phone CPUs throttle under sustained load.
- Avoid running mining and the API server as two separate heavy processes
  simultaneously on a single low-end device; mine in short bursts via the
  CLI rather than continuously.
- SQLite is fine for MVP data volumes. If the chain grows large enough that
  queries feel slow, that's the signal to migrate `SYJ_DB_FILE` /
  `database_url` to a PostgreSQL instance (the storage layer already
  supports this by URL alone — see `config/settings.py`).
