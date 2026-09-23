#!/usr/bin/env python3
"""Independent verifier for the V2 primary vector using cryptography."""
from __future__ import annotations

import hashlib
import json
from pathlib import Path

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.asymmetric.utils import Prehashed, encode_dss_signature

ROOT = Path(__file__).resolve().parents[2]
VECTOR = ROOT / "protocol" / "test-vectors" / "v2" / "primary.json"


def canon(obj):
    return json.dumps(obj, sort_keys=True, separators=(",", ":"), ensure_ascii=True, allow_nan=False).encode()


def main():
    v = json.loads(VECTOR.read_text(encoding="utf-8"))
    tx = v["transaction"]
    payload = {k: tx[k] for k in ("amount_base_units", "network_id", "nonce", "receiver", "sender", "sender_public_key", "timestamp", "version")}
    canonical = canon(payload)
    signing_bytes = b"SYJ-TX-SIGN-V2\x00" + canonical
    digest = hashlib.sha256(signing_bytes).digest()
    assert digest.hex() == v["signing_digest"]
    tx_id = hashlib.sha256(b"SYJ-TX-ID-V2\x00" + canonical).hexdigest()
    assert tx_id == v["tx_id"]

    pub = bytes.fromhex(tx["sender_public_key"])
    sig = bytes.fromhex(tx["signature"])
    key = ec.EllipticCurvePublicNumbers(
        int.from_bytes(pub[:32], "big"), int.from_bytes(pub[32:], "big"), ec.SECP256K1()
    ).public_key()
    key.verify(
        encode_dss_signature(int.from_bytes(sig[:32], "big"), int.from_bytes(sig[32:], "big")),
        digest,
        ec.ECDSA(Prehashed(hashes.SHA256())),
    )
    print("V2 independent vector verification: PASS")
    print("effective_network_id=" + v["effective_network_id"])
    print("signing_digest=" + v["signing_digest"])
    print("tx_id=" + v["tx_id"])


if __name__ == "__main__":
    main()
