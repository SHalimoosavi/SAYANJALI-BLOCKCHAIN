"""Pure-Python secp256k1 implementation B for SYJ C-2 vectors.
Standard library only and independently structured from implementation A.
"""
from hashlib import sha256
import hmac
FIELD=0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F
ORDER=0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141
BASE=(55066263022277343669578718895168534326250603453777594175500187360389116729240,
32670510020758816978083085130507043184471273380659243275938904335757337482424)
def recip(a,m): return pow(a % m,-1,m)
def ec_add(p,q):
    if p is None:return q
    if q is None:return p
    if p[0]==q[0] and (p[1]+q[1])%FIELD==0:return None
    if p==q:
        slope=(3*p[0]*p[0])*recip(2*p[1],FIELD)%FIELD
    else:
        slope=((q[1]-p[1])*recip(q[0]-p[0],FIELD))%FIELD
    nx=(slope*slope-p[0]-q[0])%FIELD
    return nx,(slope*(p[0]-nx)-p[1])%FIELD
def ec_mul(scalar,point=BASE):
    result=None
    for bit in bin(scalar)[2:]:
        result=ec_add(result,result)
        if bit=="1":result=ec_add(result,point)
    return result
def raw_public(scalar):
    if scalar<=0 or scalar>=ORDER:raise ValueError("private scalar out of range")
    x,y=ec_mul(scalar)
    return x.to_bytes(32,"big")+y.to_bytes(32,"big")
def compressed_public(scalar):
    raw=raw_public(scalar)
    return bytes([2+(raw[-1]&1)])+raw[:32]
def deterministic_k(scalar,digest):
    secret=scalar.to_bytes(32,"big")
    seed=int.from_bytes(digest,"big").to_bytes(32,"big")
    key=b"\0"*32; val=b"\1"*32
    key=hmac.new(key,val+b"\0"+secret+seed,sha256).digest()
    val=hmac.new(key,val,sha256).digest()
    key=hmac.new(key,val+b"\1"+secret+seed,sha256).digest()
    val=hmac.new(key,val,sha256).digest()
    while True:
        val=hmac.new(key,val,sha256).digest()
        n=int.from_bytes(val,"big")
        if 0<n<ORDER:return n
        key=hmac.new(key,val+b"\0",sha256).digest()
        val=hmac.new(key,val,sha256).digest()
def sign_digest(scalar,digest):
    z=int.from_bytes(digest,"big")
    k=deterministic_k(scalar,digest)
    x,_=ec_mul(k); r=x%ORDER
    s=(recip(k,ORDER)*(z+r*scalar))%ORDER
    return r, (ORDER-s if s>ORDER//2 else s)
def valid_point(point):
    if point is None:return False
    x,y=point
    return 0<=x<FIELD and 0<=y<FIELD and (y*y-x*x*x-7)%FIELD==0
def verify_digest(pub,digest,r,s):
    if len(pub)!=64 or not(0<r<ORDER and 0<s<=ORDER//2):return False
    q=(int.from_bytes(pub[:32],"big"),int.from_bytes(pub[32:],"big"))
    if not valid_point(q):return False
    invs=recip(s,ORDER); z=int.from_bytes(digest,"big")%ORDER
    point=ec_add(ec_mul((z*invs)%ORDER),ec_mul((r*invs)%ORDER,q))
    return valid_point(point) and point[0]%ORDER==r
