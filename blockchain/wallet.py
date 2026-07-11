"""
Wallet system for SAYANJALI BLOCKCHAIN.

Implements ECDSA (SECP256k1) key-pair generation, address derivation,
key import/export, and message signing/verification. SECP256k1 is used
because it is the industry-standard curve for blockchain wallets (as used
by Bitcoin and Ethereum), which keeps the door open for future
cross-chain / bridge compatibility.

Address format:
    A wallet address is derived as:
        "SYJ" + first 40 hex chars of SHA-256(public_key_hex)
    The "SYJ" prefix makes SAYANJALI addresses immediately recognizable
    and distinguishable from other chains' addresses.
"""

from __future__ import annotations

from dataclasses import dataclass

from ecdsa import BadSignatureError, SECP256k1, SigningKey, VerifyingKey

from blockchain.utils import get_logger, sha256
from blockchain.utils import WalletError

logger = get_logger("blockchain.wallet")

ADDRESS_PREFIX = "SYJ"
ADDRESS_HASH_LENGTH = 40


@dataclass
class Wallet:
    """
    Represents a SAYANJALI BLOCKCHAIN wallet.

    Attributes:
        private_key_hex: Hex-encoded ECDSA private key. Never transmit or
            log this value.
        public_key_hex: Hex-encoded ECDSA public key (uncompressed point).
        address: Derived, human-shareable wallet address.
    """

    private_key_hex: str
    public_key_hex: str
    address: str

    @classmethod
    def create(cls) -> "Wallet":
        """Generate a brand-new wallet with a fresh SECP256k1 key pair."""
        signing_key = SigningKey.generate(curve=SECP256k1)
        verifying_key = signing_key.get_verifying_key()

        private_key_hex = signing_key.to_string().hex()
        public_key_hex = verifying_key.to_string().hex()
        address = derive_address(public_key_hex)

        logger.info("Created new wallet with address %s", address)
        return cls(
            private_key_hex=private_key_hex,
            public_key_hex=public_key_hex,
            address=address,
        )

    @classmethod
    def from_private_key(cls, private_key_hex: str) -> "Wallet":
        """
        Reconstruct a wallet from an existing hex-encoded private key.

        Raises:
            WalletError: if the supplied private key is malformed.
        """
        try:
            raw = bytes.fromhex(private_key_hex)
            signing_key = SigningKey.from_string(raw, curve=SECP256k1)
        except Exception as exc:  # noqa: BLE001 - surfaced as WalletError
            raise WalletError(f"Invalid private key: {exc}") from exc

        verifying_key = signing_key.get_verifying_key()
        public_key_hex = verifying_key.to_string().hex()
        address = derive_address(public_key_hex)

        return cls(
            private_key_hex=private_key_hex,
            public_key_hex=public_key_hex,
            address=address,
        )

    def export_keys(self) -> dict[str, str]:
        """Export the wallet's key material and address as a plain dict."""
        return {
            "private_key": self.private_key_hex,
            "public_key": self.public_key_hex,
            "address": self.address,
        }

    def sign(self, message: str) -> str:
        """
        Sign an arbitrary message with this wallet's private key.

        Returns:
            Hex-encoded signature.
        """
        raw = bytes.fromhex(self.private_key_hex)
        signing_key = SigningKey.from_string(raw, curve=SECP256k1)
        signature = signing_key.sign(message.encode("utf-8"), hashfunc=__import__("hashlib").sha256)
        return signature.hex()


def derive_address(public_key_hex: str) -> str:
    """Derive a SAYANJALI wallet address from a hex-encoded public key."""
    digest = sha256(public_key_hex)
    return f"{ADDRESS_PREFIX}{digest[:ADDRESS_HASH_LENGTH]}"


def is_valid_address(address: str) -> bool:
    """
    Validate the structural correctness of a SAYANJALI wallet address.

    This checks format only (prefix + hex length); it cannot confirm the
    address was ever actually derived from a real public key, since
    addresses are one-way hashes.
    """
    if not isinstance(address, str):
        return False
    if not address.startswith(ADDRESS_PREFIX):
        return False
    remainder = address[len(ADDRESS_PREFIX):]
    if len(remainder) != ADDRESS_HASH_LENGTH:
        return False
    try:
        int(remainder, 16)
        return True
    except ValueError:
        return False


def verify_signature(public_key_hex: str, message: str, signature_hex: str) -> bool:
    """
    Verify that `signature_hex` is a valid ECDSA signature of `message`
    produced by the private key corresponding to `public_key_hex`.

    Returns:
        True if the signature is valid, False otherwise (never raises for
        a bad signature -- only for structurally malformed input).
    """
    try:
        raw_pub = bytes.fromhex(public_key_hex)
        verifying_key = VerifyingKey.from_string(raw_pub, curve=SECP256k1)
        signature = bytes.fromhex(signature_hex)
        return verifying_key.verify(
            signature, message.encode("utf-8"), hashfunc=__import__("hashlib").sha256
        )
    except BadSignatureError:
        return False
    except Exception as exc:  # noqa: BLE001
        logger.warning("Signature verification error: %s", exc)
        return False
