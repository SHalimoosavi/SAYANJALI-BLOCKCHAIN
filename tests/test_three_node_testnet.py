"""Real three-process local testnet convergence scenario for Phase 3."""
from __future__ import annotations

import os, subprocess, sys, time, uuid
from pathlib import Path
import httpx
import pytest

ROOT = Path(__file__).resolve().parent.parent
PORTS = (18981, 18982, 18983)

def launch(port: int, db: str, bootstrap: str = ""):
    env=os.environ.copy(); env.update({"SYJ_DB_FILE":db,"SYJ_PORT":str(port),"SYJ_HOST":"127.0.0.1","SYJ_ADVERTISED_ADDRESS":f"http://127.0.0.1:{port}","SYJ_DIFFICULTY":"1","SYJ_NETWORK_NAME":"sayanjali-test-net","SYJ_LOG_LEVEL":"WARNING","SYJ_NETWORK_MAINTENANCE_INTERVAL":"1"})
    env["SYJ_BOOTSTRAP_PEERS"]=bootstrap
    return subprocess.Popen([sys.executable,"-m","uvicorn","api.main:app","--host","127.0.0.1","--port",str(port),"--log-level","warning"],cwd=ROOT,env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)

def wait(url, timeout=15):
    end=time.time()+timeout
    while time.time()<end:
        try:
            if httpx.get(url+"/health",timeout=1).status_code==200:return
        except httpx.HTTPError: pass
        time.sleep(.15)
    raise TimeoutError(url)

def cli(db, port, *args):
    env=os.environ.copy(); env.update({"SYJ_DB_FILE":db,"SYJ_ADVERTISED_ADDRESS":f"http://127.0.0.1:{port}","SYJ_DIFFICULTY":"1","SYJ_NETWORK_NAME":"sayanjali-test-net","SYJ_LOG_LEVEL":"WARNING"})
    return subprocess.run([sys.executable,"-m","cli.main",*args],cwd=ROOT,env=env,capture_output=True,text=True,timeout=30)

@pytest.fixture
def three_nodes():
    dbs=[f"test_3node_{uuid.uuid4().hex}_{p}.db" for p in PORTS]
    bases=[f"http://127.0.0.1:{p}" for p in PORTS]
    procs=[launch(PORTS[0],dbs[0]),launch(PORTS[1],dbs[1]),launch(PORTS[2],dbs[2])]
    try:
        for b in bases: wait(b)
        yield bases,dbs
    finally:
        for proc in procs:
            proc.terminate()
        for proc in procs:
            try: proc.wait(timeout=5)
            except subprocess.TimeoutExpired: proc.kill()
        for db in dbs: (ROOT/"database"/db).unlink(missing_ok=True)

def test_three_node_discovery_propagation_and_convergence(three_nodes):
    bases, dbs = three_nodes
    a,b,c=bases; da,db,dc=dbs
    assert cli(da,PORTS[0],"add-peer",b).returncode==0
    assert cli(db,PORTS[1],"add-peer",c).returncode==0
    assert cli(dbs[2],PORTS[2],"add-peer",b).returncode==0
    assert cli(dbs[1],PORTS[1],"add-peer",a).returncode==0

    # A learns C transitively from B's peer list; B and C are trusted links.
    assert wait_for(lambda: any(p["address"]==c for p in httpx.get(a+"/network/peers").json()["peers"]), 8)
    sa,sb,sc=[httpx.get(x+"/network/status").json() for x in bases]
    assert len({sa["node_id"],sb["node_id"],sc["node_id"]})==3

    wallet=httpx.post(a+"/wallet/create").json()
    mined=httpx.post(a+"/mine",json={"miner_address":wallet["address"]})
    assert mined.status_code==200
    assert wait_for(lambda: httpx.get(c+"/network/status").json()["chain_length"]==2, 10)

    receiver=httpx.post(c+"/wallet/create").json()
    tx=httpx.post(a+"/transaction/create",json={"sender":wallet["address"],"receiver":receiver["address"],"amount":"10"}).json()
    from blockchain.transaction import Transaction
    from blockchain.wallet import Wallet
    t=Transaction.from_dict(tx); t.sign(Wallet.from_private_key(wallet["private_key"]))
    submitted=httpx.post(a+"/transaction/submit",json=t.to_dict())
    assert submitted.status_code==200 and submitted.json()["accepted"]
    assert wait_for(lambda: any(x["tx_hash"]==t.tx_hash for x in httpx.get(c+"/transactions/pending").json()), 10)
    mined2=httpx.post(a+"/mine",json={"miner_address":wallet["address"]})
    assert mined2.status_code==200
    assert wait_for(lambda: all(httpx.get(x+"/network/status").json()["chain_length"]==3 for x in bases), 10)

    chains=[httpx.get(x+"/network/chain").json() for x in bases]
    tips={x["chain"][-1]["hash"] for x in chains}
    works={x["work"] for x in chains}
    assert len(tips)==1 and len(works)==1
    assert httpx.get(c+f"/wallet/{receiver['address']}").json()["balance"]=="10"
    supplies=[httpx.get(x+"/network/status").json()["total_supply_base_units"] for x in bases]
    assert len(set(supplies))==1 and supplies[0] <= 720_000_000*100_000_000

def wait_for(predicate, timeout):
    end=time.time()+timeout
    while time.time()<end:
        try:
            if predicate(): return True
        except Exception: pass
        time.sleep(.2)
    return False
