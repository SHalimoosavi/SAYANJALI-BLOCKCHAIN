#!/usr/bin/env python3
import json,re,sys
from pathlib import Path
R=Path(__file__).resolve().parents[2]; V=R/"protocol/test-vectors/v3"
reg=set(json.loads((V/"domain-registry.json").read_text())["registered"])
for p in V.glob("*.json"):
    text=p.read_text()
    for d in set(re.findall(r"SYJ/[A-Za-z0-9_.-]+/v[0-9]+",text)):
        if d not in reg: raise SystemExit(f"UNREGISTERED DOMAIN: {d} in {p}")
for p in V.glob("*.json"):
    d=json.loads(p.read_text())
    def walk(x):
        if isinstance(x,str) and x.startswith("SYJ") and len(x)>=3:
            if re.fullmatch(r"SYJ[0-9a-fA-F]{40}",x):
                if len(x)!=43 or x[3:]!=x[3:].lower(): raise SystemExit(f"BAD 43-char address: {x}")
        elif isinstance(x,dict):
            for v in x.values(): walk(v)
        elif isinstance(x,list):
            for v in x: walk(v)
    walk(d)
print("PASS domain-registry and 43-character canonical-address scans")
