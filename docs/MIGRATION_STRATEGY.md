# SAYANJALI BLOCKCHAIN --- Python → Go Migration Strategy

## Objective

Move the production node implementation from Python to Go without
changing protocol behavior merely because the implementation language
changes.

## Boundary

### Python remains

-   reference implementation
-   protocol oracle
-   vector generator/verifier
-   compatibility test oracle
-   simulation/research environment
-   historical implementation

### Go becomes

-   future production node/core
-   long-running node process
-   production P2P implementation
-   production storage/state implementation
-   production RPC/CLI/miner integration

## Controlled sequence

1.  Freeze and document current protocol behavior.
2.  Normalize deterministic serialization rules.
3.  Generate committed protocol test vectors from Python.
4.  Build Go protocol primitives.
5.  Run Go against the vectors.
6.  Run Python and Go against the same fixtures.
7.  Resolve only explicit compatibility failures.
8.  Implement Go consensus/state/chain validation.
9.  Add Go persistence.
10. Implement production P2P protocol.
11. Run multi-node Go private testnet.
12. Add adversarial and fault-injection testing.
13. Benchmark only after correctness is stable.
14. Consider Python/Go dual-node compatibility only after the wire
    protocol is formally frozen.

## Branch policy

-   `phase-4-protocol-specification` --- protocol/documentation work
-   `phase-4-go-production-core` --- Go core implementation
-   `phase-4-compatibility` --- cross-language compatibility harness
-   `phase-4-testnet` --- private testnet integration

Experimental work must not go directly to `main`.

## Compatibility gate

For each protocol component:

`Python reference -> canonical vector -> Go implementation -> vector equality -> compatibility test -> next component`

A vector mismatch is a stop condition, not something to "fix" by
changing the expected vector to match Go.

## Protocol change policy

Any intentional change to existing behavior requires: 1. explicit
protocol-change statement; 2. ADR; 3. updated specification; 4. old/new
compatibility analysis; 5. new vectors; 6. migration/versioning plan; 7.
test coverage.

## Safe implementation order

1.  protocol primitives
2.  serialization
3.  cryptography
4.  wallet/address
5.  transactions
6.  Merkle
7.  block/header
8.  PoW consensus
9.  difficulty
10. chain work
11. validation
12. state transition
13. monetary accounting
14. mempool
15. storage
16. mining
17. P2P
18. peer management
19. sync
20. propagation
21. RPC
22. CLI
23. lifecycle
24. observability
25. testnet
26. adversarial testing
27. benchmarking

## Non-goals

The migration does not authorize: - new tokenomics; - new genesis
allocation; - a new consensus algorithm; - a state root; - smart
contracts; - staking; - a silent serialization rewrite; - deletion of
Python; - a blind repository rewrite.
