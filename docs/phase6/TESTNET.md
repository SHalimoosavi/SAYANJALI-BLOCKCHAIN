# Phase 6 — Three-Node Private Testnet

The repository includes `scripts/testnet/run-3-node.sh`. It creates three independent data directories, builds the real Go executable, starts three real processes, and uses the frozen binary P2P implementation for connections.

Ports used by the example:

| Node | P2P | API |
|---|---:|---:|
| A | 30301 | 18080 |
| B | 30302 | 18081 |
| C | 30303 | 18082 |

The script is intended for a clean local/private environment. It keeps all generated identities, journals and logs outside the source tree.

The API endpoints are local operational interfaces only. `/mine` performs the existing PoW block-production behavior; it does not introduce a new consensus rule.
