# SYJ Genesis Verification

A Phase 7B genesis artifact is independently verifiable from its public contents.

Verification checks:

- explicit network name;
- explicit environment;
- exact five allocation categories;
- exact Phase 7 allocation base units;
- genesis allocation total of 288,000,000 SYJ;
- mining allocation of 432,000,000 SYJ;
- maximum supply of 720,000,000 SYJ;
- existing SYJ address format;
- deterministic canonical representation;
- SHA-256 commitment equality;
- absence of private/signing material.

## Verification procedure

Generate an artifact from an approved configuration:

`go run ./cmd/syj-genesis generate --config <config.json> --out <artifact.json>`

Verify it independently:

`go run ./cmd/syj-genesis verify --artifact <artifact.json>`

Print the verified public artifact:

`go run ./cmd/syj-genesis print-summary --artifact <artifact.json>`

The five production allocation addresses must come from their externally controlled custody owners. This repository does not fabricate or generate those addresses.

The Phase 7B commitment includes the explicit environment and network identity together with the authoritative Phase 7A genesis-state representation.

No private key, seed phrase, mnemonic, password, credential, or signing material is required.
