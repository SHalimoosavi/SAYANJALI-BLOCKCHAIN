#!/usr/bin/env python3
"""Generate deterministic SYJ C-2 Stage-1 vectors."""
from __future__ import annotations
import hashlib,json,sys
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parents[2]))
from tools.c2.ec_a import public_key,compressed_public_key,sign,sha256,N

OUT=Path(__file__).resolve().parents[2]/"protocol/test-vectors/v3"
CHAIN_ID=bytes(31)+b"\x01"
RECEIVER=bytes.fromhex("11"*20)
FIELDS={"version":3,"chain_id":CHAIN_ID.hex(),"nonce":0,"amount":150000000,"fee":100000,"valid_until_height":100}
def address(raw): return "SYJ"+hashlib.sha256(raw.hex().encode()).hexdigest()[:40]
def make_tx(priv):
    raw=public_key(priv); comp=compressed_public_key(priv)
    pre=b"SYJ/tx/v3"+bytes([3])+CHAIN_ID+(0).to_bytes(8,"big")+(150000000).to_bytes(8,"big")+(100000).to_bytes(8,"big")+(100).to_bytes(8,"big")+comp+RECEIVER
    digest=sha256(pre); r,s=sign(digest,priv)
    sig=r.to_bytes(32,"big")+s.to_bytes(32,"big")
    wire=bytes([3])+(0).to_bytes(8,"big")+(150000000).to_bytes(8,"big")+(100000).to_bytes(8,"big")+(100).to_bytes(8,"big")+comp+RECEIVER+sig
    return {
      **FIELDS,"sender":address(raw),"receiver":"11"*20,
      "private_key_hex":f"{priv:064x}","public_key_raw_hex":raw.hex(),"public_key_compressed_hex":comp.hex(),
      "signing_preimage_hex":pre.hex(),"signing_preimage_bytes":len(pre),
      "signing_digest":digest.hex(),"r":f"{r:064x}","s":f"{s:064x}",
      "signature":sig.hex(),"wire_hex":wire.hex(),"wire_bytes":len(wire),
      "tx_id":hashlib.sha256(b"SYJ/tx-id/v3"+wire).hexdigest(),
      "domain":"SYJ/tx/v3","tx_id_domain":"SYJ/tx-id/v3",
      "signature_low_s":s<=N//2
    }
anchors=[]
for label,priv in [("key-1",1),("key-2",2),("key-n-minus-1",N-1)]:
    raw=public_key(priv); comp=compressed_public_key(priv)
    anchors.append({"name":label,"private_key_hex":f"{priv:064x}","private_key_hex":f"{priv:064x}","public_key_raw_hex":raw.hex(),"public_key_compressed_hex":comp.hex(),"public_key_sha256":hashlib.sha256(raw.hex().encode()).hexdigest(),"address":address(raw)})
vectors={"format":"syj-c2-stage1-v1","curve":"secp256k1","anchors":anchors}
(OUT/"ec-key-anchors.json").write_text(json.dumps(vectors,indent=2)+"\n")
addr={"format":"syj-c2-stage1-address-v1","rule":"SYJ + first 40 hex chars of SHA-256(UTF-8(public_key_hex)); suffix canonical lowercase","vectors":[{"name":a["name"],"public_key_hex":a["public_key_raw_hex"],"address":a["address"],"length":len(a["address"]),"suffix_lowercase":a["address"][3:]==a["address"][3:].lower()} for a in anchors]}
(OUT/"address-v3.json").write_text(json.dumps(addr,indent=2)+"\n")
tx1=make_tx(1); txodd=make_tx(N-1)
(OUT/"transaction-v3.json").write_text(json.dumps({"format":"syj-c2-stage1-transaction-v1","vectors":{"key-1":tx1,"odd-parity-n-minus-1":txodd}},indent=2)+"\n")
neg={"format":"syj-c2-stage1-negative-v1","ec":[{"case":"zero_private_key","private_key_hex":"00"*32,"expected":"reject"},{"case":"order_private_key","private_key_hex":f"{N:064x}","expected":"reject"},{"case":"oversized_private_key","private_key_hex":f"{N+1:064x}","expected":"reject"},{"case":"malformed_public_key_length","public_key_hex":"00"*63,"expected":"reject"},{"case":"off_curve_public_key","public_key_hex":"00"*64,"expected":"reject"},{"case":"high_s_signature","description":"valid r with s=N-original_s","expected":"reject"}],"transaction":[{"case":"wrong_version","expected":"reject"},{"case":"wrong_chain_id","expected":"reject"},{"case":"zero_amount","expected":"reject"},{"case":"zero_fee_is_structurally distinct but policy-dependent","expected":"not a Stage-1 validity claim"},{"case":"tampered_amount","expected":"reject"},{"case":"tampered_signature","expected":"reject"},{"case":"tampered_tx_id","expected":"reject"},{"case":"uppercase_address_suffix","expected":"reject"},{"case":"wrong_tx_domain","expected":"reject"}]}
(OUT/"negative-vectors.json").write_text(json.dumps(neg,indent=2)+"\n")
(OUT/"domain-registry.json").write_text(json.dumps({"format":"syj-c2-domain-registry-v1","registered":["SYJ/tx/v3","SYJ/tx-id/v3"]},indent=2)+"\n")
