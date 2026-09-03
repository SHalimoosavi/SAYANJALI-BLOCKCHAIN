"""Tests for the Phase 2 upgrade to Blockchain.replace_chain."""

from __future__ import annotations

import copy

from blockchain.blockchain import Blockchain
from blockchain.validators import chain_work, expected_difficulty
from blockchain.wallet import Wallet


def _extend_chain(source: Blockchain, miner_address: str, blocks: int) -> list:
    """Mine `blocks` additional blocks on `source` and return its chain copy."""
    for _ in range(blocks):
        source.mine_pending_transactions(miner_address)
    return copy.deepcopy(source.chain)


def test_replace_chain_rejects_inferior_work(blockchain: Blockchain):
    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)
    local_length_before = blockchain.length

    # A candidate that is just the genesis block alone has less work.
    accepted, reason = blockchain.replace_chain([blockchain.genesis_block])
    assert not accepted
    assert blockchain.length == local_length_before


def test_replace_chain_accepts_superior_work(blockchain: Blockchain):
    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)

    candidate = _extend_chain(blockchain, miner.address, blocks=2)
    # Revert local chain to simulate this node being behind.
    blockchain.chain = blockchain.chain[:2]

    accepted, reason = blockchain.replace_chain(candidate)
    assert accepted, reason
    assert blockchain.length == len(candidate)
    assert blockchain.latest_block.hash == candidate[-1].hash


def test_replace_chain_rejects_foreign_genesis(blockchain: Blockchain):
    from blockchain.block import Block

    foreign_genesis = Block.genesis(
        previous_hash="0" * 64, timestamp=1600000000.0, nonce=0, message="OTHER NETWORK"
    )
    accepted, reason = blockchain.replace_chain([foreign_genesis])
    assert not accepted
    assert "genesis" in reason.lower()


def test_replace_chain_rejects_invalid_candidate(blockchain: Blockchain):
    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)
    candidate = _extend_chain(blockchain, miner.address, blocks=2)
    blockchain.chain = blockchain.chain[:2]

    # Corrupt the candidate so it fails structural validation.
    candidate[-1].hash = "corrupted" * 8

    accepted, reason = blockchain.replace_chain(candidate)
    assert not accepted


def test_replace_chain_rejects_equal_work(blockchain: Blockchain):
    """
    A candidate with exactly the same accumulated work as the local chain
    must be rejected -- replace_chain requires strictly greater work
    (`candidate_work <= local_work` is a rejection), not merely
    non-inferior work. An identical clone of the local chain is the most
    direct way to exercise that boundary.
    """
    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)

    identical_clone = copy.deepcopy(blockchain.chain)
    local_work_before = blockchain.total_work()

    accepted, reason = blockchain.replace_chain(identical_clone)
    assert not accepted
    assert "work" in reason.lower()
    assert blockchain.total_work() == local_work_before


def test_replace_chain_reorg_updates_balances_correctly(blockchain: Blockchain):
    """
    A true reorg (candidate diverges below the local tip) must correctly
    reverse the local fork's balance effects and apply the adopted fork's
    effects, not just append on top.
    """
    from blockchain.mining import Miner

    original_miner = Wallet.create()
    blockchain.mine_pending_transactions(original_miner.address)  # block 1: local-only fork

    # Build a competing, heavier fork purely in-memory (never touching
    # storage directly), starting from the shared genesis block, mined by
    # a different address, so adopting it constitutes a true reorg. Each
    # block is mined at its actual protocol-expected difficulty (Phase 1:
    # validation now independently derives and enforces this exact value
    # per position, rather than trusting a static config literal), which
    # still accumulates more real work under the correct 16**difficulty
    # scale than the single local-fork block -- a handful of low-difficulty
    # blocks would not outweigh a single higher-difficulty one, which is
    # exactly the point of comparing work rather than length.
    competing_miner = Wallet.create()
    miner_engine = Miner(blockchain.consensus, blockchain.settings.mining.block_reward)
    candidate = [blockchain.chain[0]]
    for i in range(1, 4):
        required = expected_difficulty(candidate, blockchain.consensus, blockchain.settings.consensus)
        new_block, _ = miner_engine.mine_block(
            index=i,
            previous_hash=candidate[-1].hash,
            mempool=blockchain.mempool,
            miner_address=competing_miner.address,
            difficulty=required,
        )
        candidate.append(new_block)

    assert chain_work(candidate) > blockchain.total_work()  # sanity check the fixture itself

    accepted, reason = blockchain.replace_chain(candidate)
    assert accepted, reason

    # Original miner's reward from the discarded block must be reversed.
    assert blockchain.get_balance(original_miner.address) == 0
    # Competing miner should now hold rewards for all 3 adopted blocks.
    expected = blockchain.settings.mining.block_reward * 3
    assert blockchain.get_balance(competing_miner.address) == expected


def test_replace_chain_removes_confirmed_transactions_from_mempool(blockchain: Blockchain):
    from blockchain.transaction import Transaction

    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)

    receiver = Wallet.create()
    tx = Transaction(sender=miner.address, receiver=receiver.address, amount=1.0)
    tx.sign(miner)
    blockchain.mempool.add_transaction(tx)
    assert blockchain.mempool.contains(tx.tx_hash)

    # Build a competing chain that happens to include the same transaction,
    # confirming it, and adopt it.
    from blockchain.mining import Miner

    candidate = copy.deepcopy(blockchain.chain)
    miner_engine = Miner(blockchain.consensus, blockchain.settings.mining.block_reward)
    new_block, _ = miner_engine.mine_block(
        index=len(candidate),
        previous_hash=candidate[-1].hash,
        mempool=blockchain.mempool,
        miner_address=miner.address,
        difficulty=1,
    )
    candidate.append(new_block)

    accepted, reason = blockchain.replace_chain(candidate)
    assert accepted, reason
    assert not blockchain.mempool.contains(tx.tx_hash)
