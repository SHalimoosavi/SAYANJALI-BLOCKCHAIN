# Decision note: production genesis artifact and recipient uniqueness

**Status:** OWNER DECISION REQUIRED. No genesis data or validation-rule changes are authorized by this note.

## Finding 1: artifact-to-node-state format

The production genesis tool's `generate` command writes a `genesisconfig.Artifact`. The node's V2/V3 startup path calls `tokenomics.Load`, which expects a `tokenomics.GenesisState` JSON document.

The artifact and node-state JSON shapes differ:

- Artifact allocations use `address`, and the artifact includes environment, network name, tokenomics version and commitment fields.
- Node GenesisState allocations use `recipient`, and the state requires `version`, `sender`, `allocations` and `total_base_units`.

The CLI currently exposes `generate`, `validate`, `verify` and `print-summary`; it does not expose a command that exports the verified artifact as the exact node-loadable GenesisState.

### Options

1. Add an explicit `export-state` command that verifies the artifact and emits the canonical node GenesisState plus its commitment.
   - Pros: deterministic, auditable conversion; no manual editing.
   - Trade-offs: introduces a new genesis packaging command and needs compatibility vectors.
2. Teach node startup to accept the artifact directly and convert it after verification.
   - Pros: one canonical production package.
   - Trade-offs: changes node genesis loading and increases startup coupling to genesisconfig.
3. Keep separate files and document a manual conversion.
   - Pros: no node code change.
   - Trade-offs: error-prone and weakens reproducibility.

### Recommendation

Prefer an explicit deterministic export command with golden vectors and a clean-environment reproduction test. Do not implement until the owner approves the exact artifact-to-state contract.

## Finding 2: duplicate allocation recipients

The production `genesisconfig.Validate` path rejects duplicate recipient addresses. The node startup path loads `tokenomics.GenesisState` directly, and `GenesisState.Validate` currently validates categories, amounts and address syntax but does not reject duplicate recipients. A characterization test records that the same valid address can currently be used for all five categories.

### Options

1. Enforce unique recipients in `GenesisState.Validate` as well as the production config validator.
   - Pros: consistent validation and distinct custody destinations.
   - Trade-offs: changes which genesis-state files are accepted; requires protocol/genesis compatibility review.
2. Permit duplicate recipients and remove the uniqueness requirement from the production config validator.
   - Pros: explicit support for category aggregation into one custody address.
   - Trade-offs: category allocations no longer imply distinct recipient custody.
3. Make recipient uniqueness configurable by environment.
   - Pros: allows test flexibility and production strictness.
   - Trade-offs: adds environment-dependent validation semantics that must be frozen and documented.

### Recommendation

For a public production genesis, prefer unique recipients, but treat this as a genesis acceptance-rule decision. Obtain explicit owner approval before changing validation or existing genesis data.
