#!/usr/bin/env python3
import json,hashlib,sys
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parents[2]))
from tools.c2.ec_b import *
ROOT=Path(__file__).resolve().parents[2]; V=ROOT/"protocol/test-vectors/v3"
d=json.loads((V/"ec-key-anchors.json").read_text())
for a in d["anchors"]:
    priv=int(a["private_key_hex"],16)
    assert raw_public(priv).hex()==a["public_key_raw_hex"]
    assert compressed_public(priv).hex()==a["public_key_compressed_hex"]
tx=json.loads((V/"transaction-v3.json").read_text())["vectors"]
for name,t in tx.items():
    pre=bytes.fromhex(t["signing_preimage_hex"]); dig=sha256(pre).digest(); r,s=sign_digest(int(t["private_key_hex"],16),dig)
    assert dig.hex()==t["signing_digest"] and f"{r:064x}"==t["r"] and f"{s:064x}"==t["s"]
    assert verify_digest(bytes.fromhex(t["public_key_raw_hex"]),dig,r,s)
    wire=bytes.fromhex(t["wire_hex"])
    assert hashlib.sha256(b"SYJ/tx-id/v3"+wire).hexdigest()==t["tx_id"]
assert tx["odd-parity-n-minus-1"]["public_key_compressed_hex"]=="0379be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
print("PASS verifier B: independent EC arithmetic, RFC6979, low-S, tx_id")

odd=tx["odd-parity-n-minus-1"]; raw=bytes.fromhex(odd["public_key_raw_hex"]); dig=bytes.fromhex(odd["signing_digest"])
r=int(odd["r"],16); ss=int(odd["s"],16)
assert not verify_digest(raw,dig,r,ORDER-ss)
assert not verify_digest(raw,dig,0,ss)
assert not verify_digest(raw,bytes([dig[0]^1])+dig[1:],r,ss)
assert not verify_digest(raw[:-1],dig,r,ss)
print("PASS transaction/EC negative vectors: high-S, zero-r, tampered digest/pubkey")
