# Phase 6 Operations

## Initialize

```sh
go run ./cmd/syjd init ./node-a
```

Edit `./node-a/config.json` for the P2P/API ports and bootstrap peers.

## Start

```sh
go run ./cmd/syjd start ./node-a
```

The process stays in the foreground and handles SIGINT/SIGTERM cleanly.

## Status / peers

```sh
go run ./cmd/syjd status ./node-a
go run ./cmd/syjd peers ./node-a
```

## Stop

```sh
go run ./cmd/syjd stop ./node-a
```

The default API binds to loopback. Do not expose the API directly to an untrusted network without adding authentication and an explicit deployment security layer.
