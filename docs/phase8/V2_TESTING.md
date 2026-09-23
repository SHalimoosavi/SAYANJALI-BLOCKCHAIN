# V2 Testing and Reproduction

## Required commands

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./...
python -m pytest
python -m compileall
```

Run the V2 reference/vector generator twice and compare the generated files byte-for-byte:

```bash
python3 scripts/phase8/v2_reference.py > /tmp/v2-vector-1.txt
cp protocol/test-vectors/v2/primary.json /tmp/v2-primary-1.json
python3 scripts/phase8/v2_reference.py > /tmp/v2-vector-2.txt
cmp /tmp/v2-primary-1.json protocol/test-vectors/v2/primary.json
```

The generator is deterministic. It independently calculates EffectiveNetworkID, canonical payload, signing bytes, signing digest, deterministic low-S ECDSA signature, tx_id, and signature verification.

## Primary vector

- EffectiveNetworkID: `237a1934769295c63fe47771a4996b17c3899e52f6bacf79ed7edefec90eaaf3`
- Signing digest: `edbfdb74b27736cb4e1d09f9f641b8304d4c2da560e8fa1adc4b767a52574a31`
- Signature: `c70c07e257a8b5f45091dea4ec0ceee24db186e9fa3eb8ca2ed777b670e082d0540b1604a77a0da1499a5068a01c39314455d42c3d4ad5aa46d7417e6c26a0a9`
- tx_id: `f2d1b0239244c5c1b70805b15eba42a9e99ca32c764f17e267398136159136ac`

These values are generated from executable code and are checked by Go vector tests when the dependency-backed test suite can run.

## V1 regression gate

Never overwrite `protocol/test-vectors/*.json` with V2 vectors. The V1 fixture set remains frozen and must be run unchanged.

## Security/fuzz categories

At minimum cover malformed network IDs, malformed JSON, invalid signatures/public keys, NaN/Infinity timestamps, amount boundaries, nonce overflow, duplicate IDs, duplicate sender+nonce, same-block replay, cross-block replay, restart reconstruction, and reorg reconstruction.
