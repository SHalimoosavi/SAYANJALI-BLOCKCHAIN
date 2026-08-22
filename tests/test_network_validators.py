"""Tests for the Phase 2 validator additions: chain_work and genesis identity."""

from __future__ import annotations

from blockchain.block import Block
from blockchain.blockchain import Blockchain
from blockchain.validators import chain_work, validate_genesis_identity
from blockchain.wallet import Wallet


def test_chain_work_sums_sixteen_to_the_difficulty(blockchain: Blockchain):
    # Genesis has difficulty 0 by default -> contributes 16**0 == 1.
    assert chain_work([blockchain.genesis_block]) == 1


def test_chain_work_exact_value_for_known_difficulty():
    # Difficulty counts required leading *hexadecimal* zero characters
    # (target_prefix = "0" * difficulty in ProofOfWorkConsensus), so each
    # level narrows the valid-hash space by a factor of 16, not 2.
    block = Block(index=1, previous_hash="a" * 64, difficulty=4)
    assert chain_work([block]) == 16 ** 4 == 65536


def test_chain_work_sums_across_multiple_blocks():
    blocks = [
        Block(index=1, previous_hash="a" * 64, difficulty=1),
        Block(index=2, previous_hash="b" * 64, difficulty=2),
        Block(index=3, previous_hash="c" * 64, difficulty=3),
    ]
    assert chain_work(blocks) == (16 ** 1) + (16 ** 2) + (16 ** 3)


def test_chain_work_increases_with_mined_blocks(blockchain: Blockchain):
    miner = Wallet.create()
    work_before = chain_work(blockchain.chain)
    blockchain.mine_pending_transactions(miner.address)
    work_after = chain_work(blockchain.chain)
    assert work_after > work_before


def test_chain_work_reflects_difficulty_not_just_length():
    low = Block(index=1, previous_hash="a" * 64, difficulty=1)
    high = Block(index=1, previous_hash="a" * 64, difficulty=5)
    # A single higher-difficulty block outweighs a single lower-difficulty
    # one by a wide margin under the correct hex-based (16x per level)
    # scale -- difficulty 5 vs difficulty 1 is a 16**4 = 65536x difference,
    # far more than the (now-corrected) previous base-2 assumption implied.
    assert chain_work([high]) > chain_work([low]) * 1000


def test_chain_work_scales_by_sixteen_per_difficulty_level():
    # Each additional difficulty level must multiply a single block's
    # contribution by exactly 16 -- the direct, precise regression check
    # for the base-2 -> base-16 correction.
    for difficulty in range(0, 5):
        lower = Block(index=1, previous_hash="a" * 64, difficulty=difficulty)
        higher = Block(index=1, previous_hash="a" * 64, difficulty=difficulty + 1)
        assert chain_work([higher]) == chain_work([lower]) * 16


def test_validate_genesis_identity_accepts_matching_genesis(blockchain: Blockchain):
    is_valid, reason = validate_genesis_identity(blockchain.chain, blockchain.genesis_block)
    assert is_valid, reason


def test_validate_genesis_identity_rejects_foreign_genesis(blockchain: Blockchain):
    foreign_genesis = Block.genesis(
        previous_hash="0" * 64,
        timestamp=1600000000.0,
        nonce=0,
        message="A DIFFERENT NETWORK ENTIRELY",
    )
    candidate = [foreign_genesis]
    is_valid, reason = validate_genesis_identity(candidate, blockchain.genesis_block)
    assert not is_valid
    assert "genesis" in reason.lower()


def test_validate_genesis_identity_rejects_empty_chain(blockchain: Blockchain):
    is_valid, reason = validate_genesis_identity([], blockchain.genesis_block)
    assert not is_valid
