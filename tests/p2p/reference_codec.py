"""Small independent reference encoder for the frozen SYJ P2P v1.0 grammar.

This is a wire-format test oracle only. It does not implement node/network behavior.
"""
from __future__ import annotations
import struct

MAGIC=b"SYJP"; MAJOR=1; MINOR=0; HEADER=20; MAX_FRAME=4*1024*1024
TYPES={"HELLO":1,"HELLO_ACK":2,"GET_PEERS":3,"PEERS":4,"GET_HEADERS":5,"HEADERS":6,"GET_BLOCKS":7,"BLOCKS":8,"NEW_BLOCK":9,"NEW_TRANSACTION":10,"REJECT":11}
CAP_BLOCKS=1; CAP_TRANSACTIONS=2; CAP_SYNC=4

def u8(n): return struct.pack(">B",n)
def u16(n): return struct.pack(">H",n)
def u32(n): return struct.pack(">I",n)
def u64(n): return struct.pack(">Q",n)
def s(x):
 b=x.encode(); assert len(b)<=256; return u16(len(b))+b
def raw(b,maxn): assert len(b)<=maxn; return u32(len(b))+b

def frame(name,req,payload):
 t=TYPES[name]; assert req or name in {"NEW_BLOCK","NEW_TRANSACTION"}
 return MAGIC+bytes((MAJOR,MINOR))+u16(t)+u64(req)+u32(len(payload))+payload

def hello(ack=False):
 h=bytes(range(32)); pub=bytes(range(64)); ch=bytes([7])*32; sig=bytes([8])*64
 p=s("sayanjali-p2p")+u8(1)+u8(0)+s("sayanjali-mainnet-mvp")+h+s("node-a")+raw(pub,64)+s("127.0.0.1:3030")+u32(7)
 if ack: p+=ch+bytes([9])*32+sig; return frame("HELLO_ACK",1,p)
 return frame("HELLO",1,p+ch+sig)

def get_peers(): return frame("GET_PEERS",2,s("")+u16(256))
def peers():
 p=u16(1)+s("node-a")+s("127.0.0.1")+u16(3030)+u32(7)+s(""); return frame("PEERS",2,p)
def get_headers():
 h=bytes(range(32)); return frame("GET_HEADERS",3,u8(1)+h+bytes(32)+u16(2048))
def headers(): return frame("HEADERS",3,u16(1)+raw(b'{"index":1}',4096))
def get_blocks(): return frame("GET_BLOCKS",4,u16(1)+bytes(range(32)))
def blocks(): return frame("BLOCKS",4,u16(1)+raw(b'{"hash":"00"}',512*1024))
def new_block(): return frame("NEW_BLOCK",0,raw(b'{"hash":"00"}',512*1024))
def new_tx(): return frame("NEW_TRANSACTION",0,raw(b'{"tx_hash":"00"}',64*1024))
def reject(): return frame("REJECT",4+0,u16(5)+u8(0)+u8(1)+s("invalid request"))

def main():
 vals={"hello":hello(),"hello_ack":hello(True),"get_peers":get_peers(),"peers":peers(),"get_headers":get_headers(),"headers":headers(),"get_blocks":get_blocks(),"blocks":blocks(),"new_block":new_block(),"new_transaction":new_tx(),"reject":reject()}
 import json, pathlib
 out=pathlib.Path(__file__).parent/"fixtures"; out.mkdir(parents=True,exist_ok=True)
 (out/"valid.json").write_text(json.dumps({k:{"frame_hex":v.hex()} for k,v in vals.items()},indent=2)+"\n",encoding="utf-8")
 neg=[]
 b=bytearray(vals["get_peers"]); b[0]=0x00; neg.append(("bad_magic",bytes(b),"bad_magic"))
 b=bytearray(vals["get_peers"]); b[4]=2; neg.append(("unsupported_version",bytes(b),"unsupported_version"))
 b=bytearray(vals["get_peers"]); b[7]=99; neg.append(("unknown_type",bytes(b),"unknown_type"))
 b=bytearray(vals["get_peers"]); b[15]=0; neg.append(("zero_request_id",bytes(b),"invalid_request_id"))
 b=bytearray(vals["get_peers"]); b[19]=100; neg.append(("truncated_payload",bytes(b),"truncated"))
 b=bytearray(vals["get_peers"]); b[16:20]=struct.pack(">I",MAX_FRAME); neg.append(("oversized_length",bytes(b),"frame_too_large"))
 (out/"invalid.json").write_text(json.dumps([{"name":n,"frame_hex":b.hex(),"error":e} for n,b,e in neg],indent=2)+"\n",encoding="utf-8")

if __name__=="__main__": main()
