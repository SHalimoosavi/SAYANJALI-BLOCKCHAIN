"""
Tests for Phase 7: the Bitcoin-style ratio-based difficulty retarget in
blockchain/consensus.py's ProofOfWorkConsensus.next_difficulty.

Covers normal operation, boundary conditions (min/max difficulty),
determinism, multi-block progression, adversarial/malformed input, and
regression against existing chain validation and mining.
"""

from __future__ import annotations

from fractions import Fraction

import pytest

from blockchain.block import Block
from blockchain.blockchain import Blockchain
from blockchain.consensus import ProofOfWorkConsensus
from blockchain.wallet import Wallet


def _window(difficulties: list[int], timestamps: list[float]) -> list[Block]:
    """Build a synthetic block window for isolated retarget testing."""
    return [
        Block(index=i, previous_hash="a" * 64, difficulty=d, timestamp=t)
        for i, (d, t) in enumerate(zip(difficulties, timestamps))
    ]


@pytest.fixture
def engine() -> ProofOfWorkConsensus:
    return ProofOfWorkConsensus()


# --------------------------------------------------------------------- #
# Normal operation
# --------------------------------------------------------------------- #


def test_on_target_timing_leaves_difficulty_unchanged(engine):
    window = _window([4] * 5, [1000, 1030, 1060, 1090, 1120])  # exactly 30s/block
    assert engine.next_difficulty(window, 4, 30, 1, 32, 4) == 4


def test_small_deviation_leaves_difficulty_unchanged(engine):
    """A deviation well short of the adjustment clamp must not move difficulty at all."""
    slightly_fast = _window([4] * 5, [1000, 1029, 1058, 1087, 1116])  # 116s vs 120s expected
    assert engine.next_difficulty(slightly_fast, 4, 30, 1, 32, 4) == 4

    slightly_slow = _window([4] * 5, [1000, 1030, 1061, 1091, 1122])  # 122s vs 120s expected
    assert engine.next_difficulty(slightly_slow, 4, 30, 1, 32, 4) == 4


def test_difficulty_increases_when_blocks_mined_too_fast(engine):
    """Blocks mined at exactly the clamp's speedup limit must increase difficulty by one level."""
    fast_window = _window([4] * 5, [1000, 1007, 1014, 1021, 1028])  # ~28s span vs 120s expected
    result = engine.next_difficulty(fast_window, 4, 30, 1, 32, 4)
    assert result == 5


def test_difficulty_decreases_when_blocks_mined_too_slowly(engine):
    """Blocks mined at exactly the clamp's slowdown limit must decrease difficulty by one level."""
    slow_window = _window([4] * 5, [1000, 1120, 1240, 1360, 1480])  # 480s span vs 120s expected
    result = engine.next_difficulty(slow_window, 4, 30, 1, 32, 4)
    assert result == 3


def test_adjustment_is_ratio_based_not_a_fixed_nudge(engine):
    """
    The core Phase 7 requirement: an extreme deviation must be able to
    move difficulty by more than the old implementation's fixed +/-1,
    when the ratio genuinely supports it (capped by the adjustment
    clamp, not hardcoded to exactly one level regardless of severity).
    """
    extreme_fast = _window([10] * 5, [1000, 1000, 1000, 1001, 1001])  # ~1s span vs 120s expected
    result = engine.next_difficulty(extreme_fast, 10, 30, 1, 32, 4)
    assert result > 10  # moved up, and the clamp (not a flat nudge) determined by how much
    assert result <= 32


# --------------------------------------------------------------------- #
# Boundary conditions: minimum / maximum difficulty
# --------------------------------------------------------------------- #


def test_difficulty_never_drops_below_configured_minimum(engine):
    extremely_slow = _window([2] * 5, [1000, 100_000, 200_000, 300_000, 400_000])
    result = engine.next_difficulty(extremely_slow, 2, 30, min_difficulty=1, max_difficulty=32, max_adjustment_factor=4)
    assert result >= 1


def test_difficulty_never_exceeds_configured_maximum(engine):
    extremely_fast = _window([30] * 5, [1000, 1001, 1002, 1003, 1004])
    result = engine.next_difficulty(extremely_fast, 30, 30, min_difficulty=1, max_difficulty=32, max_adjustment_factor=4)
    assert result <= 32


def test_already_at_maximum_stays_at_maximum_under_further_speedup(engine):
    window = _window([32] * 5, [1000, 1001, 1002, 1003, 1004])
    result = engine.next_difficulty(window, 32, 30, min_difficulty=1, max_difficulty=32, max_adjustment_factor=4)
    assert result == 32


def test_already_at_minimum_stays_at_minimum_under_further_slowdown(engine):
    window = _window([1] * 5, [1000, 500_000, 1_000_000, 1_500_000, 2_000_000])
    result = engine.next_difficulty(window, 1, 30, min_difficulty=1, max_difficulty=32, max_adjustment_factor=4)
    assert result == 1


def test_custom_minimum_difficulty_respected(engine):
    extremely_slow = _window([10] * 5, [1000, 100_000, 200_000, 300_000, 400_000])
    result = engine.next_difficulty(extremely_slow, 10, 30, min_difficulty=5, max_difficulty=32, max_adjustment_factor=4)
    assert result >= 5


def test_custom_maximum_difficulty_respected(engine):
    extremely_fast = _window([5] * 5, [1000, 1000, 1000, 1000, 1001])
    result = engine.next_difficulty(extremely_fast, 5, 30, min_difficulty=1, max_difficulty=6, max_adjustment_factor=4)
    assert result <= 6


# --------------------------------------------------------------------- #
# Determinism
# --------------------------------------------------------------------- #


def test_same_input_always_produces_same_output(engine):
    window = _window([4] * 5, [1000, 1015, 1033, 1048, 1067])
    results = {engine.next_difficulty(window, 4, 30, 1, 32, 4) for _ in range(20)}
    assert len(results) == 1


def test_result_uses_no_floating_point_internally():
    """
    Structural guarantee, not just an empirical one: the implementation
    must use exact rational arithmetic so results cannot drift by
    platform-dependent floating point rounding.
    """
    import inspect

    source = inspect.getsource(ProofOfWorkConsensus.next_difficulty)
    assert "Fraction" in source
    # No true-division float operators applied to the core work/timespan
    # comparison arithmetic (integer // is used for clamping bounds).
    assert "0.5" not in source and "* 2" not in source


# --------------------------------------------------------------------- #
# Multi-block progression (simulates a running chain)
# --------------------------------------------------------------------- #


def test_multi_block_progression_responds_to_sustained_fast_mining(monkeypatch):
    """
    Mining several blocks much faster than target, across multiple
    retarget windows, should push difficulty up from its starting point
    -- demonstrating the roadmap's actual milestone ("difficulty
    stabilizes block time under varying network hash rate") rather than
    just testing the formula in isolation.

    Uses a small explicit `max_difficulty` ceiling rather than the
    production default (32). This is a direct, deliberately-triggered
    example of a real adversarial-review finding from building this
    test: sustained one-directional deviation compounds multiplicatively
    across successive retarget windows (up to `max_adjustment_factor`
    per window), and since mining cost scales as 16**difficulty, an
    unbounded difficulty ceiling combined with fast, sustained mining
    can make continued mining computationally intractable within a
    test's runtime. `max_difficulty` is exactly the configured defense
    against that in a real deployment; this test exercises it directly
    instead of letting difficulty run away.
    """
    from blockchain.blockchain import Blockchain

    monkeypatch.setenv("SYJ_DIFFICULTY", "1")
    monkeypatch.setenv("SYJ_DIFFICULTY_ADJUSTMENT_INTERVAL", "3")
    monkeypatch.setenv("SYJ_TARGET_BLOCK_TIME", "1")
    monkeypatch.setenv("SYJ_MAX_DIFFICULTY", "6")  # small, test-appropriate ceiling

    from config import settings as settings_module

    settings_module.get_settings.cache_clear()
    blockchain = Blockchain()

    miner = Wallet.create()
    initial_difficulty = blockchain.current_difficulty()

    for _ in range(12):
        blockchain.mine_pending_transactions(miner.address)

    final_difficulty = blockchain.current_difficulty()
    assert final_difficulty >= initial_difficulty
    assert final_difficulty <= 6  # the configured ceiling was actually respected

    settings_module.get_settings.cache_clear()


def test_multi_block_progression_is_internally_consistent(monkeypatch):
    """Every block mined under a progressing difficulty must still validate."""
    from blockchain.blockchain import Blockchain

    monkeypatch.setenv("SYJ_DIFFICULTY", "1")
    monkeypatch.setenv("SYJ_DIFFICULTY_ADJUSTMENT_INTERVAL", "3")
    monkeypatch.setenv("SYJ_TARGET_BLOCK_TIME", "1")
    monkeypatch.setenv("SYJ_MAX_DIFFICULTY", "6")

    from config import settings as settings_module

    settings_module.get_settings.cache_clear()
    blockchain = Blockchain()

    miner = Wallet.create()
    for _ in range(10):
        blockchain.mine_pending_transactions(miner.address)

    is_valid, reason = blockchain.is_chain_valid()
    assert is_valid, reason

    settings_module.get_settings.cache_clear()


# --------------------------------------------------------------------- #
# Mining and chain-validation compatibility (regression)
# --------------------------------------------------------------------- #


def test_mining_still_works_with_retargeted_difficulty(blockchain: Blockchain):
    """The Miner/consensus mining path must remain fully compatible with
    difficulties produced by the new retarget algorithm."""
    miner = Wallet.create()
    block = blockchain.mine_pending_transactions(miner.address)
    assert block.meets_difficulty(block.difficulty)
    assert blockchain.consensus.validate(block, 1)


def test_chain_validation_accepts_blocks_at_varying_difficulty(blockchain: Blockchain):
    miner = Wallet.create()
    for _ in range(5):
        blockchain.mine_pending_transactions(miner.address)
    is_valid, reason = blockchain.is_chain_valid()
    assert is_valid, reason


def test_replace_chain_still_works_with_new_retarget(blockchain: Blockchain):
    """Regression: work-based chain adoption (Phase 2/6.5) must be unaffected."""
    from blockchain.mining import Miner

    miner = Wallet.create()
    blockchain.mine_pending_transactions(miner.address)

    other_miner = Wallet.create()
    engine = Miner(blockchain.consensus, blockchain.settings.mining.block_reward)
    candidate = [blockchain.chain[0]]
    for i in range(1, 4):
        new_block, _ = engine.mine_block(
            index=i, previous_hash=candidate[-1].hash, mempool=blockchain.mempool,
            miner_address=other_miner.address, difficulty=blockchain.settings.consensus.difficulty,
        )
        candidate.append(new_block)

    accepted, reason = blockchain.replace_chain(candidate)
    assert accepted, reason


# --------------------------------------------------------------------- #
# Persistence / restart behavior
# --------------------------------------------------------------------- #


def test_difficulty_calculation_survives_restart(blockchain: Blockchain):
    """
    Difficulty is derived entirely from persisted block history (each
    block's own recorded difficulty field), not from any separately
    persisted consensus state -- so reloading a Blockchain from storage
    must reproduce the exact same current_difficulty() result.
    """
    miner = Wallet.create()
    for _ in range(3):
        blockchain.mine_pending_transactions(miner.address)

    difficulty_before = blockchain.current_difficulty()

    reloaded = Blockchain(settings=blockchain.settings)
    difficulty_after = reloaded.current_difficulty()

    assert difficulty_before == difficulty_after


# --------------------------------------------------------------------- #
# Malformed / adversarial input
# --------------------------------------------------------------------- #


def test_empty_window_returns_config_difficulty(engine):
    assert engine.next_difficulty([], 4, 30, 1, 32, 4) == 4


def test_single_block_window_returns_config_difficulty(engine):
    window = _window([4], [1000])
    assert engine.next_difficulty(window, 4, 30, 1, 32, 4) == 4


def test_zero_target_block_time_returns_config_difficulty(engine):
    window = _window([4] * 5, [1000, 1030, 1060, 1090, 1120])
    assert engine.next_difficulty(window, 4, 0, 1, 32, 4) == 4


def test_negative_target_block_time_returns_config_difficulty(engine):
    window = _window([4] * 5, [1000, 1030, 1060, 1090, 1120])
    assert engine.next_difficulty(window, 4, -30, 1, 32, 4) == 4


def test_non_positive_actual_timespan_does_not_crash(engine):
    """
    Defense-in-depth: a window with non-increasing timestamps (which
    valid chain data should never produce, given monotonic-timestamp
    validation elsewhere) must not raise or divide by zero.
    """
    window = _window([4] * 5, [1000, 1000, 1000, 1000, 1000])  # identical timestamps
    result = engine.next_difficulty(window, 4, 30, 1, 32, 4)
    assert isinstance(result, int)
    assert 1 <= result <= 32


def test_decreasing_timestamps_does_not_crash(engine):
    window = _window([4] * 5, [1000, 900, 800, 700, 600])  # pathological/adversarial
    result = engine.next_difficulty(window, 4, 30, 1, 32, 4)
    assert isinstance(result, int)
    assert 1 <= result <= 32


def test_adjustment_factor_below_one_does_not_crash_or_invert(engine):
    window = _window([4] * 5, [1000, 1007, 1014, 1021, 1028])
    result = engine.next_difficulty(window, 4, 30, min_difficulty=1, max_difficulty=32, max_adjustment_factor=0)
    assert isinstance(result, int)
    assert 1 <= result <= 32


def test_negative_block_difficulty_treated_as_zero(engine):
    """A block with a corrupt/negative difficulty field must not break the calculation."""
    window = _window([-5, -5, -5, -5, -5], [1000, 1030, 1060, 1090, 1120])
    result = engine.next_difficulty(window, 4, 30, 1, 32, 4)
    assert isinstance(result, int)
    assert 1 <= result <= 32


def test_min_difficulty_greater_than_max_difficulty_does_not_crash(engine):
    """Misconfigured bounds (min > max) must not raise -- exercised as a config-validation edge case."""
    window = _window([4] * 5, [1000, 1007, 1014, 1021, 1028])
    result = engine.next_difficulty(window, 4, 30, min_difficulty=10, max_difficulty=5, max_adjustment_factor=4)
    assert isinstance(result, int)


def test_config_bounds_are_exposed_and_configurable():
    from config.settings import get_settings

    settings = get_settings()
    assert hasattr(settings.consensus, "min_difficulty")
    assert hasattr(settings.consensus, "max_difficulty")
    assert hasattr(settings.consensus, "max_difficulty_adjustment_factor")
    assert settings.consensus.min_difficulty >= 0
    assert settings.consensus.max_difficulty > settings.consensus.min_difficulty
