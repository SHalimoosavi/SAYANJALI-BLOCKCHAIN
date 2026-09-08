# SYJ Production Genesis Configuration

Phase 7B adds an operator-controlled configuration and verification layer around the authoritative Phase 7A `tokenomics.GenesisState`.

It does not change the historical genesis block, block or transaction serialization, consensus rules, mining rules, or P2P wire grammar.

## Environments

The configuration requires an explicit environment:

- `private-testnet`
- `public-testnet`
- `production`
- `mainnet`

Production and mainnet configurations fail closed for missing or invalid allocation addresses and for MVP-style network identities.

The existing frozen block-genesis network identity is not silently renamed by Phase 7B. Network identity changes are an operational compatibility decision.

## Five production allocations

Exactly five allocation categories are accepted with the established Phase 7A amounts:

| Category | SYJ | Base units |
|---|---:|---:|
| Presale | 72,000,000 | 7,200,000,000,000,000 |
| Treasury | 57,600,000 | 5,760,000,000,000,000 |
| Ecosystem / Grants | 36,000,000 | 3,600,000,000,000,000 |
| Liquidity | 43,200,000 | 4,320,000,000,000,000 |
| Team / Advisors | 79,200,000 | 7,920,000,000,000,000 |
| **Genesis total** | **288,000,000** | **28,800,000,000,000,000** |

The established mining allocation is:

- 432,000,000 SYJ
- 43,200,000,000,000,000 base units

Genesis plus mining therefore equals the established maximum supply of 720,000,000 SYJ.

## Address safety

Only public SYJ addresses belong in genesis configuration.

The tooling never requires:

- private keys
- seed phrases
- mnemonics
- passwords
- credentials
- signing material

The configuration layer reuses the existing `internal/wallet.ValidAddress` validator.

The current SYJ address format does not contain a network discriminator. Therefore address-format validation alone cannot prove that an address belongs to a particular network.

Production addresses must be obtained from their externally controlled custody owners and independently verified.

The repository intentionally does not contain the five real production addresses and must not fabricate them.

## Deterministic workflow

1. Obtain the five real public SYJ allocation addresses from their custody owners.
2. Put only those public addresses into a private operator configuration copy.
3. Validate the configuration:

   `go run ./cmd/syj-genesis validate --config <config.json>`

4. Generate the artifact:

   `go run ./cmd/syj-genesis generate --config <config.json> --out <artifact.json>`

5. Record the resulting SHA-256 commitment through an independent channel.
6. Verify the artifact:

   `go run ./cmd/syj-genesis verify --artifact <artifact.json>`

7. Independently reproduce the commitment before any launch decision.

The commitment uses the repository's existing canonical JSON implementation and the authoritative Phase 7A `GenesisState` canonical representation. Allocation ordering therefore does not alter the commitment.

No timestamp, random value, map iteration order, machine state, or operating-system state is included in the canonical commitment.

## Vesting and custody

The following are operational/application controls rather than consensus rules:

- Presale: 3-month cliff followed by 12-month linear vesting.
- Team / Advisors: 12-month cliff followed by 24-month linear vesting.
- Treasury and Ecosystem / Grants: multisig-gated and milestone-based custody.
- Liquidity: designated liquidity-provisioning custody.

Phase 7B does not introduce consensus vesting or consensus multisig rules.

## Final production gate

A production commitment must not be generated until all five real public allocation addresses have been supplied and independently verified.

The supplied production template intentionally contains empty addresses and therefore must fail validation until populated.
