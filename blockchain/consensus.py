"""
Consensus engine for SAYANJALI BLOCKCHAIN.

The MVP implements Proof of Work (PoW) via the `ProofOfWorkConsensus`
class, which conforms to the `ConsensusEngine` abstract interface. Future
algorithms (Proof of Stake, Delegated Proof of Stake) should be added as
new classes implementing the same interface and selected in
config/settings.py via `ConsensusConfig.algorithm`, so that
blockchain/blockchain.py never needs to change when consensus changes.
"""

from __future__ import annotations

from abc import ABC, abstractmethod
from fractions import Fraction

from blockchain.block import Block
from blockchain.utils import ValidationError, get_logger

logger = get_logger("blockchain.consensus")


class ConsensusEngine(ABC):
    """Abstract interface every consensus algorithm must implement."""

    @abstractmethod
    def mine(self, block: Block, difficulty: int) -> Block:
        """Find a valid nonce/proof for `block` and return the finalized block."""
        raise NotImplementedError

    @abstractmethod
    def validate(self, block: Block, difficulty: int) -> bool:
        """Return True if `block` satisfies this consensus algorithm's rules."""
        raise NotImplementedError

    @abstractmethod
    def next_difficulty(
        self,
        previous_blocks: list[Block],
        config_difficulty: int,
        target_block_time: int,
        min_difficulty: int = 1,
        max_difficulty: int = 32,
        max_adjustment_factor: int = 4,
    ) -> int:
        """
        Compute the difficulty target for the next block.

        `min_difficulty`/`max_difficulty`/`max_adjustment_factor` default
        to sane values so an engine that doesn't care about tunable bounds
        can still implement this method without extra boilerplate, but
        `ProofOfWorkConsensus` always receives explicit values from
        `Blockchain.current_difficulty`, which reads them from
        `config.settings.ConsensusConfig`.
        """
        raise NotImplementedError


class ProofOfWorkConsensus(ConsensusEngine):
    """
    Classic Proof-of-Work consensus.

    A block is valid when its hash, recomputed from its header fields,
    has at least `difficulty` leading hexadecimal zero characters. Mining
    searches for a nonce producing such a hash.
    """

    def mine(self, block: Block, difficulty: int) -> Block:
        """
        Repeatedly increment `block.nonce` and recompute the hash until
        the difficulty target is met.

        Mutates and returns the same block instance for convenience.
        """
        if difficulty < 0:
            raise ValidationError("Difficulty must be non-negative.")

        target_prefix = "0" * difficulty
        block.nonce = 0
        block.difficulty = difficulty
        block.recompute()

        while not block.hash.startswith(target_prefix):
            block.nonce += 1
            block.recompute()
            if block.nonce > 2**32:
                # Extremely unlikely for MVP difficulty levels, but guards
                # against an infinite loop if difficulty is misconfigured.
                raise ValidationError(
                    "Exceeded maximum nonce search space without finding "
                    "a valid hash. Check difficulty configuration."
                )

        logger.info(
            "Mined block %s with nonce %s (hash=%s)",
            block.index,
            block.nonce,
            block.hash,
        )
        return block

    def validate(self, block: Block, difficulty: int) -> bool:
        """
        Validate that `block` was mined correctly.

        The block's own `difficulty` field (part of its hashed header, so
        it cannot be tampered with independently of the hash) is what its
        proof-of-work is actually checked against. The `difficulty`
        parameter is the difficulty the caller currently expects new
        blocks to be mined at; if it is greater than what the block
        recorded, the block is rejected as too easy for the network's
        current requirements.
        """
        expected_hash = block.compute_hash()
        if block.hash != expected_hash:
            logger.warning(
                "Block %s hash mismatch: stored=%s recomputed=%s",
                block.index,
                block.hash,
                expected_hash,
            )
            return False

        if block.difficulty < difficulty:
            logger.warning(
                "Block %s recorded difficulty %s is below required minimum %s",
                block.index,
                block.difficulty,
                difficulty,
            )
            return False

        return block.meets_difficulty(block.difficulty)

    def next_difficulty(
        self,
        previous_blocks: list[Block],
        config_difficulty: int,
        target_block_time: int,
        min_difficulty: int = 1,
        max_difficulty: int = 32,
        max_adjustment_factor: int = 4,
    ) -> int:
        """
        Bitcoin-style ratio-based difficulty retarget.

        Unlike a flat +-1 nudge, the adjustment magnitude scales with how
        far the actual time to mine the window deviated from the target,
        computed entirely with integers and `fractions.Fraction` -- never
        floating point -- so every node reaches the exact same result
        from the same chain state, with no platform-dependent rounding.

        Algorithm (mirrors Bitcoin's retarget, adapted to this project's
        discrete "leading hex zeros" difficulty representation rather
        than a continuous numeric target):

        1. `expected_timespan` = how long the window *should* have taken
           at the configured target block time.
        2. `actual_timespan` = how long it *actually* took, read directly
           from the window's own block timestamps -- never from the
           local wall clock, so retargeting is fully deterministic and
           reproducible by any node re-validating the same chain, and
           never depends on when a node happens to run the calculation.
        3. `actual_timespan` is clamped to
           [expected_timespan / max_adjustment_factor,
            expected_timespan * max_adjustment_factor] before use, the
           same defense Bitcoin's own retarget uses: it bounds how much
           a single unusually fast, slow, or manipulated interval can
           swing the result in one step, regardless of how extreme the
           raw measured timespan is.
        4. The window's actual mined difficulty (`previous_blocks[-1]
           .difficulty` -- the chain's real current state, not the
           static `config_difficulty` the previous implementation
           incorrectly nudged relative to) implies a "work per block" of
           `16 ** difficulty` (matching `blockchain.validators.chain_work`'s
           accounting exactly). The new work target is that value scaled
           by `expected_timespan / clamped_actual_timespan`.
        5. The new integer difficulty moves toward whichever direction
           the new work target implies, one level at a time, as long as
           the target clears that level's geometric-midpoint threshold
           (see the implementation comment below for why the midpoint,
           not a simple round-down, is used) -- so a deviation has to
           approach the configured adjustment clamp before it actually
           changes difficulty; smaller deviations leave it unchanged.
        6. The result is clamped to `[min_difficulty, max_difficulty]`.

        Returns `config_difficulty` unchanged if the window has fewer
        than two blocks (nothing to measure yet) or the configured
        target/expected timespan is non-positive (a misconfiguration
        this function defends against rather than divides by zero on).
        """
        if len(previous_blocks) < 2:
            return config_difficulty

        interval = len(previous_blocks) - 1
        expected_timespan = interval * target_block_time
        if expected_timespan <= 0 or target_block_time <= 0:
            return config_difficulty

        actual_timespan = previous_blocks[-1].timestamp - previous_blocks[0].timestamp
        # Chain validation already enforces strictly increasing block
        # timestamps, so a valid chain's window should never produce a
        # non-positive span here -- this guard is defense-in-depth against
        # a caller passing an unvalidated or synthetic block list, not an
        # expected path for real chain data.
        if actual_timespan <= 0:
            actual_timespan = 1

        if max_adjustment_factor < 1:
            max_adjustment_factor = 1  # a factor below 1 would invert the clamp
        min_timespan = max(1, expected_timespan // max_adjustment_factor)
        max_timespan = expected_timespan * max_adjustment_factor
        clamped_timespan = max(min_timespan, min(actual_timespan, max_timespan))

        base_difficulty = max(previous_blocks[-1].difficulty, 0)
        current_work = 16 ** base_difficulty

        # Exact rational arithmetic throughout -- no floats anywhere in
        # this calculation, so results are bit-for-bit identical across
        # every platform this project runs on (including Termux/ARM64).
        target_work = Fraction(current_work * expected_timespan, clamped_timespan)

        # Consecutive integer difficulty levels differ by a full 16x in
        # implied work (see blockchain.validators.chain_work), so a naive
        # "round down to the highest level whose threshold doesn't exceed
        # target_work" is asymmetric: it flips down a full level for any
        # slowdown at all, however tiny, while requiring a full 16x
        # speedup to flip up even one level -- unreachable under the
        # default 4x adjustment clamp. Comparing against each level's
        # geometric midpoint (sqrt(16) = 4x, matching the default
        # max_adjustment_factor exactly) instead gives symmetric,
        # sensible behavior: only a deviation approaching the clamp's
        # own limit is enough to shift difficulty by one level; anything
        # smaller leaves difficulty unchanged.
        #
        # The up-threshold from level d (target_work >= 16**d * 4) is
        # numerically identical to the down-threshold from level d+1
        # (target_work <= 16**(d+1) / 4), since both equal 16**d * 4. If
        # an "increase" loop and a "decrease" loop both ran unconditionally
        # in sequence, hitting that shared boundary would move up and then
        # immediately move back down, cancelling out. Deciding the
        # direction once, from the original (pre-mutation) comparison,
        # and only ever running the one matching loop, makes the two
        # directions mutually exclusive so this cannot happen.
        new_difficulty = base_difficulty
        if target_work > Fraction(current_work):
            while (
                new_difficulty < max_difficulty
                and target_work >= Fraction(16 ** new_difficulty) * 4
            ):
                new_difficulty += 1
        elif target_work < Fraction(current_work):
            while (
                new_difficulty > min_difficulty
                and target_work <= Fraction(16 ** new_difficulty, 4)
            ):
                new_difficulty -= 1

        return max(min_difficulty, min(new_difficulty, max_difficulty))


def get_consensus_engine(algorithm: str = "pow") -> ConsensusEngine:
    """
    Factory returning the ConsensusEngine implementation for `algorithm`.

    Raises:
        ValidationError: if the requested algorithm is not yet implemented.
        This will happen for "pos" / "dpos" until those engines are built,
        which is intentional -- the config system already accepts those
        values so future work is additive, not a breaking config change.
    """
    engines: dict[str, type[ConsensusEngine]] = {
        "pow": ProofOfWorkConsensus,
    }
    engine_cls = engines.get(algorithm)
    if engine_cls is None:
        raise ValidationError(
            f"Consensus algorithm '{algorithm}' is not implemented yet. "
            f"Available: {list(engines.keys())}"
        )
    return engine_cls()
