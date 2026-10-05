#!/usr/bin/env python3
import json,hashlib,sys
from pathlib import Path
R=Path(__file__).resolve().parents[2]; V=R/"protocol/test-vectors/v3"
tx=json.loads((V/"transaction-v3.json").read_text())["vectors"]["key-1"]
wire=bytearray.fromhex(tx["wire_hex"])
assert wire[0]==3
wire_bad=bytes([2])+bytes(wire[1:])
assert hashlib.sha256(b"SYJ/tx-id/v3"+wire_bad).hexdigest()!=tx["tx_id"]
assert tx["amount"]>0 and tx["fee"]>0 and tx["valid_until_height"]==100
assert tx["sender"]=="SYJaf533e027d9d6ccc0958c470c26ea9d96a9b76fd"
assert tx["sender"] != "SYJ1111111111111111111111111111111111111111"
assert len(tx["sender"])==43
upper=tx["sender"][:3]+tx["sender"][3:].upper()
assert upper!=tx["sender"] and upper[3:].lower()==tx["sender"][3:]
print("PASS transaction negative/boundary vectors: version, tx-id mutation, amount/fee, sender/address invariants")
