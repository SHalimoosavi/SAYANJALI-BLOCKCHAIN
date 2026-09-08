# Phase 6 Security and Resource Limits

Implemented controls include:

- frozen P2P frame/message size limits;
- frozen request-tracker limit of 64 outstanding requests/connection;
- handshake and request timeouts;
- bounded mempool size;
- bounded peer count;
- bounded synchronization batch sizes from the frozen wire protocol;
- CRC-protected persistent journal;
- private identity file permissions (0600) and data directory permissions (0700);
- no private-key logging or network/API exposure;
- no wildcard CORS in the Go local API;
- consensus validation before block/transaction relay.

Remaining hardening gates include authenticated/encrypted transport for hostile public networks, peer reputation/quarantine/ban policy, persistent peer database policy, stronger API authentication, and a formal Sybil/eclipse-resistance design. These are not claimed solved by Phase 6.
