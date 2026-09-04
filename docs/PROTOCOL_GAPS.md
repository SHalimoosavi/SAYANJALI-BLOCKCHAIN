# SAYANJALI BLOCKCHAIN --- Protocol Gaps

This is a gap register, not a redesign proposal. Missing behavior is
recorded as undefined until an explicit protocol decision is approved.

## Critical compatibility gaps

### G-001 --- Normative protocol specification

**Severity:** Critical\
**Status:** Open\
Rules are currently distributed across Python source/config/tests. A
single normative specification does not yet exist.

### G-002 --- Deterministic cross-language serialization

**Severity:** Critical\
**Status:** Open\
The current implementation relies on Python JSON serialization behavior,
including float formatting and `default=str`. Go must not guess these
rules.

### G-003 --- Protocol test vectors

**Severity:** Critical\
**Status:** Open\
No committed Python/Go compatibility vector suite currently defines
expected hashes, signatures, Merkle roots, difficulty results and
monetary transitions.

### G-004 --- Protocol version vs implementation version

**Severity:** High\
**Status:** Open\
The package and API version strings are not aligned with the current
development checkpoint. A formal protocol-version field is absent from
block/transaction serialization.

## Consensus gaps

### G-005 --- Timestamp consensus bounds

**Severity:** High\
**Status:** Open\
Only strict monotonicity is enforced. Future-time and median-time rules
are undefined.

### G-006 --- Difficulty window semantics

**Severity:** High\
**Status:** Open\
The setting is `10`, but the current implementation uses 10 blocks to
measure 9 intervals. This is existing behavior and must be vectorized
before any change.

### G-007 --- Difficulty representation

**Severity:** High\
**Status:** Open\
Difficulty is an integer count of leading hexadecimal zeros. Work is
`16 ** difficulty`. This is valid current behavior but unusual and must
be preserved exactly.

### G-008 --- Chain-work domain

**Severity:** Medium\
**Status:** Open\
The chain-work formula is simple and deterministic, but its long-term
economic/security meaning has not been formally documented beyond the
current implementation.

## Monetary/state gaps

### G-009 --- Issuance schedule

**Severity:** High\
**Status:** Open\
`50 SYJ` is the current configured block reward and `210,000` is a
reserved halving interval, but the halving rule is not enforced. No
final emission schedule has been approved.

### G-010 --- Genesis allocation

**Severity:** Critical for mainnet\
**Status:** Undefined\
No final genesis distribution/allocation is defined. Do not invent one.

### G-011 --- Transaction nonce

**Severity:** High\
**Status:** Undefined\
No sender nonce exists. This affects replay semantics and future
account/state architecture.

### G-012 --- Fees

**Severity:** Medium\
**Status:** Undefined\
No transaction fee model is currently defined.

### G-013 --- State root

**Severity:** High\
**Status:** Undefined\
No state root is part of consensus. Do not add one without an explicit
protocol change.

## Networking gaps

### G-014 --- Production wire protocol

**Severity:** Critical\
**Status:** Open\
The current HTTP API is a prototype P2P transport. Final framing,
message grammar, request IDs, limits and compatibility rules are not
normative.

### G-015 --- Incremental synchronization

**Severity:** High\
**Status:** Open\
Current synchronization is full-chain. Production sync needs an explicit
headers/blocks strategy before scale testing.

### G-016 --- Peer reputation

**Severity:** High\
**Status:** Partial\
Failure backoff exists, but persistent reputation/quarantine/ban
semantics are absent.

### G-017 --- Transport security

**Severity:** High\
**Status:** Partial\
Peer authentication is cryptographic at the message layer, but transport
is still HTTP. Production TLS/secure transport semantics are undefined.

### G-018 --- DNS rebinding

**Severity:** High\
**Status:** Partial\
Address validation resolves hostnames once but does not pin the resolved
IP to the connection.

### G-019 --- Sybil/eclipsing strategy

**Severity:** High\
**Status:** Open\
Authentication proves key possession but does not by itself solve Sybil
or eclipse resistance.

## Operational gaps

### G-020 --- CI

**Severity:** High\
**Status:** Open\
No `.github/workflows/` directory exists at the audited checkpoint.

### G-021 --- Dependency reproducibility

**Severity:** High\
**Status:** Open\
`requirements.txt` uses minimum-version constraints; no lockfile is
present.

### G-022 --- Wallet key exposure

**Severity:** High\
**Status:** Open\
`POST /wallet/create` returns private key material. This must not be
retained in a production-facing node API.

### G-023 --- Documentation drift

**Severity:** Medium\
**Status:** Open\
README and badge counts do not match the verified Phase 3 baseline.

## Compatibility policy

Every gap is classified as: - **Defined:** directly implemented and
testable; - **Partial:** behavior exists but is not
production-complete; - **Undefined:** no protocol rule exists and
implementation must not invent one; - **Protocol change required:**
changing it would require an ADR and compatibility decision.
