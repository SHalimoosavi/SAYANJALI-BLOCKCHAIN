#!/usr/bin/env python3
"""Generate deterministic SYJ protocol compatibility vectors from Python reference code."""
from __future__ import annotations

import json
import sys
from fractions import Fraction
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from blockchain.block import Block
from blockchain.consensus import ProofOfWorkConsensus
from blockchain.native_asset import (
    BASE_UNITS_PER_SYJ, MAX_SUPPLY_BASE_UNITS, MAX_SUPPLY_SYJ,
    format_amount, from_base_units, to_base_units, validate_base_units,
)
from blockchain.transaction import COINBASE_SENDER, Transaction
from blockchain.utils import deterministic_json, merkle_root, sha256
from blockchain.validators import _state_from_chain, chain_work
from blockchain.wallet import Wallet, derive_address, verify_signature
from config.settings import get_settings

VEC = ROOT / "protocol" / "test-vectors"
VEC.mkdir(parents=True, exist_ok=True)


def write(name: str, payload: dict) -> None:
    (VEC / name).write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def hex_bytes(s: str) -> str:
    return s.encode("utf-8").hex()


def fixed_transaction() -> tuple[Wallet, Transaction]:
    wallet = Wallet.from_private_key("01".zfill(64))
    receiver = "SYJ" + "1" * 40
    tx = Transaction(sender=wallet.address, receiver=receiver, amount_base_units=123456789, timestamp=1735689601.25)
    tx.sender_public_key = wallet.public_key_hex
    # Frozen once as a compatibility fixture because Wallet.sign() is intentionally non-deterministic.
    tx.signature = "e66e9a40bea0e745c354af08235f2d61e119f7bdfc9db8532a8c7f67138190d8197cb130b5a654f0e883fd4c6572d64d80e42e8562bd2a06c78018e3e02994c4"
    tx.tx_hash = tx.compute_hash()
    assert tx.verify()
    return wallet, tx


def gen_genesis() -> None:
    s = get_settings()
    g = Block.genesis(s.genesis.previous_hash, s.genesis.timestamp, s.genesis.nonce, s.genesis.message)
    header = g.header_payload()
    write("genesis.json", {
        "vector_format_version": 1,
        "reference": "Python implementation",
        "inputs": {
            "index": s.genesis.index,
            "previous_hash": s.genesis.previous_hash,
            "timestamp": s.genesis.timestamp,
            "nonce": s.genesis.nonce,
            "difficulty": g.difficulty,
            "message": s.genesis.message,
        },
        "genesis_transaction_hash": g.transactions[0].tx_hash,
        "header_payload": header,
        "canonical_header_json": deterministic_json(header),
        "canonical_header_utf8_hex": hex_bytes(deterministic_json(header)),
        "block_hash": g.hash,
        "genesis_allocation": "undefined; no allocation is frozen by this vector",
    })


def gen_address() -> None:
    w = Wallet.from_private_key("01".zfill(64))
    write("address.json", {
        "vector_format_version": 1,
        "curve": "SECP256k1",
        "private_key_hex": w.private_key_hex,
        "public_key_representation": "VerifyingKey.to_string().hex()",
        "public_key_hex": w.public_key_hex,
        "address_rule": '"SYJ" + first 40 hex chars of SHA-256(UTF-8(public_key_hex))',
        "public_key_utf8_hex": hex_bytes(w.public_key_hex),
        "public_key_sha256": sha256(w.public_key_hex),
        "address": w.address,
    })


def gen_transaction() -> None:
    w, tx = fixed_transaction()
    payload = tx._signing_payload()
    message = tx.signing_message()
    hash_payload = dict(payload, sender_public_key=tx.sender_public_key, signature=tx.signature)
    write("transaction.json", {
        "vector_format_version": 1,
        "signature_generation": "not deterministic; fixed signature fixture is normative for this vector",
        "curve": "SECP256k1",
        "private_key_hex": w.private_key_hex,
        "public_key_hex": w.public_key_hex,
        "sender": tx.sender,
        "receiver": tx.receiver,
        "amount_base_units": tx.amount_base_units,
        "timestamp": tx.timestamp,
        "signing_payload": payload,
        "canonical_signing_message": message,
        "canonical_signing_message_utf8_hex": hex_bytes(message),
        "sender_public_key": tx.sender_public_key,
        "fixed_signature": tx.signature,
        "signature_verifies": verify_signature(tx.sender_public_key, message, tx.signature),
        "transaction_hash_payload": hash_payload,
        "canonical_transaction_hash_json": deterministic_json(hash_payload),
        "canonical_transaction_hash_utf8_hex": hex_bytes(deterministic_json(hash_payload)),
        "transaction_hash": tx.tx_hash,
    })


def gen_merkle() -> None:
    _, tx = fixed_transaction()
    leaves = [tx.tx_hash, sha256("SYJ-vector-leaf-2"), sha256("SYJ-vector-leaf-3"), sha256("SYJ-vector-leaf-4"), sha256("SYJ-vector-leaf-5")]
    cases = []
    for name, vals in [
        ("empty", []), ("one", leaves[:1]), ("two", leaves[:2]), ("three", leaves[:3]), ("five", leaves[:5])
    ]:
        cases.append({"name": name, "transaction_hashes": vals, "expected_merkle_root": merkle_root(vals)})
    write("merkle.json", {"vector_format_version": 1, "leaf_rule": "transaction hash strings", "odd_rule": "duplicate final hash at each odd level", "empty_rule": "SHA-256(empty UTF-8 string)", "cases": cases})


def gen_block_pow() -> None:
    _, tx = fixed_transaction()
    b = Block(index=1, previous_hash="a" * 64, transactions=[tx], timestamp=1735689631.25, nonce=104, difficulty=2)
    header = b.header_payload()
    write("block.json", {
        "vector_format_version": 1,
        "header_fields": ["index", "previous_hash", "timestamp", "nonce", "difficulty", "merkle_root"],
        "header_payload": header,
        "canonical_header_json": deterministic_json(header),
        "canonical_header_utf8_hex": hex_bytes(deterministic_json(header)),
        "expected_block_hash": b.hash,
        "merkle_root": b.merkle_root,
    })
    pow_cases = []
    for d in [0, 1, 2, 3]:
        pow_cases.append({"difficulty": d, "hash": b.hash, "meets_difficulty": b.meets_difficulty(d)})
    write("pow.json", {"vector_format_version": 1, "rule": "hash starts with difficulty count of hexadecimal '0' characters", "cases": pow_cases})


def difficulty_case(name: str, first: float, last: float, base: int) -> dict:
    blocks = [Block(index=i, previous_hash="0" * 64, timestamp=first if i == 0 else last if i == 9 else first + (last-first) * i // 9, nonce=0, difficulty=base) for i in range(10)]
    engine = ProofOfWorkConsensus()
    expected = 9 * 30
    actual = last - first
    if actual <= 0: actual = 1
    min_ts = max(1, expected // 4); max_ts = expected * 4
    clamped = max(min_ts, min(int(actual), max_ts))
    current_work = 16 ** max(base, 0)
    target = Fraction(current_work * expected, clamped)
    final = engine.next_difficulty(blocks, 4, 30, 1, 32, 4)
    return {"name": name, "window_size": 10, "first_timestamp": first, "last_timestamp": last, "actual_timespan": last-first, "expected_timespan": expected, "clamped_timespan": clamped, "previous_difficulty": base, "current_work": current_work, "target_work_numerator": target.numerator, "target_work_denominator": target.denominator, "target_work": f"{target.numerator}/{target.denominator}", "expected_difficulty": final}


def gen_difficulty() -> None:
    cases = [
        difficulty_case("normal", 1000, 1270, 4),
        difficulty_case("faster_than_target", 1000, 1068, 4),
        difficulty_case("slower_than_target", 1000, 2080, 4),
        difficulty_case("extreme_fast_clamped", 1000, 1001, 4),
        difficulty_case("extreme_slow_clamped", 1000.0, 1000.0 + 5000.0, 4),
        difficulty_case("minimum_floor", 1000, 2080, 1),
        difficulty_case("maximum_ceiling", 1000, 1068, 32),
    ]
    write("difficulty.json", {"vector_format_version": 1, "configuration": {"target_block_time": 30, "adjustment_interval": 10, "min_difficulty": 1, "max_difficulty": 32, "max_adjustment_factor": 4}, "cases": cases})


def gen_chain_work() -> None:
    difficulties = [0, 1, 2, 4, 8]
    blocks = [Block(index=i, previous_hash="0" * 64, timestamp=1735689600.0 + i, nonce=0, difficulty=d) for i, d in enumerate(difficulties)]
    write("chain_work.json", {"vector_format_version": 1, "rule": "sum(16 ** max(block.difficulty, 0))", "cases": [{"difficulties": difficulties, "per_block_work": [16 ** d for d in difficulties], "accumulated_work": chain_work(blocks)}, {"difficulties": [3, 3, 4], "per_block_work": [16**3,16**3,16**4], "accumulated_work": 16**3+16**3+16**4}]})


def gen_monetary() -> None:
    _, tx = fixed_transaction()
    valid = validate_base_units(123456789)
    reward = to_base_units("50.0")
    receiver = "SYJ" + "2" * 40
    genesis = Block.genesis("0" * 64, 1735689600.0, 0, get_settings().genesis.message)
    cb = Transaction.new_coinbase_base_units(receiver, reward)
    b1 = Block(index=1, previous_hash=genesis.hash, transactions=[cb], timestamp=1735689630.0, nonce=0, difficulty=0)
    balances, supply, reason = _state_from_chain([genesis, b1], reward, MAX_SUPPLY_BASE_UNITS)
    cb_max = Transaction.new_coinbase_base_units(receiver, MAX_SUPPLY_BASE_UNITS)
    bmax = Block(index=1, previous_hash=genesis.hash, transactions=[cb_max], timestamp=1735689630.0, nonce=0, difficulty=0)
    cb_extra = Transaction.new_coinbase_base_units(receiver, 1)
    bextra = Block(index=2, previous_hash=bmax.hash, transactions=[cb_extra], timestamp=1735689660.0, nonce=0, difficulty=0)
    _, capped_supply, cap_reason = _state_from_chain([genesis, bmax], MAX_SUPPLY_BASE_UNITS, MAX_SUPPLY_BASE_UNITS)
    _, _, exhausted_reason = _state_from_chain([genesis, bmax, bextra], reward, MAX_SUPPLY_BASE_UNITS)
    _, _, insufficient_reason = _state_from_chain([genesis, b1, Block(index=2, previous_hash=b1.hash, transactions=[tx], timestamp=1735689660.0, nonce=0, difficulty=0)], reward, MAX_SUPPLY_BASE_UNITS)
    write("monetary.json", {"vector_format_version": 1, "symbol": "SYJ", "base_units_per_syj": BASE_UNITS_PER_SYJ, "max_supply_syj": str(MAX_SUPPLY_SYJ), "max_supply_base_units": MAX_SUPPLY_BASE_UNITS, "default_block_reward_syj": "50", "default_block_reward_base_units": reward, "cases": [
        {"name":"conversion", "inputs":["1","50.0","0.00000001","720000000"], "base_units":[to_base_units(x) for x in ["1","50.0","0.00000001","720000000"]], "round_trip":[str(from_base_units(to_base_units(x))) for x in ["1","50.0","0.00000001","720000000"]]},
        {"name":"format_amount", "base_units":123456789, "formatted":format_amount(123456789)},
        {"name":"single_reward_issuance", "balances":balances, "total_supply":supply, "reason":reason},
        {"name":"maximum_supply_boundary", "total_supply":capped_supply, "reason":cap_reason},
        {"name":"post_exhaustion_issuance_rejected", "reason":exhausted_reason},
        {"name":"insufficient_balance_rejected", "reason":insufficient_reason},
        {"name":"invalid_conversion_over_supply", "valid":validate_base_units(MAX_SUPPLY_BASE_UNITS+1)[0], "reason":validate_base_units(MAX_SUPPLY_BASE_UNITS+1)[1]},
    ]})


def main() -> None:
    gen_genesis(); gen_address(); gen_transaction(); gen_merkle(); gen_block_pow(); gen_difficulty(); gen_chain_work(); gen_monetary()
    print(f"Generated {len(list(VEC.glob('*.json')))} vector files in {VEC}")

if __name__ == "__main__": main()
