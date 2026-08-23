"""
P2P identity keypair for SAYANJALI BLOCKCHAIN's networking layer.

This is deliberately a separate concept from `blockchain.wallet.Wallet`.
A node's P2P identity key authenticates *the node* to its peers; a wallet
key authorizes *spending value* on the chain. Conflating the two would
mean a node's networking layer needs access to funds-controlling key
material just to talk to peers, and would mean a peer's chain address
becomes derivable from (or confused with) its network identity. Keeping
them separate also means a compromised P2P identity key never exposes
any wallet, and vice versa.

Reuses the exact same cryptographic primitive already used for wallets
(ECDSA on SECP256k1, via the `ecdsa` library) rather than introducing a
new one, per the explicit instruction not to invent a home-grown
cryptographic primitive.

Private P2P identity key material:
    - is generated once per node and persisted via `Storage` (a dedicated
      table, never mixed with wallet or chain data)
    - is never sent over HTTP
    - is never logged
    - is never included in any propagated message
    - is only ever used locally, to produce signatures that are sent in
      place of the key itself
"""

from __future__ import annotations

import hashlib

from ecdsa import BadSignatureError, SECP256k1, SigningKey, VerifyingKey

from blockchain.utils import WalletError, get_logger

logger = get_logger("blockchain.network.identity")


class P2PIdentity:
    """
    A node's persistent P2P identity keypair.

    `node_id` here is the pre-existing, backward-compatible identifier
    from `Storage.get_or_create_node_id()` (an opaque persisted string) --
    this class does not change what `node_id` means or how it's exposed.
    What it adds is a cryptographic public/private keypair *bound to*
    that same node_id via storage, so the previously bare, self-asserted
    identifier now has a verifiable public key behind it.
    """

    def __init__(self, node_id: str, private_key_hex: str, public_key_hex: str) -> None:
        self.node_id = node_id
        self.private_key_hex = private_key_hex
        self.public_key_hex = public_key_hex

    @classmethod
    def generate(cls, node_id: str) -> "P2PIdentity":
        """Generate a brand-new P2P identity keypair for `node_id`."""
        signing_key = SigningKey.generate(curve=SECP256k1)
        verifying_key = signing_key.get_verifying_key()
        return cls(
            node_id=node_id,
            private_key_hex=signing_key.to_string().hex(),
            public_key_hex=verifying_key.to_string().hex(),
        )

    def sign(self, message: str) -> str:
        """Sign `message` with this identity's private key. Returns hex signature."""
        raw = bytes.fromhex(self.private_key_hex)
        signing_key = SigningKey.from_string(raw, curve=SECP256k1)
        signature = signing_key.sign(message.encode("utf-8"), hashfunc=hashlib.sha256)
        return signature.hex()


def verify_identity_signature(
    public_key_hex: str, message: str, signature_hex: str
) -> bool:
    """
    Verify a signature was produced by the private key corresponding to
    `public_key_hex` over `message`.

    Mirrors `blockchain.wallet.verify_signature` exactly (same curve, same
    hash function) but is kept as an independent function in this module
    so the P2P identity verification path never imports from `wallet.py`
    -- reinforcing that these are separate trust domains even though they
    share an underlying primitive.
    """
    try:
        raw_pub = bytes.fromhex(public_key_hex)
        verifying_key = VerifyingKey.from_string(raw_pub, curve=SECP256k1)
        signature = bytes.fromhex(signature_hex)
        return verifying_key.verify(
            signature, message.encode("utf-8"), hashfunc=hashlib.sha256
        )
    except BadSignatureError:
        return False
    except Exception as exc:  # noqa: BLE001
        logger.warning("P2P identity signature verification error: %s", exc)
        return False


def is_valid_public_key(public_key_hex: str) -> bool:
    """Structural check that a string is plausibly a SECP256k1 public key."""
    try:
        raw = bytes.fromhex(public_key_hex)
        VerifyingKey.from_string(raw, curve=SECP256k1)
        return True
    except Exception:  # noqa: BLE001
        return False
