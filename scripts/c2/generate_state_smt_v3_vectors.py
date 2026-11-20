#!/usr/bin/env python3
"""Independent Build 2A reference generator for the proposed V3 state SMT vectors.

Standard library only. This file deliberately does not import SAYANJALI BLOCKCHAIN code.
"""
from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass
from typing import Iterable

HASH = "sha256"
LEAF_TAG = b"SYJ-STATE-SMT-V3-LEAF\x00"
NODE_TAG = b"SYJ-STATE-SMT-V3-NODE\x00"
EMPTY_TAG = b"SYJ-STATE-SMT-V3-EMPTY\x00"
PROOF_TAG = b"SYJ-STATE-SMT-V3-PROOF\x00"
SPEC_VERSION = "state-smt-v3-vector-0.1"
DEPTH = 256


def h(data: bytes) -> bytes:
    return hashlib.sha256(data).digest()


def u32(v: int) -> bytes:
    return v.to_bytes(4, "big", signed=False)


def u64(v: int) -> bytes:
    return v.to_bytes(8, "big", signed=False)


def u16(v: int) -> bytes:
    return v.to_bytes(2, "big", signed=False)


def bit(key: bytes, level: int) -> int:
    """Return the direction bit at tree level 256..1, MSB first."""
    index = DEPTH - level
    return (key[index // 8] >> (7 - (index % 8))) & 1


EMPTY: list[bytes] = [b""] * (DEPTH + 1)
EMPTY[DEPTH] = h(EMPTY_TAG + u16(DEPTH))
for level in range(DEPTH - 1, -1, -1):
    EMPTY[level] = h(NODE_TAG + EMPTY[level + 1] + EMPTY[level + 1])


@dataclass(frozen=True)
class Account:
    address: str
    balance: int
    nonce: int

    def key(self) -> bytes:
        return h(self.address.encode("utf-8"))

    def leaf_bytes(self) -> bytes:
        address = self.address.encode("utf-8")
        return b"\x01" + self.key() + u32(len(address)) + address + u64(self.balance) + u64(self.nonce)

    def leaf_hash(self) -> bytes:
        return h(LEAF_TAG + self.leaf_bytes())


class SMT:
    def __init__(self) -> None:
        self.leaves: dict[bytes, Account] = {}

    def clone(self) -> "SMT":
        out = SMT()
        out.leaves = dict(self.leaves)
        return out

    def put(self, account: Account) -> dict:
        key = account.key()
        prior = self.leaves.get(key)
        if prior is not None and prior.address != account.address:
            raise ValueError("duplicate key from different addresses")
        self.leaves[key] = account
        return {"key": key.hex(), "leaf_hash": account.leaf_hash().hex()}

    def delete(self, address: str) -> dict:
        key = h(address.encode("utf-8"))
        prior = self.leaves.get(key)
        if prior is None:
            raise KeyError(address)
        if prior.address != address:
            raise ValueError("key/address mismatch")
        leaf = prior.leaf_hash().hex()
        del self.leaves[key]
        return {"key": key.hex(), "deleted_leaf_hash": leaf}

    def node_hashes(self) -> list[dict]:
        nodes: dict[tuple[int, int], bytes] = {}
        for key, account in self.leaves.items():
            nodes[(DEPTH, int.from_bytes(key, "big"))] = account.leaf_hash()
        for level in range(DEPTH - 1, -1, -1):
            width = 1 << (DEPTH - level)
            parents = {idx // 2 for (lvl, idx) in nodes if lvl == level + 1}
            for parent in parents:
                left = nodes.get((level + 1, parent * 2), EMPTY[level + 1])
                right = nodes.get((level + 1, parent * 2 + 1), EMPTY[level + 1])
                nodes[(level, parent)] = h(NODE_TAG + left + right)
        return [
            {"level": level, "index": index, "hash": value.hex()}
            for (level, index), value in sorted(nodes.items())
        ]

    def root(self) -> bytes:
        if not self.leaves:
            return EMPTY[0]
        return self._root_from(0, list(self.leaves.items()))

    def _root_from(self, level: int, items: list[tuple[bytes, Account]]) -> bytes:
        if level == DEPTH:
            return items[0][1].leaf_hash()
        left: list[tuple[bytes, Account]] = []
        right: list[tuple[bytes, Account]] = []
        for key, account in items:
            (right if bit(key, DEPTH - level) else left).append((key, account))
        lh = self._root_from(level + 1, left) if left else EMPTY[level + 1]
        rh = self._root_from(level + 1, right) if right else EMPTY[level + 1]
        return h(NODE_TAG + lh + rh)

    def proof(self, address: str) -> dict:
        key = h(address.encode("utf-8"))
        siblings: list[str] = []
        directions: list[int] = []
        current = list(self.leaves.items())
        for level in range(DEPTH):
            tree_level = DEPTH - level
            b = bit(key, tree_level)
            directions.append(b)
            same = [(k, a) for k, a in current if bit(k, tree_level) == b]
            other = [(k, a) for k, a in current if bit(k, tree_level) != b]
            if other:
                sibling = self._root_from(level + 1, other)
            else:
                sibling = EMPTY[level + 1]
            siblings.append(sibling.hex())
            current = same
        account = self.leaves.get(key)
        if account is not None and account.address != address:
            raise ValueError("duplicate key collision")
        leaf = account.leaf_hash() if account is not None else EMPTY[DEPTH]
        proof_kind = "inclusion" if account is not None else "non-inclusion"
        return {
            "query_address": address,
            "query_key": key.hex(),
            "proof_kind": proof_kind,
            "leaf_hash": leaf.hex(),
            "siblings_root_to_leaf": siblings,
            "directions_root_to_leaf": directions,
            "expected_root": self.root().hex(),
            "verification_result": self.verify_proof(key, leaf, siblings, directions, self.root()),
        }

    @staticmethod
    def verify_proof(key: bytes, leaf: bytes, siblings: list[str], directions: list[int], expected_root: bytes) -> bool:
        cur = leaf
        for sibling_hex, direction in zip(reversed(siblings), reversed(directions)):
            sibling = bytes.fromhex(sibling_hex)
            cur = h(NODE_TAG + sibling + cur) if direction == 1 else h(NODE_TAG + cur + sibling)
        return cur == expected_root


def account_obj(a: Account) -> dict:
    return {
        "address": a.address,
        "balance": a.balance,
        "nonce": a.nonce,
        "key": a.key().hex(),
        "leaf_bytes": a.leaf_bytes().hex(),
        "leaf_hash": a.leaf_hash().hex(),
    }


def op_result(smt: SMT, operation: str, account: Account | None = None, address: str | None = None) -> dict:
    if operation == "insert":
        assert account is not None
        result = smt.put(account)
    elif operation == "update":
        assert account is not None
        result = smt.put(account)
    elif operation == "delete":
        assert address is not None
        result = smt.delete(address)
    else:
        raise ValueError(operation)
    return {"operation": operation, **(account_obj(account) if account else {"address": address}), **result, "resulting_root": smt.root().hex()}


def deterministic_accounts(n: int) -> list[Account]:
    seed = b"SYJ-C2-V3-1000-ACCOUNTS-SEED-20261002"
    out = []
    for i in range(n):
        address = "SYJ" + hashlib.sha256(seed + u32(i)).hexdigest()[:40]
        balance = 1_000_000 + ((i * 1_000_003) % 9_000_000)
        nonce = i % 17
        out.append(Account(address, balance, nonce))
    return out


def find_long_prefix_pair() -> tuple[Account, Account, int]:
    first = Account("SYJprefixA", 100, 1)
    best: tuple[Account, Account, int] | None = None
    target = first.key()
    for i in range(1, 500_000):
        candidate = Account("SYJprefix" + hashlib.sha256(u32(i)).hexdigest()[:20], 200, 2)
        k = candidate.key()
        common = 0
        for a, b in zip(target, k):
            x = a ^ b
            if x == 0:
                common += 8
            else:
                common += 8 - x.bit_length()
                break
        if best is None or common > best[2]:
            best = (first, candidate, common)
        if common >= 20:
            return first, candidate, common
    assert best is not None
    return best


def main() -> None:
    long_a, long_b, common = find_long_prefix_pair()
    v: list[dict] = []

    empty = SMT()
    v.append({"id": "V1", "name": "empty_tree", "accounts": [], "root": empty.root().hex()})

    one = SMT()
    a1 = Account("SYJ1111111111111111111111111111111111111111", 1000, 7)
    one.put(a1)
    v.append({"id": "V2", "name": "one_account", "accounts": [account_obj(a1)], "root": one.root().hex(), "nodes": one.node_hashes()[:5] + one.node_hashes()[-5:]})

    two = SMT()
    two.put(long_a)
    two.put(long_b)
    v.append({"id": "V3", "name": "two_accounts_long_common_prefix", "accounts": [account_obj(long_a), account_obj(long_b)], "common_key_prefix_bits": common, "root": two.root().hex(), "nodes": two.node_hashes()})

    extremes = SMT()
    lo = Account("SYJ-low-extreme", 1, 0)
    hi = Account("SYJ-high-extreme", 2, 1)
    # Search deterministic addresses whose keys begin with 0 and 1 respectively.
    for i in range(100000):
        c = Account("SYJext" + str(i), i + 1, i % 5)
        if c.key()[0] == 0x00 and lo.address == "SYJ-low-extreme":
            lo = c
        if c.key()[0] == 0xFF and hi.address == "SYJ-high-extreme":
            hi = c
        if lo.address != "SYJ-low-extreme" and hi.address != "SYJ-high-extreme":
            break
    if lo.address == "SYJ-low-extreme" or hi.address == "SYJ-high-extreme":
        raise RuntimeError("deterministic extreme-key search did not find both 0x00 and 0xFF prefixes")
    extremes.put(lo); extremes.put(hi)
    v.append({"id": "V4", "name": "opposite_keyspace_extremes", "accounts": [account_obj(lo), account_obj(hi)], "root": extremes.root().hex(), "keys_first_byte": [lo.key()[0], hi.key()[0]], "nodes": extremes.node_hashes()})

    seq = SMT()
    ops = [
        op_result(seq, "insert", Account("SYJ-seq-a", 10, 0)),
        op_result(seq, "insert", Account("SYJ-seq-b", 20, 1)),
        op_result(seq, "insert", Account("SYJ-seq-c", 30, 2)),
    ]
    v.append({"id": "V5", "name": "insert_sequence", "operations": ops, "final_root": seq.root().hex()})

    upd = SMT()
    upd.put(Account("SYJ-update", 100, 3))
    update = op_result(upd, "update", Account("SYJ-update", 155, 4))
    update["affected_nodes"] = upd.node_hashes()
    v.append({"id": "V6", "name": "update_sequence", "initial": account_obj(Account("SYJ-update", 100, 3)), "operation": update, "final_root": upd.root().hex()})

    dele = SMT()
    dele.put(Account("SYJ-delete-a", 50, 1)); dele.put(Account("SYJ-delete-b", 60, 2))
    before = dele.root().hex()
    deletion = op_result(dele, "delete", address="SYJ-delete-a")
    deletion["affected_nodes"] = dele.node_hashes()
    v.append({"id": "V7", "name": "delete_sequence", "before_root": before, "deleted_address": "SYJ-delete-a", "operation": deletion, "final_root": dele.root().hex()})

    many_accounts = deterministic_accounts(1000)
    many = SMT()
    for account in many_accounts:
        many.put(account)
    v.append({
        "id": "V8", "name": "exactly_1000_deterministic_accounts", "generation": {
            "seed_hex": (b"SYJ-C2-V3-1000-ACCOUNTS-SEED-20261002").hex(),
            "address_rule": "SYJ + SHA256(seed || uint32_be(index)).hexdigest()[:40]",
            "balance_rule": "1000000 + ((index * 1000003) mod 9000000)",
            "nonce_rule": "index mod 17",
            "count": 1000,
        }, "count": 1000, "root": many.root().hex(), "accounts": [account_obj(a) for a in many_accounts]})

    proof_smt = SMT(); proof_smt.put(a1); proof_smt.put(long_b); proof_smt.put(lo); proof_smt.put(hi)
    inc = proof_smt.proof(a1.address)
    non = proof_smt.proof("SYJ-proof-absent-address")
    long_proof = proof_smt.proof(long_b.address)
    extreme_proof = proof_smt.proof(hi.address)
    v.append({"id": "V9", "name": "inclusion_proofs", "proofs": [inc, long_proof, extreme_proof]})
    v.append({"id": "V10", "name": "non_inclusion_proofs", "proofs": [non], "rule": "non-inclusion is proven by an empty leaf at the queried 256-bit key and its 256 sibling hashes; no neighboring leaf is required"})

    doc = {
        "schema": SPEC_VERSION,
        "purpose": "Build 2A frozen candidate SMT cryptographic/reference vectors; no production activation",
        "encoding": {
            "hash": "SHA-256",
            "address_encoding": "UTF-8 bytes of the exact address string",
            "key": "SHA-256(address_utf8)",
            "key_width_bits": 256,
            "leaf_fields": "version_u8 || key_32 || address_len_u32_be || address_utf8 || balance_u64_be || nonce_u64_be",
            "integer_encoding": "unsigned big-endian",
            "leaf_domain_tag_hex": LEAF_TAG.hex(),
            "node_domain_tag_hex": NODE_TAG.hex(),
            "empty_domain_tag_hex": EMPTY_TAG.hex(),
            "proof_domain_tag_hex": PROOF_TAG.hex(),
            "leaf_hash": "SHA256(LEAF_TAG || canonical_leaf_bytes)",
            "node_hash": "SHA256(NODE_TAG || left_hash || right_hash)",
            "empty_leaf_hash": "SHA256(EMPTY_TAG || uint16_be(256))",
            "empty_parent_recurrence": "empty[level] = SHA256(NODE_TAG || empty[level+1] || empty[level+1])",
            "level_convention": "level 256 is leaf; level 0 is root",
            "direction_convention": "MSB-first key bit; 0=left, 1=right",
            "root_encoding": "32 raw bytes represented as lowercase hexadecimal in vectors",
            "duplicate_key": "reject if different addresses hash to the same key; same-address update replaces the prior leaf",
            "duplicate_account": "a sequential operation on an existing address is an update; a static account set must not contain duplicate addresses",
            "delete": "remove the keyed leaf; the resulting path uses empty-subtree hashes",
            "proof_encoding": "queried key + leaf hash + 256 sibling hashes root-to-leaf + 256 direction bits",
            "inclusion": "leaf hash is the account leaf and reconstructed root equals expected root",
            "non_inclusion": "leaf hash is empty_leaf_hash and reconstructed root equals expected root; no neighbor leaf is required",
            "canonical_json": "vector JSON is UTF-8, 2-space indentation, sorted object keys, newline terminated; JSON is transport only, never cryptographic input",
        },
        "empty_subtree_hashes": [{"level": level, "hash": EMPTY[level].hex()} for level in range(DEPTH, -1, -1)],
        "vectors": v,
    }
    out = json.dumps(doc, indent=2, sort_keys=True, ensure_ascii=False) + "\n"
    with open("protocol/test-vectors/state-smt-v3.json", "w", encoding="utf-8", newline="\n") as f:
        f.write(out)
    print(hashlib.sha256(out.encode("utf-8")).hexdigest())
    print("vectors=10 accounts_v8=1000")


if __name__ == "__main__":
    main()
