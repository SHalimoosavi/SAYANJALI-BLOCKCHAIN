# SYJ Phase 7 Address-Independent Build Manifest

## Source

Built from the uploaded local worktree archive:
`SAYANJALI-BLOCKCHAIN-PHASE6.5-LOCAL.zip`

The uploaded worktree identifies the intended branch as:
`phase-6.5-network-hardening-go`

Its HEAD is the Phase 6 baseline commit:
`114aa6f3a3e88f6d416001ea761d89b237d5d926`

The archive did not include `.git`, so the uncommitted status cannot be
reconstructed mechanically. The supplied source contains the expected local
Phase 6.5 hardening files, including `internal/security/` and changes under
chain/node/p2pnode.

## Phase 7 work added to this copy

- `pkg/protocol/constants.go`: added `GenesisAllocationSender` only.
- `internal/tokenomics/allocation.go`: exact Phase 7 allocation model,
  structural validation, injected existing-address validation, and supply plan.
- `internal/tokenomics/allocation_test.go`: allocation/supply invariant tests.
- `protocol/tokenomics/genesis_allocation.schema.json`: schema.
- `protocol/tokenomics/genesis_allocation.template.json`: address placeholders only.
- `docs/phase7/VESTING_SCHEDULE.md`
- `docs/phase7/MULTISIG_CUSTODY.md`
- `docs/phase7/FINAL_ADDRESS_BINDING.md`
- `docs/phase7/INTEGRATION_STATUS.md`

## Safety boundaries preserved

- Frozen Phase 4 genesis implementation/vector untouched.
- No production address generated or substituted.
- No private key/seed generated or included.
- No nonce added.
- No halving enabled.
- No on-chain vesting.
- No native consensus multisig.
- No P2P wire change.

## Tests / validation

PASS:
- `go test ./internal/tokenomics`
- `go test ./pkg/protocol`
- Frozen Phase 4 protocol/vector SHA-256 checks for all 10 known artifacts.
- Secret filename/pattern scan of the working copy.

BLOCKED in this environment:
- `go test ./...`
- `go build ./...`
- `go vet ./...`

Reason: the container cannot download the existing Go dependency
`github.com/decred/dcrd/dcrec/secp256k1/v4@v4.4.1` because outbound network/DNS
is unavailable. This is an environment limitation, not a test failure in the
Phase 7 package.

Also pending on the real local Termux environment:
- full Phase 6.5 regression,
- race testing where supported,
- final address validation,
- final Phase 7 genesis construction/vector/hash,
- genesis-state integration and mining-budget enforcement against the actual
  chain state.

## GitHub status

No GitHub ref was modified by this build. The connected GitHub write operation
for branch creation returned HTTP 403, so no commit/push/PR is claimed.

## Final address gate

The production genesis remains blocked until these five real public SYJ
addresses are intentionally supplied:

1. presale
2. treasury
3. ecosystem/grants
4. liquidity
5. team/advisors

They belong only in the final `protocol/tokenomics/genesis_allocation.json`
recipient fields, after validation with the repository's existing address
validator.
