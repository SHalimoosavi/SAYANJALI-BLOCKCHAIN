# SYJ Tokenomics — Phase 7

Phase 7 establishes a maximum supply of **720,000,000 SYJ**.

All monetary values used by the protocol are represented in base units.

## Supply allocation

| Component | SYJ | Base units |
|---|---:|---:|
| Presale | 72,000,000 | 7,200,000,000,000,000 |
| Treasury | 57,600,000 | 5,760,000,000,000,000 |
| Ecosystem / Grants | 36,000,000 | 3,600,000,000,000,000 |
| Liquidity | 43,200,000 | 4,320,000,000,000,000 |
| Team / Advisors | 79,200,000 | 7,920,000,000,000,000 |
| **Genesis allocation** | **288,000,000** | **28,800,000,000,000,000** |
| **Mining allocation** | **432,000,000** | **43,200,000,000,000,000** |
| **Maximum supply** | **720,000,000** | **72,000,000,000,000,000** |

The five genesis allocations are fixed by the authoritative Phase 7A `tokenomics.GenesisState`.

Mining issuance is tracked separately and cannot exceed the established mining allocation.

## Genesis allocation

Genesis allocation is an economic initial-state operation, not a normal block transaction.

The authoritative Phase 7 implementation:

- validates the exact five categories;
- validates exact base-unit amounts;
- validates the exact 288M genesis total;
- validates recipient addresses using the existing SYJ address validator;
- initializes recipient balances;
- preserves genesis balances across chain restart and reorganization;
- prevents the genesis allocation sender from being used as a normal allocation transaction;
- provides deterministic canonical serialization and SHA-256 commitment.

## Production custody

Production allocation addresses must be supplied by the entities controlling the corresponding custody destinations.

The repository does not contain and must not fabricate production addresses.

Private keys and signing material remain outside the genesis configuration.

Recommended operational controls include:

- multisig custody for treasury and ecosystem funds;
- designated liquidity custody;
- independent verification of all five public recipient addresses;
- separate offline records of the final genesis commitment.

## Vesting

The following are application/off-chain policy controls and are not consensus rules:

- Presale: 3-month cliff followed by 12-month linear vesting.
- Team / Advisors: 12-month cliff followed by 24-month linear vesting.
- Treasury and Ecosystem / Grants: milestone-based custody.
- Liquidity: designated liquidity-provisioning custody.

Phase 7B does not introduce consensus-level vesting or multisig rules.
