# Phase 6 Build / Audit Record

## Baseline

- Source archive: `SAYANJALI-BLOCKCHAIN-main.zip`
- Source archive SHA-256: `42a95912fd4c40773620c96b081b46c3a095f37f68b5541b7d722088da900f3d`
- Git metadata is not included in the supplied GitHub ZIP, so an exact Git base commit cannot be independently extracted from this artifact.
- Phase 4 protocol SHA-256: `7c898c90c1308493fc45e43542f80ac9508dab64d5c70377b084689bec97b3d5`
- Phase 5.1 dependency remains `github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.1`.

## Frozen-layer verification

The Phase 4 specification and all nine vector files were checked against the supplied baseline and remain byte-identical in this working tree. The Phase 5.2 wire specification and `internal/p2p/` files were not modified by Phase 6.

## Implemented

- persistent node identity with corruption detection;
- executable `cmd/syjd` entrypoint;
- append-only CRC-protected blockchain journal;
- deterministic chain reload and genesis verification;
- active-chain selection by frozen accumulated work;
- fork retention and higher-work reorganization;
- deterministic state replay from persisted blocks;
- bounded concurrent mempool;
- TCP P2P transport around the frozen Phase 5.2 codec;
- authenticated HELLO / HELLO_ACK integration;
- peer limits, request tracking and timeouts;
- GET_PEERS, GET_HEADERS, GET_BLOCKS serving;
- headers-first synchronization followed by block retrieval;
- NEW_BLOCK and NEW_TRANSACTION propagation;
- local operational HTTP API;
- start/stop/status/peers CLI operations;
- three-node testnet harness;
- unit/integration tests for identity, storage, chain fork/reorg, handshake and block propagation.

## Validation executed in this build environment

- `gofmt`: executed on all Phase 6 Go files.
- A temporary local dependency stub was used only for **compile/type-check validation** because the real secp256k1 dependency is not available to this isolated build environment. The stub is not part of the source tree and is not included in the delivery ZIP.
- Temporary-stub `go test -run '^$' ./...`: compile/type-check PASS for the entire repository.
- Real `go test ./...`: **ENVIRONMENT-LIMITED** here because `v4.4.1` cannot be downloaded from `proxy.golang.org` in this isolated environment.
- Full real Go tests, vet and production build must therefore be run from the user's Termux environment where `go mod download` and the baseline suite were already demonstrated to work.

## Not claimed

This artifact is not labelled production-ready. A truthful clean-room runtime validation requires the real `v4.4.1` dependency and must still demonstrate the actual executable, three-node network, synchronization, propagation, fork/reorg, shutdown and restart behavior on the final ZIP.

## Remaining hardening gates

- TLS/encrypted transport for hostile public deployment;
- peer reputation/quarantine/ban policy;
- public API authentication/authorization;
- stronger persistent peer database policy;
- orphaned transaction re-injection after reorg;
- future timestamp/MTP policy (currently undefined by the frozen protocol);
- transaction nonce/replay protection (currently undefined by the frozen protocol);
- finalized halving/emission schedule;
- finalized fee policy;
- finalized genesis allocation.

These are not silently introduced by Phase 6.

## Frozen vector hashes

```text
address.json    934bc7f39cfa1f237b6977176ce7a495f1376a4b74c6e055bfe7c0e05772a06c
block.json      a6a729f8371046c6673b356c0d599bf8aa0f6b9e9cb019441a058d5b61a1e074
chain_work.json 31c2b2b8de636036b5346c24c9e6317906419b911413aa70329a53478d4e0471
difficulty.json 899bb814808a30c70e436c297b77d9f035bfa5db0580b0ccff33c1c133aaf2dc
genesis.json    bcbc4ad94084b4e4cbf9c6247dfda8dccc60a5a42b9f660955fc1c311dbd9434
merkle.json     48e484876a36fb9e5945c581b626d193499dc122776ace810e3abfdab482202e
monetary.json   7f502bb4d569bbd5a2a059088394bdd75a090ec63f551bc80065f2b838bb0618
pow.json        1471889f5b45b2fbb58c6f07525b856d4c844f2d70735c5ef4e599d079fd3b4d
transaction.json 8729cff6d3012eff82367fea7ba3a1999030c8d5e97bfc7796565b156283a426
```
