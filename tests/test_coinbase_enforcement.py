"""
Phase 1 tests: coinbase/block-reward ENFORCEMENT.

Proves a block claiming any reward other than the exact protocol-configured
`block_reward` is rejected -- and that a block with zero or multiple
coinbase transactions is rejected -- closing the Phase 0-identified gap
where nothing checked the coinbase amount at all.
"""

from __future__ import annotations

import copy

from blockchain.block import Block
from blockchain.blockchain import Blockchain
from blockchain.transaction import Transaction
from blockchain.validators import validate_block_against_chain, validate_block_structure
from blockchain.wallet import Wallet


def _block_with_coinbase_amount(blockchain: Blockchain, miner_address: str, amount: float) -> Block:
    """Mine a normal, valid block, then substitute a different coinbase amount."""
    block = blockchain.mine_pending_transactions(miner_address)
    tampered_coinbase = Transaction(
        sender=block.transactions[0].sender,
        receiver=block.transactions[0].receiver,
        amount=amount,
        timestamp=block.transactions[0].timestamp,
    )
    new_block = Block(
        index=block.index, previous_hash=block.previous_hash,
        transactions=[tampered_coinbase] + block.transactions[1:],
        timestamp=block.timestamp, nonce=block.nonce, difficulty=block.difficulty,
    )
    return new_block


def test_exact_valid_reward_accepted(blockchain: Blockchain):
    miner = Wallet.create()
    reward = blockchain.settings.mining.block_reward
    prefix = list(blockchain.chain)
    block = blockchain.mine_pending_transactions(miner.address)  # honest mine, exact reward

    is_valid, reason = validate_block_against_chain(
        block, prefix, blockchain.consensus, blockchain.settings.consensus, reward,
    )
    assert is_valid, reason


def test_reward_plus_one_rejected(blockchain: Blockchain):
    reward = blockchain.settings.mining.block_reward
    miner = Wallet.create()

    is_valid, reason = validate_block_structure(
        _block_with_coinbase_amount(blockchain, miner.address, reward + 1.0),
        expected_coinbase_reward=reward,
    )
    assert not is_valid
    assert "coinbase" in reason.lower() or "reward" in reason.lower()


def test_reward_far_above_allowed_rejected(blockchain: Blockchain):
    reward = blockchain.settings.mining.block_reward
    miner = Wallet.create()

    is_valid, reason = validate_block_structure(
        _block_with_coinbase_amount(blockchain, miner.address, reward * 1000),
        expected_coinbase_reward=reward,
    )
    assert not is_valid


def test_reward_below_configured_rejected(blockchain: Blockchain):
    """
    An under-claimed reward is also rejected -- exact match, not merely a
    ceiling -- since the protocol's issuance schedule is a fixed value,
    not merely a maximum.
    """
    reward = blockchain.settings.mining.block_reward
    miner = Wallet.create()

    is_valid, reason = validate_block_structure(
        _block_with_coinbase_amount(blockchain, miner.address, reward / 2),
        expected_coinbase_reward=reward,
    )
    assert not is_valid


def test_zero_coinbase_transactions_rejected(blockchain: Blockchain):
    """A non-genesis block with no coinbase transaction at all must be rejected."""
    miner = Wallet.create()
    receiver = Wallet.create()
    tx = Transaction(sender=miner.address, receiver=receiver.address, amount=1.0)
    tx.sign(miner)

    block = Block(index=1, previous_hash=blockchain.chain[0].hash, transactions=[tx])
    is_valid, reason = validate_block_structure(block, expected_coinbase_reward=blockchain.settings.mining.block_reward)
    assert not is_valid
    assert "coinbase" in reason.lower()


def test_duplicate_coinbase_transactions_rejected(blockchain: Blockchain):
    """Regression: more than one coinbase transaction remains rejected."""
    miner = Wallet.create()
    reward = blockchain.settings.mining.block_reward
    coinbase_1 = Transaction.new_coinbase(miner.address, reward)
    coinbase_2 = Transaction.new_coinbase(miner.address, reward)

    block = Block(index=1, previous_hash=blockchain.chain[0].hash, transactions=[coinbase_1, coinbase_2])
    is_valid, reason = validate_block_structure(block, expected_coinbase_reward=reward)
    assert not is_valid
    assert "more than one" in reason.lower()


def test_malformed_coinbase_amount_rejected(blockchain: Blockchain):
    """A non-positive coinbase amount is caught by ordinary transaction validation too."""
    miner = Wallet.create()
    reward = blockchain.settings.mining.block_reward

    is_valid, reason = validate_block_structure(
        _block_with_coinbase_amount(blockchain, miner.address, -5.0),
        expected_coinbase_reward=reward,
    )
    assert not is_valid


def test_invalid_reward_block_rejected_via_full_chain_validation(blockchain: Blockchain):
    """The exact required behavior: INVALID BLOCK REWARD -> BLOCK REJECTED, at the chain-validation entry point."""
    miner = Wallet.create()
    reward = blockchain.settings.mining.block_reward
    prefix = list(blockchain.chain)
    bad_block = _block_with_coinbase_amount(blockchain, miner.address, reward * 10)

    is_valid, reason = validate_block_against_chain(
        bad_block, prefix, blockchain.consensus, blockchain.settings.consensus, reward,
    )
    assert not is_valid
    # bad_block itself (the tampered copy) was never submitted through
    # Blockchain.mine_pending_transactions/replace_chain, so this asserts
    # only what actually matters: the independent validation call above
    # correctly rejects it. (The helper's own honest mine already
    # advanced blockchain.length separately -- that's expected, not a
    # side effect of the rejected block.)


def test_manipulated_transaction_list_with_extra_coinbase_rejected(blockchain: Blockchain):
    """A block trying to sneak in a second, smaller-looking coinbase alongside a legitimate one is still rejected."""
    miner = Wallet.create()
    reward = blockchain.settings.mining.block_reward
    block = blockchain.mine_pending_transactions(miner.address)

    sneaky_coinbase = Transaction.new_coinbase(Wallet.create().address, 0.01)
    tampered = Block(
        index=block.index, previous_hash=block.previous_hash,
        transactions=list(block.transactions) + [sneaky_coinbase],
        timestamp=block.timestamp, nonce=block.nonce, difficulty=block.difficulty,
    )
    is_valid, reason = validate_block_structure(tampered, expected_coinbase_reward=reward)
    assert not is_valid
    assert "more than one" in reason.lower()


def test_reward_enforcement_survives_retarget_windows(blockchain: Blockchain):
    """The exact reward requirement holds across multiple blocks/retarget windows, not just the first block."""
    miner = Wallet.create()
    reward = blockchain.settings.mining.block_reward
    for _ in range(5):
        blockchain.mine_pending_transactions(miner.address)

    is_valid, reason = blockchain.is_chain_valid()
    assert is_valid, reason
    for block in blockchain.chain[1:]:
        coinbase = next(tx for tx in block.transactions if tx.is_coinbase())
        assert coinbase.amount == reward


def test_reward_enforcement_during_reorg(blockchain: Blockchain):
    """A competing chain with a manipulated reward on one of its blocks must not be adopted via replace_chain."""
    from blockchain.mining import Miner
    from blockchain.validators import expected_difficulty

    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)

    attacker = Wallet.create()
    engine = Miner(blockchain.consensus, blockchain.settings.mining.block_reward)
    candidate = [blockchain.chain[0]]
    for i in range(1, 4):
        required = expected_difficulty(candidate, blockchain.consensus, blockchain.settings.consensus)
        new_block, _ = engine.mine_block(
            index=i, previous_hash=candidate[-1].hash, mempool=blockchain.mempool,
            miner_address=attacker.address, difficulty=required,
        )
        candidate.append(new_block)

    # Tamper with the last block's coinbase amount after mining.
    reward = blockchain.settings.mining.block_reward
    last = candidate[-1]
    tampered_coinbase = Transaction(
        sender=last.transactions[0].sender, receiver=last.transactions[0].receiver,
        amount=reward * 100, timestamp=last.transactions[0].timestamp,
    )
    candidate[-1] = Block(
        index=last.index, previous_hash=last.previous_hash,
        transactions=[tampered_coinbase] + last.transactions[1:],
        timestamp=last.timestamp, nonce=last.nonce, difficulty=last.difficulty,
    )

    accepted, reason = blockchain.replace_chain(candidate)
    assert not accepted
