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


## Data-directory ownership

Run only one node process against a given data directory. The node takes an exclusive advisory lock on `.syj.lock` before opening the chain journal. A second process using the same directory fails to start with `data directory is already locked`. The lock is released on a normal store close and automatically when the operating system ends the holder process. Leave the lock file in place; do not remove it while the node is running. Advisory locks may not be honored by network filesystems, so keep node data on a local filesystem.
