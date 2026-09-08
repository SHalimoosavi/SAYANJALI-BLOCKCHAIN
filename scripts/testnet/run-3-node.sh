#!/bin/sh
set -eu

ROOT=${1:-./.phase6-testnet}
BIN="$ROOT/syjd"

PYTHON_BIN=${PYTHON_BIN:-python3}

if ! command -v "$PYTHON_BIN" >/dev/null 2>&1; then
    echo "ERROR: python3 is required for CSPRNG token generation" >&2
    exit 1
fi

if ! "$PYTHON_BIN" -c 'import secrets' >/dev/null 2>&1; then
    echo "ERROR: Python secrets module is unavailable" >&2
    exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
    echo "ERROR: curl is required" >&2
    exit 1
fi

umask 077

# This launcher owns the default isolated runtime directory.
# Refuse to operate on an existing runtime so cleanup cannot accidentally
# remove unrelated user data.
if [ -e "$ROOT" ]; then
    echo "ERROR: runtime directory already exists: $ROOT" >&2
    echo "Remove it first if it is a stale SAYANJALI testnet runtime." >&2
    exit 1
fi

mkdir -p "$ROOT/node-a" "$ROOT/node-b" "$ROOT/node-c" "$ROOT/logs"

cleanup() {
    set +e

    if [ -f "$ROOT/a.pid" ]; then
        "$BIN" stop "$ROOT/node-a" >/dev/null 2>&1 || true
    fi

    if [ -f "$ROOT/b.pid" ]; then
        "$BIN" stop "$ROOT/node-b" >/dev/null 2>&1 || true
    fi

    if [ -f "$ROOT/c.pid" ]; then
        "$BIN" stop "$ROOT/node-c" >/dev/null 2>&1 || true
    fi

    for pid_file in "$ROOT/a.pid" "$ROOT/b.pid" "$ROOT/c.pid"; do
        if [ -f "$pid_file" ]; then
            pid=$(cat "$pid_file" 2>/dev/null || true)
            if [ -n "$pid" ]; then
                kill "$pid" 2>/dev/null || true
                wait "$pid" 2>/dev/null || true
            fi
        fi
    done

    rm -rf "$ROOT"
}

trap cleanup EXIT INT TERM HUP

generate_token() {
    "$PYTHON_BIN" -c 'import secrets; print(secrets.token_hex(32))'
}

TOKEN_A=$(generate_token)
TOKEN_B=$(generate_token)
TOKEN_C=$(generate_token)

if [ -z "$TOKEN_A" ] || [ -z "$TOKEN_B" ] || [ -z "$TOKEN_C" ]; then
    echo "ERROR: failed to generate API authentication credentials" >&2
    exit 1
fi

if [ "$TOKEN_A" = "$TOKEN_B" ] || [ "$TOKEN_A" = "$TOKEN_C" ] || [ "$TOKEN_B" = "$TOKEN_C" ]; then
    echo "ERROR: generated API credentials are not unique" >&2
    exit 1
fi

go build -o "$BIN" ./cmd/syjd

cat > "$ROOT/node-a/config.json" <<JSON
{"data_dir":"$ROOT/node-a","network_name":"sayanjali-mainnet-mvp","listen_address":"127.0.0.1:30301","advertised_address":"127.0.0.1:30301","seeds":["127.0.0.1:30302","127.0.0.1:30303"],"max_peers":8,"api_listen_address":"127.0.0.1:18080","api_auth_token":"$TOKEN_A","mempool_max":1000,"log_level":"INFO"}
JSON

cat > "$ROOT/node-b/config.json" <<JSON
{"data_dir":"$ROOT/node-b","network_name":"sayanjali-mainnet-mvp","listen_address":"127.0.0.1:30302","advertised_address":"127.0.0.1:30302","seeds":["127.0.0.1:30301","127.0.0.1:30303"],"max_peers":8,"api_listen_address":"127.0.0.1:18081","api_auth_token":"$TOKEN_B","mempool_max":1000,"log_level":"INFO"}
JSON

cat > "$ROOT/node-c/config.json" <<JSON
{"data_dir":"$ROOT/node-c","network_name":"sayanjali-mainnet-mvp","listen_address":"127.0.0.1:30303","advertised_address":"127.0.0.1:30303","seeds":["127.0.0.1:30301","127.0.0.1:30302"],"max_peers":8,"api_listen_address":"127.0.0.1:18082","api_auth_token":"$TOKEN_C","mempool_max":1000,"log_level":"INFO"}
JSON

chmod 600 "$ROOT/node-a/config.json" \
           "$ROOT/node-b/config.json" \
           "$ROOT/node-c/config.json"

"$BIN" start "$ROOT/node-a" >"$ROOT/logs/a.log" 2>&1 &
echo $! >"$ROOT/a.pid"

"$BIN" start "$ROOT/node-b" >"$ROOT/logs/b.log" 2>&1 &
echo $! >"$ROOT/b.pid"

"$BIN" start "$ROOT/node-c" >"$ROOT/logs/c.log" 2>&1 &
echo $! >"$ROOT/c.pid"

wait_for_health() {
    url=$1
    i=0

    while [ "$i" -lt 60 ]; do
        status=$(curl -sS -o /dev/null -w '%{http_code}' "$url/health" 2>/dev/null || true)
        if [ "$status" = "200" ]; then
            return 0
        fi
        sleep 1
        i=$((i + 1))
    done

    echo "ERROR: node failed health check: $url" >&2
    return 1
}

wait_for_health "http://127.0.0.1:18080"
wait_for_health "http://127.0.0.1:18081"
wait_for_health "http://127.0.0.1:18082"

api_status() {
    token=$1
    method=$2
    url=$3

    curl -sS \
        -o /dev/null \
        -w '%{http_code}' \
        -X "$method" \
        -H "Authorization: Bearer $token" \
        "$url"
}

assert_status() {
    expected=$1
    actual=$2
    description=$3

    if [ "$actual" != "$expected" ]; then
        echo "ERROR: $description: expected HTTP $expected, got HTTP $actual" >&2
        return 1
    fi
}

# Authentication gate checks.
assert_status 401 \
    "$(curl -sS -o /dev/null -w '%{http_code}' -X POST http://127.0.0.1:18080/mine)" \
    "missing /mine authorization"

assert_status 401 \
    "$(curl -sS -o /dev/null -w '%{http_code}' -X POST \
        -H 'Authorization: Bearer invalid-test-token' \
        http://127.0.0.1:18080/mine)" \
    "incorrect /mine authorization"

assert_status 401 \
    "$(api_status "$TOKEN_B" POST http://127.0.0.1:18080/mine)" \
    "cross-node /mine authorization"

assert_status 201 \
    "$(api_status "$TOKEN_A" POST http://127.0.0.1:18080/mine)" \
    "correct /mine authorization"

assert_status 401 \
    "$(curl -sS -o /dev/null -w '%{http_code}' -X POST http://127.0.0.1:18080/shutdown)" \
    "missing /shutdown authorization"

assert_status 401 \
    "$(api_status "$TOKEN_B" POST http://127.0.0.1:18080/shutdown)" \
    "cross-node /shutdown authorization"

assert_status 401 \
    "$(curl -sS -o /dev/null -w '%{http_code}' -X POST http://127.0.0.1:18080/transactions)" \
    "missing POST /transactions authorization"

assert_status 401 \
    "$(curl -sS -o /dev/null -w '%{http_code}' -X POST \
        -H 'Authorization: Bearer invalid-test-token' \
        http://127.0.0.1:18080/transactions)" \
    "incorrect POST /transactions authorization"

# A correctly authenticated request reaches the transaction handler.
# An empty body is intentionally used here because constructing a funded,
# cryptographically signed transaction is outside this launcher.
assert_status 400 \
    "$(curl -sS -o /dev/null -w '%{http_code}' -X POST \
        -H "Authorization: Bearer $TOKEN_A" \
        -H 'Content-Type: application/json' \
        --data '{}' \
        http://127.0.0.1:18080/transactions)" \
    "correct POST /transactions authentication gate"

echo "3-node testnet started successfully."
echo "API endpoints: 18080, 18081, 18082"
echo "P2P endpoints: 30301, 30302, 30303"
echo "Authentication smoke tests: PASS"
echo "Tokens are intentionally not printed."
echo "Press Ctrl-C to stop the testnet and remove its runtime state."

wait
