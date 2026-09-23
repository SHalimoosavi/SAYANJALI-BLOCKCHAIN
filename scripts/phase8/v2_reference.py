#!/usr/bin/env python3
"""Independent Python V2 protocol reference/vector generator."""
from __future__ import annotations

import hashlib
import hmac
import json
from pathlib import Path

P = 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F
N = 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141
G = (55066263022277343669578718895168534326250603453777594175500187360389116729240,
     32670510020758816978083085130507043184471273380659243275938904335757337482424)
HALF_N = N // 2


def canon(obj):
    return json.dumps(obj, sort_keys=True, separators=(",", ":"), ensure_ascii=True, allow_nan=False).encode()


def sha256(b):
    return hashlib.sha256(b).digest()


def inv(a, m=N):
    return pow(a, -1, m)


def point_add(a, b):
    if a is None:
        return b
    if b is None:
        return a
    x1, y1 = a
    x2, y2 = b
    if x1 == x2 and (y1 + y2) % P == 0:
        return None
    if a == b:
        lam = (3 * x1 * x1) * inv((2 * y1) % P, P) % P
    else:
        lam = (y2 - y1) * inv((x2 - x1) % P, P) % P
    x3 = (lam * lam - x1 - x2) % P
    y3 = (lam * (x1 - x3) - y1) % P
    return x3, y3


def scalar_mul(k, point=G):
    out = None
    cur = point
    while k:
        if k & 1:
            out = point_add(out, cur)
        cur = point_add(cur, cur)
        k >>= 1
    return out


def bits2octets(h1):
    z1 = int.from_bytes(h1, 'big')
    z2 = z1 - N if z1 >= N else z1
    return z2.to_bytes(32, 'big')


def rfc6979_k(priv, digest):
    x = priv.to_bytes(32, 'big')
    h1 = bits2octets(digest)
    V = b'\x01' * 32
    K = b'\x00' * 32
    K = hmac.new(K, V + b'\x00' + x + h1, hashlib.sha256).digest()
    V = hmac.new(K, V, hashlib.sha256).digest()
    K = hmac.new(K, V + b'\x01' + x + h1, hashlib.sha256).digest()
    V = hmac.new(K, V, hashlib.sha256).digest()
    while True:
        T = b''
        while len(T) < 32:
            V = hmac.new(K, V, hashlib.sha256).digest()
            T += V
        k = int.from_bytes(T, 'big')
        if 1 <= k < N:
            return k
        K = hmac.new(K, V + b'\x00', hashlib.sha256).digest()
        V = hmac.new(K, V, hashlib.sha256).digest()


def sign(priv, digest):
    z = int.from_bytes(digest, 'big')
    k = rfc6979_k(priv, digest)
    x, _ = scalar_mul(k)
    r = x % N
    s = ((z + r * priv) * inv(k)) % N
    if s > HALF_N:
        s = N - s
    return r.to_bytes(32, 'big') + s.to_bytes(32, 'big')


def verify(pub, digest, sig):
    r = int.from_bytes(sig[:32], 'big')
    s = int.from_bytes(sig[32:], 'big')
    if not (1 <= r < N and 1 <= s <= HALF_N):
        return False
    w = inv(s)
    z = int.from_bytes(digest, 'big')
    p1 = scalar_mul(z * w, G)
    p2 = scalar_mul(r * w, pub)
    q = point_add(p1, p2)
    return q is not None and q[0] % N == r


def effective_network_id():
    gs_commit = '36351980711cd88fe6ff134f0e4e858ee1a4572a9f44b7bde9f57213e7f1eb82'
    historical = '5c95d2d7b63bde94fbfcfe4c2a95850ca96e0fc649e93e545b8b32f3a9c7cc1b'
    name = 'sayanjali-syj-phase7-v1'
    payload = {
        'consensus_protocol_version': 2,
        'genesis_state_commitment': gs_commit,
        'historical_genesis_hash': historical,
        'network_name': name,
    }
    return hashlib.sha256(b'SYJ-EFFECTIVE-NETWORK-ID-V1\x00' + canon(payload)).hexdigest(), canon(payload)


def main():
    network_id, effective_payload = effective_network_id()
    priv = 1
    pub = bytes.fromhex('79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8')
    pub_xy = (int.from_bytes(pub[:32], 'big'), int.from_bytes(pub[32:], 'big'))
    tx = {
        'version': 2,
        'network_id': network_id,
        'sender': 'SYJaf533e027d9d6ccc0958c470c26ea9d96a9b76fd',
        'receiver': 'SYJ1111111111111111111111111111111111111111',
        'amount_base_units': 123456789,
        'nonce': 0,
        'timestamp': 1735689601.25,
        'sender_public_key': pub.hex(),
    }
    payload = {
        'amount_base_units': tx['amount_base_units'],
        'network_id': tx['network_id'],
        'nonce': tx['nonce'],
        'receiver': tx['receiver'],
        'sender': tx['sender'],
        'sender_public_key': tx['sender_public_key'],
        'timestamp': tx['timestamp'],
        'version': 2,
    }
    canonical = canon(payload)
    signing_bytes = b'SYJ-TX-SIGN-V2\x00' + canonical
    digest = sha256(signing_bytes)
    sig = sign(priv, digest)
    assert verify(pub_xy, digest, sig)
    txid = hashlib.sha256(b'SYJ-TX-ID-V2\x00' + canonical).hexdigest()
    out = {
        'effective_network_id': network_id,
        'effective_network_id_canonical_json': effective_payload.decode(),
        'wire_network_name': 'syjnet-v2-' + network_id,
        'genesis_state_commitment': '36351980711cd88fe6ff134f0e4e858ee1a4572a9f44b7bde9f57213e7f1eb82',
        'historical_genesis_hash': '5c95d2d7b63bde94fbfcfe4c2a95850ca96e0fc649e93e545b8b32f3a9c7cc1b',
        'signing_payload': payload,
        'canonical_signing_payload': canonical.decode(),
        'signing_bytes_utf8_hex': signing_bytes.hex(),
        'signing_digest': digest.hex(),
        'signature': sig.hex(),
        'signature_low_s': int.from_bytes(sig[32:], 'big') <= HALF_N,
        'signature_verifies': True,
        'tx_id': txid,
        'transaction': {**tx, 'signature': sig.hex(), 'tx_id': txid},
    }
    out_path = Path(__file__).resolve().parents[2] / 'protocol' / 'test-vectors' / 'v2' / 'primary.json'
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(json.dumps(out, indent=2, sort_keys=True) + '\n', encoding='utf-8')
    print(json.dumps(out, indent=2, sort_keys=True))


if __name__ == '__main__':
    main()
