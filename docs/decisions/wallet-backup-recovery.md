# Decision note: user wallet backup and recovery

**Status:** OWNER DECISION REQUIRED. No wallet format or custody implementation is authorized by this note.

## Current baseline

The repository has Go key-generation and encrypted node-identity persistence. Node identity storage is not a user wallet backup/recovery product. The Python reference wallet uses the maintained `cryptography` package.

## Options

### Option A — encrypted keystore export

- Advantages: direct support for current key material; password-protected portable file.
- Trade-offs: users must safely store the file and password; password loss can make funds inaccessible.

### Option B — mnemonic seed and deterministic derivation

- Advantages: human-readable backup and standard recovery model.
- Trade-offs: requires selecting a mnemonic standard, entropy size, derivation path, passphrase rules and address/key compatibility. The current raw-key address derivation does not define these.

### Option C — hardware wallet integration

- Advantages: private keys can remain in dedicated hardware.
- Trade-offs: device support, signing protocol, recovery model and integration testing add substantial work.

## Recommendation

Design a user-wallet layer separate from node identity. Evaluate an interoperable mnemonic standard plus encrypted local keystore, but do not select derivation paths or change address/key semantics until owner approval and compatibility vectors exist. Document that lost keys mean lost access unless a separately approved custody model says otherwise; do not imply a hidden recovery authority.
