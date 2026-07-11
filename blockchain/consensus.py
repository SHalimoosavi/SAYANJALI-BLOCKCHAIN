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
        self, previous_blocks: list[Block], config_difficulty: int, target_block_time: int
    ) -> int:
        """Compute the difficulty target for the next block."""
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
    ) -> int:
        """
        Simple difficulty retarget based on average time between the last
        blocks compared to `target_block_time`.

        This is intentionally conservative for the MVP: it nudges
        difficulty up or down by at most 1 per adjustment window rather
        than performing a full Bitcoin-style ratio retarget, to keep
        behavior predictable while the network is small.
        """
        if len(previous_blocks) < 2:
            return config_difficulty

        elapsed = previous_blocks[-1].timestamp - previous_blocks[0].timestamp
        num_intervals = len(previous_blocks) - 1
        if num_intervals <= 0 or elapsed <= 0:
            return config_difficulty

        average_time = elapsed / num_intervals
        current_difficulty = previous_blocks[-1].hash.count("0")  # rough heuristic

        if average_time < target_block_time * 0.5:
            return config_difficulty + 1
        if average_time > target_block_time * 2:
            return max(1, config_difficulty - 1)
        return config_difficulty


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
