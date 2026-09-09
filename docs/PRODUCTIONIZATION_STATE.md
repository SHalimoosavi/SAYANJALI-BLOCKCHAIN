# SAYANJALI BLOCKCHAIN — Productionization State

Last updated: Phase 7 production-hardening working tree

## Current architecture

- **Go (`cmd/syjd`, `internal/`)** — production-track node/core.
- **Python (`blockchain/`, `api/`, `cli/`)** — reference/oracle implementation retained for protocol compatibility and regression coverage. It is not the production network API.

## Phase 7 monetary state

- Maximum: 720,000,000 SYJ (`72,000,000,000,000,000` base units)
- Genesis allocation: 288,000,000 SYJ (`28,800,000,000,000,000` base units)
- Mining allocation: 432,000,000 SYJ (`43,200,000,000,000,000` base units)
- Network: `sayanjali-syj-phase7-v1`

The frozen historical block genesis remains unchanged. Economic GenesisState is deterministic initial state and is not an ordinary transaction.

## Security hardening status

- Coinbase transaction validation is fail-closed on all transaction integrity errors.
- Block timestamps remain strictly increasing and now use deterministic median-time-past evaluation plus a chain-history future-step bound.
- Mutating Go API routes require bearer authentication. Non-loopback API listeners require TLS.
- Go P2P supports a TLS transport wrapper without changing Phase 5.2 frame/message encoding.
- Node identity private keys are encrypted at rest with externally supplied AES-256-GCM key material.
- Config and identity files require owner-only permissions.
- Journal replay truncates only an incomplete final header/payload tail; complete-record CRC corruption remains fatal.
- The three-node testnet exercises the actual Phase 7 economic network.

## Not a mainnet-ready declaration

This repository should not be represented as fully mainnet-ready solely from these code changes. Production still requires approved custody/genesis operations, certificate and CA lifecycle management, external secret management, CI/status enforcement, reproducible dependency policy, monitoring/alerting, operational recovery procedures, independent security review, and deployment-specific validation.
