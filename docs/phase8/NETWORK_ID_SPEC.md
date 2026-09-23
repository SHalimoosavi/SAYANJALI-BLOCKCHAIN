# V2 Network Identity Specification

## EffectiveNetworkID inputs

```json
{
  "consensus_protocol_version": 2,
  "genesis_state_commitment": "...",
  "historical_genesis_hash": "...",
  "network_name": "sayanjali-syj-phase7-v1"
}
```

Keys are serialized using the repository's canonical JSON implementation.

The construction is:

`SHA256("SYJ-EFFECTIVE-NETWORK-ID-V1\\0" || canonical_json(inputs))`

The `V1` suffix belongs to this identity-construction algorithm. It does not mean that the resulting consensus protocol is V1.

## Audited private-testnet identity

- GenesisState commitment: `36351980711cd88fe6ff134f0e4e858ee1a4572a9f44b7bde9f57213e7f1eb82`
- Historical genesis hash: `5c95d2d7b63bde94fbfcfe4c2a95850ca96e0fc649e93e545b8b32f3a9c7cc1b`
- Network name input: `sayanjali-syj-phase7-v1`
- EffectiveNetworkID: `237a1934769295c63fe47771a4996b17c3899e52f6bacf79ed7edefec90eaaf3`
- P2P HELLO NetworkName: `syjnet-v2-237a1934769295c63fe47771a4996b17c3899e52f6bacf79ed7edefec90eaaf3`

## Fail-closed startup

A V2 node refuses startup when the GenesisState is missing/malformed, its commitment is wrong, the historical genesis hash changes, the configured network ID does not equal independent derivation, or the HELLO network name is not the derived V2 wire representation.

## Public network rule

A future public V2 network must receive an explicitly approved network name and independently approved GenesisState. Its identity must be recalculated; the private-testnet identity must not be copied as a production identity.
