#!/bin/sh
set -eu
ROOT=${1:-./.phase6-testnet}
BIN="$ROOT/syjd"
mkdir -p "$ROOT/node-a" "$ROOT/node-b" "$ROOT/node-c" "$ROOT/logs"
go build -o "$BIN" ./cmd/syjd
cat > "$ROOT/node-a/config.json" <<JSON
{"data_dir":"$ROOT/node-a","network_name":"sayanjali-mainnet-mvp","listen_address":"127.0.0.1:30301","advertised_address":"127.0.0.1:30301","seeds":["127.0.0.1:30302","127.0.0.1:30303"],"max_peers":8,"api_listen_address":"127.0.0.1:18080","mempool_max":1000,"log_level":"INFO"}
JSON
cat > "$ROOT/node-b/config.json" <<JSON
{"data_dir":"$ROOT/node-b","network_name":"sayanjali-mainnet-mvp","listen_address":"127.0.0.1:30302","advertised_address":"127.0.0.1:30302","seeds":["127.0.0.1:30301","127.0.0.1:30303"],"max_peers":8,"api_listen_address":"127.0.0.1:18081","mempool_max":1000,"log_level":"INFO"}
JSON
cat > "$ROOT/node-c/config.json" <<JSON
{"data_dir":"$ROOT/node-c","network_name":"sayanjali-mainnet-mvp","listen_address":"127.0.0.1:30303","advertised_address":"127.0.0.1:30303","seeds":["127.0.0.1:30301","127.0.0.1:30302"],"max_peers":8,"api_listen_address":"127.0.0.1:18082","mempool_max":1000,"log_level":"INFO"}
JSON
"$BIN" start "$ROOT/node-a" >"$ROOT/logs/a.log" 2>&1 & echo $! >"$ROOT/a.pid"
"$BIN" start "$ROOT/node-b" >"$ROOT/logs/b.log" 2>&1 & echo $! >"$ROOT/b.pid"
"$BIN" start "$ROOT/node-c" >"$ROOT/logs/c.log" 2>&1 & echo $! >"$ROOT/c.pid"
echo "3-node testnet started under $ROOT"
echo "A: API 18080 / P2P 30301"
echo "B: API 18081 / P2P 30302"
echo "C: API 18082 / P2P 30303"
echo "Use: curl -s -X POST http://127.0.0.1:18080/mine"
echo "Stop with: $BIN stop $ROOT/node-a ; $BIN stop $ROOT/node-b ; $BIN stop $ROOT/node-c"
