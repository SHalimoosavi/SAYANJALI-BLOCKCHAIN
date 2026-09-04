#!/usr/bin/env python3
"""Verify frozen SYJ compatibility vectors against the Python reference implementation."""
from __future__ import annotations
import json, sys
from fractions import Fraction
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
if str(ROOT) not in sys.path: sys.path.insert(0,str(ROOT))
from blockchain.block import Block
from blockchain.consensus import ProofOfWorkConsensus
from blockchain.native_asset import BASE_UNITS_PER_SYJ, MAX_SUPPLY_BASE_UNITS, format_amount, from_base_units, to_base_units, validate_base_units
from blockchain.transaction import Transaction
from blockchain.utils import deterministic_json, merkle_root, sha256
from blockchain.validators import _state_from_chain, chain_work
from blockchain.wallet import Wallet, derive_address, verify_signature
from config.settings import get_settings
VEC=ROOT/'protocol'/'test-vectors'

def load(name): return json.loads((VEC/name).read_text(encoding='utf-8'))
def check(ok,msg):
    if not ok: raise AssertionError(msg)

def verify_genesis():
    v=load('genesis.json'); s=get_settings(); g=Block.genesis(s.genesis.previous_hash,s.genesis.timestamp,s.genesis.nonce,s.genesis.message)
    check(v['genesis_transaction_hash']==g.transactions[0].tx_hash,'genesis tx hash drift')
    check(v['header_payload']==g.header_payload(),'genesis header drift')
    check(v['canonical_header_json']==deterministic_json(g.header_payload()),'genesis serialization drift')
    check(v['canonical_header_utf8_hex']==g.header_payload() and False or v['canonical_header_utf8_hex']==deterministic_json(g.header_payload()).encode().hex(),'genesis bytes drift')
    check(v['block_hash']==g.hash,'genesis block hash drift')

def verify_address():
    v=load('address.json'); w=Wallet.from_private_key(v['private_key_hex'])
    check(w.public_key_hex==v['public_key_hex'],'public key drift')
    check(derive_address(v['public_key_hex'])==v['address'],'address drift')
    check(sha256(v['public_key_hex'])==v['public_key_sha256'],'address hash drift')

def verify_tx():
    v=load('transaction.json')
    tx=Transaction(sender=v['sender'],receiver=v['receiver'],amount_base_units=v['amount_base_units'],timestamp=v['timestamp'],sender_public_key=v['sender_public_key'],signature=v['fixed_signature'],tx_hash=v['transaction_hash'])
    check(tx._signing_payload()==v['signing_payload'],'signing payload drift')
    check(tx.signing_message()==v['canonical_signing_message'],'signing message drift')
    check(v['canonical_signing_message_utf8_hex']==tx.signing_message().encode().hex(),'signing bytes drift')
    check(verify_signature(v['public_key_hex'],tx.signing_message(),v['fixed_signature']),'fixed signature no longer verifies')
    check(tx.compute_hash()==v['transaction_hash'],'transaction hash drift')
    check(tx.verify(),'transaction fixture rejected')

def verify_merkle():
    v=load('merkle.json')
    for c in v['cases']:
        check(merkle_root(c['transaction_hashes'])==c['expected_merkle_root'],f"merkle drift: {c['name']}")

def verify_block_pow():
    v=load('block.json'); h=v['header_payload']; b=Block(index=h['index'],previous_hash=h['previous_hash'],transactions=[],timestamp=h['timestamp'],nonce=h['nonce'],difficulty=h['difficulty'],merkle_root=h['merkle_root'])
    check(b.header_payload()==h,'block header drift'); check(deterministic_json(h)==v['canonical_header_json'],'block serialization drift'); check(deterministic_json(h).encode().hex()==v['canonical_header_utf8_hex'],'block bytes drift'); check(b.hash==v['expected_block_hash'],'block hash drift')
    p=load('pow.json')
    for c in p['cases']: check(b.meets_difficulty(c['difficulty'])==c['meets_difficulty'],f"pow drift: {c['difficulty']}")

def verify_difficulty():
    v=load('difficulty.json'); e=ProofOfWorkConsensus()
    for c in v['cases']:
        first,last,base=c['first_timestamp'],c['last_timestamp'],c['previous_difficulty']
        blocks=[Block(index=i,previous_hash='0'*64,timestamp=first if i==0 else last if i==9 else first+(last-first)*i//9,nonce=0,difficulty=base) for i in range(10)]
        actual=last-first; actual_guard=1 if actual<=0 else actual; expected=9*30; clamped=max(expected//4,min(actual_guard,expected*4)); target=Fraction(16**max(base,0)*expected,clamped)
        got=e.next_difficulty(blocks,4,30,1,32,4)
        for key,val in [('actual_timespan',actual),('expected_timespan',expected),('clamped_timespan',clamped),('current_work',16**max(base,0)),('target_work_numerator',target.numerator),('target_work_denominator',target.denominator),('expected_difficulty',got)]: check(c[key]==val,f"difficulty drift {c['name']}: {key}")

def verify_work():
    v=load('chain_work.json')
    for c in v['cases']:
        blocks=[Block(index=i,previous_hash='0'*64,timestamp=1735689600+i,nonce=0,difficulty=d) for i,d in enumerate(c['difficulties'])]
        check(c['per_block_work']==[16**d for d in c['difficulties']],'per-block work drift'); check(chain_work(blocks)==c['accumulated_work'],'chain work drift')

def verify_monetary():
    v=load('monetary.json'); check(v['base_units_per_syj']==BASE_UNITS_PER_SYJ,'base unit constant drift'); check(v['max_supply_base_units']==MAX_SUPPLY_BASE_UNITS,'max supply drift'); check(to_base_units('50.0')==v['default_block_reward_base_units'],'reward conversion drift')
    for x,b in zip(['1','50.0','0.00000001','720000000'],v['cases'][0]['base_units']): check(to_base_units(x)==b,'conversion drift')
    check(format_amount(123456789)==v['cases'][1]['formatted'],'amount formatting drift')
    s=get_settings(); genesis=Block.genesis('0'*64,s.genesis.timestamp,s.genesis.nonce,s.genesis.message); receiver='SYJ'+'2'*40; reward=v['default_block_reward_base_units']; cb=Transaction.new_coinbase_base_units(receiver,reward); b1=Block(index=1,previous_hash=genesis.hash,transactions=[cb],timestamp=1735689630,nonce=0,difficulty=0); balances,supply,reason=_state_from_chain([genesis,b1],reward,MAX_SUPPLY_BASE_UNITS); c=v['cases'][2]; check(balances==c['balances'] and supply==c['total_supply'] and reason==c['reason'],'issuance drift')
    cbmax=Transaction.new_coinbase_base_units(receiver,MAX_SUPPLY_BASE_UNITS); bmax=Block(index=1,previous_hash=genesis.hash,transactions=[cbmax],timestamp=1735689630,nonce=0,difficulty=0); _,supply,reason=_state_from_chain([genesis,bmax],MAX_SUPPLY_BASE_UNITS,MAX_SUPPLY_BASE_UNITS); check(supply==MAX_SUPPLY_BASE_UNITS,'max supply boundary drift')
    cbextra=Transaction.new_coinbase_base_units(receiver,1); bextra=Block(index=2,previous_hash=bmax.hash,transactions=[cbextra],timestamp=1735689660,nonce=0,difficulty=0); _,_,reason2=_state_from_chain([genesis,bmax,bextra],reward,MAX_SUPPLY_BASE_UNITS); check(reason2==v['cases'][4]['reason'],'exhaustion rule drift')
    txv=load('transaction.json'); tx=Transaction(sender=txv['sender'],receiver=txv['receiver'],amount_base_units=txv['amount_base_units'],timestamp=txv['timestamp'],sender_public_key=txv['sender_public_key'],signature=txv['fixed_signature'],tx_hash=txv['transaction_hash']); bad=Block(index=2,previous_hash=b1.hash,transactions=[tx],timestamp=1735689660,nonce=0,difficulty=0); _,_,reason3=_state_from_chain([genesis,b1,bad],reward,MAX_SUPPLY_BASE_UNITS); check(reason3==v['cases'][5]['reason'],'insufficient balance rule drift')
    check(not validate_base_units(MAX_SUPPLY_BASE_UNITS+1)[0],'over-supply amount accepted')

def main():
    funcs=[verify_genesis,verify_address,verify_tx,verify_merkle,verify_block_pow,verify_difficulty,verify_work,verify_monetary]
    for f in funcs: f(); print('PASS',f.__name__)
    print(f'Verified {sum(1 for _ in VEC.glob("*.json"))} vector files.')
if __name__=='__main__': main()
