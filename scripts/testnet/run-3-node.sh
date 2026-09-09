#!/bin/sh
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
ROOT=${1:-"$REPO_ROOT/.phase6-testnet"}
BIN="$ROOT/syjd"
GENESIS="$REPO_ROOT/configs/genesis/phase7-private-testnet.state.json"
NETWORK="sayanjali-syj-phase7-v1"
PYTHON_BIN=${PYTHON_BIN:-python3}

command -v "$PYTHON_BIN" >/dev/null 2>&1 || { echo "ERROR: python3 is required" >&2; exit 1; }
"$PYTHON_BIN" -c 'import secrets' >/dev/null 2>&1 || { echo "ERROR: Python secrets module is unavailable" >&2; exit 1; }
command -v curl >/dev/null 2>&1 || { echo "ERROR: curl is required" >&2; exit 1; }

[ -f "$GENESIS" ] || { echo "ERROR: missing Phase 7 genesis state: $GENESIS" >&2; exit 1; }
umask 077
[ ! -e "$ROOT" ] || { echo "ERROR: runtime directory already exists: $ROOT" >&2; exit 1; }
mkdir -p "$ROOT/node-a" "$ROOT/node-b" "$ROOT/node-c" "$ROOT/logs"

cleanup() {
    set +e
    for d in node-a node-b node-c; do
        if [ -f "$ROOT/$d/config.json" ]; then
            "$BIN" stop "$ROOT/$d" >/dev/null 2>&1 || true
        fi
    done
    for pid_file in "$ROOT/a.pid" "$ROOT/b.pid" "$ROOT/c.pid"; do
        if [ -f "$pid_file" ]; then
            pid=$(cat "$pid_file" 2>/dev/null || true)
            [ -n "$pid" ] && kill "$pid" 2>/dev/null || true
            [ -n "$pid" ] && wait "$pid" 2>/dev/null || true
        fi
    done
    rm -rf "$ROOT"
}
trap cleanup EXIT INT TERM HUP

generate_secret() { "$PYTHON_BIN" -c 'import secrets; print(secrets.token_hex(32))'; }
TOKEN_A=$(generate_secret); TOKEN_B=$(generate_secret); TOKEN_C=$(generate_secret)
IDENTITY_KEY_A=$(generate_secret); IDENTITY_KEY_B=$(generate_secret); IDENTITY_KEY_C=$(generate_secret)
[ "$TOKEN_A" != "$TOKEN_B" ] && [ "$TOKEN_A" != "$TOKEN_C" ] && [ "$TOKEN_B" != "$TOKEN_C" ] || { echo "ERROR: API credentials are not unique" >&2; exit 1; }
[ "$IDENTITY_KEY_A" != "$IDENTITY_KEY_B" ] && [ "$IDENTITY_KEY_A" != "$IDENTITY_KEY_C" ] && [ "$IDENTITY_KEY_B" != "$IDENTITY_KEY_C" ] || { echo "ERROR: identity encryption keys are not unique" >&2; exit 1; }

GO_ENV_A="SYJ_IDENTITY_ENCRYPTION_KEY=$IDENTITY_KEY_A"
GO_ENV_B="SYJ_IDENTITY_ENCRYPTION_KEY=$IDENTITY_KEY_B"
GO_ENV_C="SYJ_IDENTITY_ENCRYPTION_KEY=$IDENTITY_KEY_C"

go build -o "$BIN" ./cmd/syjd

cat > "$ROOT/node-a/config.json" <<JSON
{"data_dir":"$ROOT/node-a","network_name":"$NETWORK","listen_address":"127.0.0.1:30301","advertised_address":"127.0.0.1:30301","seeds":["127.0.0.1:30302","127.0.0.1:30303"],"max_peers":8,"api_listen_address":"127.0.0.1:18080","api_auth_token":"$TOKEN_A","phase7_genesis_state_path":"$GENESIS","phase7_genesis_commitment":"36351980711cd88fe6ff134f0e4e858ee1a4572a9f44b7bde9f57213e7f1eb82","mempool_max":1000,"log_level":"INFO"}
JSON
cat > "$ROOT/node-b/config.json" <<JSON
{"data_dir":"$ROOT/node-b","network_name":"$NETWORK","listen_address":"127.0.0.1:30302","advertised_address":"127.0.0.1:30302","seeds":["127.0.0.1:30301","127.0.0.1:30303"],"max_peers":8,"api_listen_address":"127.0.0.1:18081","api_auth_token":"$TOKEN_B","phase7_genesis_state_path":"$GENESIS","phase7_genesis_commitment":"36351980711cd88fe6ff134f0e4e858ee1a4572a9f44b7bde9f57213e7f1eb82","mempool_max":1000,"log_level":"INFO"}
JSON
cat > "$ROOT/node-c/config.json" <<JSON
{"data_dir":"$ROOT/node-c","network_name":"$NETWORK","listen_address":"127.0.0.1:30303","advertised_address":"127.0.0.1:30303","seeds":["127.0.0.1:30301","127.0.0.1:30302"],"max_peers":8,"api_listen_address":"127.0.0.1:18082","api_auth_token":"$TOKEN_C","phase7_genesis_state_path":"$GENESIS","phase7_genesis_commitment":"36351980711cd88fe6ff134f0e4e858ee1a4572a9f44b7bde9f57213e7f1eb82","mempool_max":1000,"log_level":"INFO"}
JSON
chmod 600 "$ROOT/node-a/config.json" "$ROOT/node-b/config.json" "$ROOT/node-c/config.json"

SYJ_IDENTITY_ENCRYPTION_KEY="$IDENTITY_KEY_A" "$BIN" start "$ROOT/node-a" >"$ROOT/logs/a.log" 2>&1 & echo $! >"$ROOT/a.pid"
SYJ_IDENTITY_ENCRYPTION_KEY="$IDENTITY_KEY_B" "$BIN" start "$ROOT/node-b" >"$ROOT/logs/b.log" 2>&1 & echo $! >"$ROOT/b.pid"
SYJ_IDENTITY_ENCRYPTION_KEY="$IDENTITY_KEY_C" "$BIN" start "$ROOT/node-c" >"$ROOT/logs/c.log" 2>&1 & echo $! >"$ROOT/c.pid"

wait_for_health() {
    url=$1
    i=0
    while [ "$i" -lt 60 ]; do
        status=$(curl -sS -o /dev/null -w '%{http_code}' "$url/health" 2>/dev/null || true)
        [ "$status" = "200" ] && return 0
        sleep 1; i=$((i + 1))
    done
    echo "ERROR: node failed health check: $url" >&2
    return 1
}
wait_for_health http://127.0.0.1:18080
wait_for_health http://127.0.0.1:18081
wait_for_health http://127.0.0.1:18082

api_status() { token=$1; method=$2; url=$3; curl -sS -o /dev/null -w '%{http_code}' -X "$method" -H "Authorization: Bearer $token" "$url"; }
get_status() { token=$1; url=$2; curl -fsS -H "Authorization: Bearer $token" "$url/status"; }
assert_status() { [ "$1" = "$2" ] || { echo "ERROR: $3: expected $1 got $2" >&2; return 1; }; }

assert_status 401 "$(curl -sS -o /dev/null -w '%{http_code}' -X POST http://127.0.0.1:18080/mine)" "missing /mine authorization"
assert_status 401 "$(api_status invalid-token POST http://127.0.0.1:18080/mine)" "invalid /mine authorization"
assert_status 401 "$(api_status "$TOKEN_B" POST http://127.0.0.1:18080/mine)" "cross-node /mine authorization"

A_STATUS=$(get_status "$TOKEN_A" http://127.0.0.1:18080)
B_STATUS=$(get_status "$TOKEN_B" http://127.0.0.1:18081)
C_STATUS=$(get_status "$TOKEN_C" http://127.0.0.1:18082)
for status in "$A_STATUS" "$B_STATUS" "$C_STATUS"; do
    echo "$status" | grep -q 'sayanjali-syj-phase7-v1' || { echo "ERROR: Phase 7 network identity missing" >&2; exit 1; }
    echo "$status" | grep -q '"genesis_supply_base_units":28800000000000000' || { echo "ERROR: 288M genesis allocation not initialized" >&2; exit 1; }
    echo "$status" | grep -q '"mining_issued_base_units":0' || { echo "ERROR: unexpected initial mining issuance" >&2; exit 1; }
done

assert_status 201 "$(api_status "$TOKEN_A" POST http://127.0.0.1:18080/mine)" "authenticated mining"

wait_for_chain_height() {
    url=$1; token=$2; expected=$3; i=0
    while [ "$i" -lt 60 ]; do
        h=$(get_status "$token" "$url" | "$PYTHON_BIN" -c 'import json,sys; print(json.load(sys.stdin)["height"])' 2>/dev/null || true)
        [ "$h" = "$expected" ] && return 0
        sleep 1; i=$((i + 1))
    done
    echo "ERROR: chain height did not reach $expected at $url" >&2; return 1
}
wait_for_chain_height http://127.0.0.1:18080 "$TOKEN_A" 1
wait_for_chain_height http://127.0.0.1:18081 "$TOKEN_B" 1
wait_for_chain_height http://127.0.0.1:18082 "$TOKEN_C" 1

PEERS_A=$(curl -fsS http://127.0.0.1:18080/peers)
PEERS_B=$(curl -fsS http://127.0.0.1:18081/peers)
[ "$(echo "$PEERS_A" | "$PYTHON_BIN" -c 'import json,sys; print(len(json.load(sys.stdin)))')" -ge 1 ] || { echo "ERROR: node A peer discovery failed" >&2; exit 1; }
[ "$(echo "$PEERS_B" | "$PYTHON_BIN" -c 'import json,sys; print(len(json.load(sys.stdin)))')" -ge 1 ] || { echo "ERROR: node B peer discovery failed" >&2; exit 1; }

# Build a signed transaction from node A's encrypted persistent identity to
# node C's deterministic Phase 7 test recipient. The helper is created only
# for this runtime and is deleted before cleanup.
HELPER="$ROOT/txhelper.go"
cat > "$HELPER" <<'GO'
package main
import (
  "encoding/json"
  "os"
  "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/identity"
  "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
)
func main() {
  if len(os.Args) != 4 { panic("usage: txhelper <identity-dir> <receiver> <amount>") }
  id, _, err := identity.LoadOrCreate(os.Args[1]); if err != nil { panic(err) }
  tx := transaction.Transaction{Sender:id.Address, Receiver:os.Args[2], AmountBaseUnits:100000000, Timestamp:float64(1735689631)}
  if err := tx.Sign(func() *wallet.KeyPair { k, e := wallet.FromPrivateKeyHex(id.PrivateKeyHex); if e != nil { panic(e) }; return k }()); err != nil { panic(err) }
  _ = json.NewEncoder(os.Stdout).Encode(tx)
}
GO
# Replace imports in generated helper without ever persisting credentials.
sed -i 's#"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"#"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"\n  "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/wallet"#' "$HELPER"
TX_JSON=$(SYJ_IDENTITY_ENCRYPTION_KEY="$IDENTITY_KEY_A" go run "$HELPER" "$ROOT/node-a/identity" "SYJ69cdd15b3450cecd1713bba9667af43f73b468ec" 100000000)
rm -f "$HELPER"
TX_HASH=$(printf '%s' "$TX_JSON" | "$PYTHON_BIN" -c 'import json,sys; print(json.load(sys.stdin)["tx_hash"])')

assert_status 202 "$(printf '%s' "$TX_JSON" | curl -sS -o /dev/null -w '%{http_code}' -X POST -H "Authorization: Bearer $TOKEN_A" -H 'Content-Type: application/json' --data @- http://127.0.0.1:18080/transactions)" "authenticated transaction submission"

wait_for_pending() {
    i=0
    while [ "$i" -lt 60 ]; do
        count=$(curl -fsS http://127.0.0.1:18082/transactions | "$PYTHON_BIN" -c 'import json,sys; print(json.load(sys.stdin)["count"])' 2>/dev/null || true)
        [ "$count" -ge 1 ] && return 0
        sleep 1; i=$((i + 1))
    done
    echo "ERROR: transaction was not propagated to node C" >&2; return 1
}
wait_for_pending

assert_status 201 "$(api_status "$TOKEN_A" POST http://127.0.0.1:18080/mine)" "second authenticated mining"
wait_for_chain_height http://127.0.0.1:18080 "$TOKEN_A" 2
wait_for_chain_height http://127.0.0.1:18081 "$TOKEN_B" 2
wait_for_chain_height http://127.0.0.1:18082 "$TOKEN_C" 2

for pair in "http://127.0.0.1:18080 $TOKEN_A" "http://127.0.0.1:18081 $TOKEN_B" "http://127.0.0.1:18082 $TOKEN_C"; do
    set -- $pair
    TIP=$(get_status "$2" "$1" | "$PYTHON_BIN" -c 'import json,sys; print(json.load(sys.stdin)["tip_hash"])')
    curl -fsS "$1/blocks/$TIP" | grep -q "$TX_HASH" || { echo "ERROR: transaction not included on $1" >&2; exit 1; }
done

for pair in "http://127.0.0.1:18080 $TOKEN_A" "http://127.0.0.1:18081 $TOKEN_B" "http://127.0.0.1:18082 $TOKEN_C"; do
    set -- $pair
    get_status "$2" "$1" | grep -q '"supply_base_units":28800010000000000' || { echo "ERROR: unexpected Phase 7 supply after two blocks" >&2; exit 1; }
done

# Restart node B and verify persistence, identity, reconnection and auth.
B_ID_BEFORE=$(get_status "$TOKEN_B" http://127.0.0.1:18081 | "$PYTHON_BIN" -c 'import json,sys; print(json.load(sys.stdin)["node_id"])')
assert_status 202 "$(api_status "$TOKEN_B" POST http://127.0.0.1:18081/shutdown)" "authenticated shutdown"
sleep 2
SYJ_IDENTITY_ENCRYPTION_KEY="$IDENTITY_KEY_B" "$BIN" start "$ROOT/node-b" >"$ROOT/logs/b-restart.log" 2>&1 & echo $! >"$ROOT/b.pid"
wait_for_health http://127.0.0.1:18081
B_ID_AFTER=$(get_status "$TOKEN_B" http://127.0.0.1:18081 | "$PYTHON_BIN" -c 'import json,sys; print(json.load(sys.stdin)["node_id"])')
[ "$B_ID_BEFORE" = "$B_ID_AFTER" ] || { echo "ERROR: node identity changed after restart" >&2; exit 1; }
wait_for_chain_height http://127.0.0.1:18081 "$TOKEN_B" 2
assert_status 201 "$(api_status "$TOKEN_B" POST http://127.0.0.1:18081/mine)" "post-restart authenticated mining"
wait_for_chain_height http://127.0.0.1:18080 "$TOKEN_A" 3
wait_for_chain_height http://127.0.0.1:18081 "$TOKEN_B" 3
wait_for_chain_height http://127.0.0.1:18082 "$TOKEN_C" 3

printf '%s\n' "PHASE 7 THREE-NODE TESTNET: PASS" \
  "network identity: $NETWORK" \
  "genesis allocation: 28800000000000000 base units" \
  "mining allocation: 43200000000000000 base units" \
  "maximum supply: 72000000000000000 base units" \
  "authentication, peer discovery, propagation, inclusion, convergence, restart, identity recovery: PASS" \
  "credentials are not printed; runtime will be removed on exit."
