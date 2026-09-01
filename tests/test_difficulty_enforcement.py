"""
Phase 1 tests: consensus difficulty ENFORCEMENT during validation.

These are distinct from tests/test_consensus_retarget.py, which tests the
retarget *calculation* in isolation. These tests prove the calculation is
actually enforced: that a block whose self-declared difficulty doesn't
match the protocol-required value for its chain position is rejected,
regardless of whether its hash satisfies that self-declared value.
"""

from __future__ import annotations

from blockchain.blockchain import Blockchain
from blockchain.mining import Miner
from blockchain.network.handshake import AuthContext, build_auth_envelope
from blockchain.network.identity import P2PIdentity
from blockchain.network.node import NetworkNode
from blockchain.network.propagation import receive_block
from blockchain.validators import (
    expected_difficulty,
    validate_block_against_chain,
    validate_chain,
)
from blockchain.wallet import Wallet

PEER_ADDRESS = "http://127.0.0.1:9001"


def _mine_at(blockchain: Blockchain, index: int, previous_hash: str, difficulty: int, miner_address: str):
    engine = Miner(blockchain.consensus, blockchain.settings.mining.block_reward)
    block, _ = engine.mine_block(
        index=index, previous_hash=previous_hash, mempool=blockchain.mempool,
        miner_address=miner_address, difficulty=difficulty,
    )
    return block


# --------------------------------------------------------------------- #
# Direct validation-layer enforcement
# --------------------------------------------------------------------- #


def test_block_at_exact_expected_difficulty_accepted(blockchain: Blockchain):
    miner = Wallet.create()
    required = expected_difficulty(blockchain.chain, blockchain.consensus, blockchain.settings.consensus)
    block = _mine_at(blockchain, blockchain.latest_block.index + 1, blockchain.latest_block.hash, required, miner.address)

    is_valid, reason = validate_block_against_chain(
        block, blockchain.chain, blockchain.consensus, blockchain.settings.consensus,
        blockchain.settings.mining.block_reward,
    )
    assert is_valid, reason


def test_artificially_low_difficulty_rejected(blockchain: Blockchain):
    """The exact Phase 0 gap: a block mined at a trivially low difficulty
    must be rejected even though its hash genuinely satisfies that low
    value -- the check is against the protocol-expected value, not a floor."""
    miner = Wallet.create()
    required = expected_difficulty(blockchain.chain, blockchain.consensus, blockchain.settings.consensus)
    artificially_low = max(1, required - 1) if required > 1 else 1

    # Force a genuinely mismatched case: if required is already at the
    # minimum, use the block's own low difficulty directly (still != required
    # is only meaningful if required > 1; guard by asserting the setup).
    if required <= 1:
        import pytest
        pytest.skip("Test fixture's expected difficulty is already at the minimum; no lower value to test against.")

    block = _mine_at(blockchain, blockchain.latest_block.index + 1, blockchain.latest_block.hash, artificially_low, miner.address)

    is_valid, reason = validate_block_against_chain(
        block, blockchain.chain, blockchain.consensus, blockchain.settings.consensus,
        blockchain.settings.mining.block_reward,
    )
    assert not is_valid
    assert "difficulty" in reason.lower()


def test_artificially_high_difficulty_rejected(blockchain: Blockchain, monkeypatch):
    """
    A block claiming MORE work than the protocol requires must also be
    rejected -- exact match, not merely a floor -- even though mining at
    higher difficulty is "harder." Accepting it would let a miner
    unilaterally decide the network's difficulty schedule.
    """
    miner = Wallet.create()
    required = expected_difficulty(blockchain.chain, blockchain.consensus, blockchain.settings.consensus)
    artificially_high = required + 1

    block = _mine_at(blockchain, blockchain.latest_block.index + 1, blockchain.latest_block.hash, artificially_high, miner.address)

    is_valid, reason = validate_block_against_chain(
        block, blockchain.chain, blockchain.consensus, blockchain.settings.consensus,
        blockchain.settings.mining.block_reward,
    )
    assert not is_valid
    assert "difficulty" in reason.lower()


def test_mismatched_difficulty_on_propagated_block_rejected(blockchain: Blockchain):
    """The same enforcement must apply to blocks received from peers, not just local validation."""
    node = NetworkNode(blockchain)
    identity = P2PIdentity.generate("peer-x")
    ctx = AuthContext(
        identity=identity, network_name=node.settings.network_name,
        genesis_hash=node.blockchain.genesis_block.hash, advertised_address=PEER_ADDRESS,
    )
    blockchain.storage.upsert_peer_credential(
        address=PEER_ADDRESS, node_id=identity.node_id,
        public_key_hex=identity.public_key_hex, trusted=True,
    )

    miner = Wallet.create()
    required = expected_difficulty(blockchain.chain, blockchain.consensus, blockchain.settings.consensus)
    if required <= 1:
        import pytest
        pytest.skip("Fixture's expected difficulty already at minimum.")

    bad_block = _mine_at(blockchain, blockchain.latest_block.index + 1, blockchain.latest_block.hash, required - 1, miner.address)
    envelope = build_auth_envelope(ctx, bad_block.to_dict())

    accepted, reason, _ = receive_block(node, bad_block.to_dict(), PEER_ADDRESS, envelope)
    assert not accepted
    assert "difficulty" in reason.lower()
    assert blockchain.length == 1  # nothing was appended


def test_mismatched_difficulty_on_synchronized_chain_rejected(blockchain: Blockchain):
    """A full candidate chain with one manipulated-difficulty block must fail chain-wide validation."""
    miner = Wallet.create()
    candidate = list(blockchain.chain)
    required = expected_difficulty(candidate, blockchain.consensus, blockchain.settings.consensus)
    block = _mine_at(blockchain, candidate[-1].index + 1, candidate[-1].hash, required, miner.address)
    candidate.append(block)

    # Tamper with the recorded difficulty after mining (still self-consistent
    # with its own hash requirement at the lower value, since we recompute).
    from blockchain.consensus import ProofOfWorkConsensus

    tampered_engine = ProofOfWorkConsensus()
    retry_block = tampered_engine.mine(
        type(block)(index=block.index, previous_hash=block.previous_hash, transactions=block.transactions, timestamp=block.timestamp),
        difficulty=max(1, required - 1) if required > 1 else 1,
    )
    if retry_block.difficulty == required:
        import pytest
        pytest.skip("Fixture's expected difficulty already at minimum; cannot construct a mismatched case.")
    candidate[-1] = retry_block

    is_valid, reason = validate_chain(
        candidate, blockchain.consensus, blockchain.settings.consensus,
        blockchain.settings.mining.block_reward,
    )
    assert not is_valid
    assert "difficulty" in reason.lower()


def test_difficulty_enforcement_at_retarget_boundary(monkeypatch):
    """Manipulation exactly at a retarget window boundary must still be caught."""
    monkeypatch.setenv("SYJ_DIFFICULTY", "3")  # high enough for headroom, low enough to mine fast
    monkeypatch.setenv("SYJ_DIFFICULTY_ADJUSTMENT_INTERVAL", "2")

    from config import settings as settings_module

    settings_module.get_settings.cache_clear()
    blockchain = Blockchain()

    interval = blockchain.settings.consensus.difficulty_adjustment_interval
    miner = Wallet.create()

    for _ in range(interval):
        blockchain.mine_pending_transactions(miner.address)

    required = expected_difficulty(blockchain.chain, blockchain.consensus, blockchain.settings.consensus)
    assert required > 1  # guaranteed by the high starting difficulty and small clamp headroom

    bad_block = _mine_at(blockchain, blockchain.latest_block.index + 1, blockchain.latest_block.hash, required - 1, miner.address)
    is_valid, reason = validate_block_against_chain(
        bad_block, blockchain.chain, blockchain.consensus, blockchain.settings.consensus,
        blockchain.settings.mining.block_reward,
    )
    assert not is_valid

    settings_module.get_settings.cache_clear()


def test_difficulty_enforcement_between_retargets(monkeypatch):
    """Manipulation on a block that is NOT itself a retarget boundary must still be caught."""
    monkeypatch.setenv("SYJ_DIFFICULTY", "3")  # deliberately high enough so a "-1" mismatch is always available

    from config import settings as settings_module

    settings_module.get_settings.cache_clear()
    blockchain = Blockchain()

    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)  # one honest block first

    required = expected_difficulty(blockchain.chain, blockchain.consensus, blockchain.settings.consensus)
    assert required > 1  # guaranteed by the high starting difficulty, not incidental timing

    bad_block = _mine_at(blockchain, blockchain.latest_block.index + 1, blockchain.latest_block.hash, required - 1, miner.address)
    is_valid, reason = validate_block_against_chain(
        bad_block, blockchain.chain, blockchain.consensus, blockchain.settings.consensus,
        blockchain.settings.mining.block_reward,
    )
    assert not is_valid

    settings_module.get_settings.cache_clear()


def test_competing_chain_with_manipulated_difficulty_does_not_win_replace_chain(blockchain: Blockchain):
    """
    An attacker's chain with fabricated (inflated) declared difficulty
    values to win the accumulated-work race must be rejected once exact
    validation catches the mismatch, even if the fabricated work total
    looked superior.
    """
    from blockchain.block import Block

    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)

    attacker = Wallet.create()
    # Construct a block claiming an inflated difficulty far beyond what a
    # single easily-mined hash could satisfy, and beyond what the
    # protocol's own retarget would ever produce in one step.
    fake_block = Block(
        index=1, previous_hash=blockchain.chain[0].hash,
        transactions=[__import__("blockchain.transaction", fromlist=["Transaction"]).Transaction.new_coinbase(attacker.address, blockchain.settings.mining.block_reward)],
        difficulty=20,  # wildly inflated; hash will not actually satisfy this
    )
    candidate = [blockchain.chain[0], fake_block]

    accepted, reason = blockchain.replace_chain(candidate)
    assert not accepted


def test_deterministic_expected_difficulty_across_two_independent_evaluations(blockchain: Blockchain):
    """Two independent computations from identical chain state must agree exactly."""
    miner = Wallet.create()
    for _ in range(3):
        blockchain.mine_pending_transactions(miner.address)

    result_a = expected_difficulty(blockchain.chain, blockchain.consensus, blockchain.settings.consensus)
    result_b = expected_difficulty(list(blockchain.chain), blockchain.consensus, blockchain.settings.consensus)
    assert result_a == result_b


def test_first_block_after_genesis_uses_config_difficulty(blockchain: Blockchain):
    """Correct first-window behavior: with fewer than 2 blocks of history, the configured default applies."""
    required = expected_difficulty([blockchain.chain[0]], blockchain.consensus, blockchain.settings.consensus)
    assert required == blockchain.settings.consensus.difficulty


def test_multi_window_progression_enforced_correctly(blockchain: Blockchain):
    """Mining several blocks across multiple retarget windows must all validate under exact enforcement."""
    miner = Wallet.create()
    for _ in range(8):
        blockchain.mine_pending_transactions(miner.address)
    is_valid, reason = blockchain.is_chain_valid()
    assert is_valid, reason


def test_restart_preserves_difficulty_enforcement(blockchain: Blockchain):
    """A reloaded chain must validate identically under the same exact-match rule."""
    miner = Wallet.create()
    for _ in range(3):
        blockchain.mine_pending_transactions(miner.address)

    reloaded = Blockchain(settings=blockchain.settings)
    is_valid, reason = reloaded.is_chain_valid()
    assert is_valid, reason
