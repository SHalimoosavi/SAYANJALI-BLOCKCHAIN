#!/usr/bin/env python3
import json,hashlib,sys
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parents[2]))
from tools.c2.ec_a import *
ROOT=Path(__file__).resolve().parents[2]; V=ROOT/"protocol/test-vectors/v3"
d=json.loads((V/"ec-key-anchors.json").read_text())
assert d["anchors"][1]["public_key_raw_hex"]=="c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee51ae168fea63dc339a3c58419466ceaeef7f632653266d0e1236431a950cfe52a"
assert d["anchors"][1]["public_key_compressed_hex"]=="02c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee5"
tx=json.loads((V/"transaction-v3.json").read_text())["vectors"]
odd=tx["odd-parity-n-minus-1"]
assert odd["signing_digest"]=="49abd933182355725c2ecb7e641f37b3ae0b0f64e14ddda3e9f2aeefd6ed3c81"
assert odd["r"]=="45922ce791663e9dd46ae275d08f46647de3c631d9c74c8d2a2b5a05da5207bb"
assert odd["s"]=="21cf1e654515402614444e1af3940f61d20e38439d10a616e7b25dd94e450584"
assert odd["tx_id"]=="dcdb66c98113fc8068d8f67ba277663a3b36e681406839e12e5af37452809cf5"
raw=bytes.fromhex(odd["public_key_raw_hex"]); sig=bytes.fromhex(odd["signature"])
assert verify(bytes.fromhex(odd["signing_digest"]),raw,int(odd["r"],16),int(odd["s"],16))
assert len(odd["sender"])==43 and odd["sender"]=="SYJa139031e68bee75cfbc184adc1d2801c7f9ef94c"
print("PASS verifier A: anchors, owner odd-parity golden, signature, tx_id, address length")

# Negative cases.
odd=tx["odd-parity-n-minus-1"]; raw=bytes.fromhex(odd["public_key_raw_hex"])
r=int(odd["r"],16); s=int(odd["s"],16); dig=bytes.fromhex(odd["signing_digest"])
assert not verify(dig,raw,r,N-s)
assert not verify(dig,raw,0,s)
assert not verify(dig,raw,r,0)
assert not verify(bytes([dig[0]^1])+dig[1:],raw,r,s)
assert not verify(dig,raw[:-1],r,s)
assert not verify(dig,raw,int(odd["r"],16)+1,s)
print("PASS EC negative vectors: zero/high-S/tampered digest/pubkey/r")
