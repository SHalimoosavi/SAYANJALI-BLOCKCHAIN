"""Pure-Python secp256k1 implementation A for SYJ C-2 vectors.
Standard library only; independent of the production cryptography/OpenSSL code.
"""
import hashlib,hmac
P=0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F
N=0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141
G=(55066263022277343669578718895168534326250603453777594175500187360389116729240,
32670510020758816978083085130507043184471273380659243275938904335757337482424)
INF=None
def inv(x,m): return pow(x,-1,m)
def add(a,b):
    if a is None:return b
    if b is None:return a
    x1,y1=a;x2,y2=b
    if x1==x2:
        if (y1+y2)%P==0:return None
        lam=(3*x1*x1)*inv(2*y1,P)%P
    else: lam=(y2-y1)*inv((x2-x1)%P,P)%P
    x=(lam*lam-x1-x2)%P
    return x,(lam*(x1-x)-y1)%P
def mul(k,p=G):
    out=None; base=p
    while k:
        if k&1: out=add(out,base)
        base=add(base,base); k >>= 1
    return out
def public_key(priv):
    if not 1<=priv<N: raise ValueError("private scalar out of range")
    x,y=mul(priv); return x.to_bytes(32,"big")+y.to_bytes(32,"big")
def compressed_public_key(priv):
    raw=public_key(priv); return (b"\x02" if raw[-1]%2==0 else b"\x03")+raw[:32]
def rfc6979(priv,digest):
    bx=priv.to_bytes(32,"big"); bh=int.from_bytes(digest,"big").to_bytes(32,"big")
    v=b"\x01"*32;k=b"\x00"*32
    k=hmac.new(k,v+b"\x00"+bx+bh,hashlib.sha256).digest()
    v=hmac.new(k,v,hashlib.sha256).digest()
    k=hmac.new(k,v+b"\x01"+bx+bh,hashlib.sha256).digest()
    v=hmac.new(k,v,hashlib.sha256).digest()
    while True:
        v=hmac.new(k,v,hashlib.sha256).digest()
        candidate=int.from_bytes(v,"big")
        if 1<=candidate<N:return candidate
        k=hmac.new(k,v+b"\x00",hashlib.sha256).digest()
        v=hmac.new(k,v,hashlib.sha256).digest()
def sign(digest,priv):
    z=int.from_bytes(digest,"big"); k=rfc6979(priv,digest)
    x,_=mul(k); r=x%N
    s=(pow(k,-1,N)*(z+r*priv))%N
    if s>N//2:s=N-s
    return r,s
def verify(digest,pub64,r,s):
    if len(pub64)!=64 or not 1<=r<N or not 1<=s<=N//2:return False
    q=(int.from_bytes(pub64[:32],"big"),int.from_bytes(pub64[32:],"big"))
    if (q[1]*q[1]-q[0]**3-7)%P:return False
    w=pow(s,-1,N);z=int.from_bytes(digest,"big")%N
    point=add(mul((z*w)%N),mul((r*w)%N,q))
    return point is not None and point[0]%N==r
def sha256(data): return hashlib.sha256(data).digest()
