"""
Validation logic for SAYANJALI BLOCKCHAIN.

Centralizes the rules that determine whether a block, a chain, or a
transaction is acceptable. Keeping validation separate from Blockchain
itself makes it straightforward to unit test rules in isolation and to
reuse them from the API layer (e.g. validating a submitted transaction
before it ever touches the mempool).
"""

from __future__ import annotations

from typing import Optional

from blockchain.block import Block
from blockchain.consensus import ConsensusEngine
from blockchain.transaction import Transaction
from blockchain.utils import get_logger
from config.settings import ConsensusConfig

logger = get_logger("blockchain.validators")


def validate_transaction(transaction: Transaction) -> tuple[bool, str]:
    """
    Validate a single transaction's structure and signature.

    Returns:
        (is_valid, reason) -- reason is an empty string when valid.
    """
    if transaction.amount <= 0:
        return False, "Transaction amount must be positive."
    if not transaction.is_coinbase() and transaction.sender == transaction.receiver:
        return False, "Sender and receiver must differ."
    if not transaction.verify():
        return False, "Transaction signature verification failed."
    return True, ""


def chain_work(chain: list[Block]) -> int:
    """
    Compute the accumulated proof-of-work of a chain.

    This blockchain's difficulty target is defined in
    `ProofOfWorkConsensus.mine`/`validate` as a count of required leading
    *hexadecimal* zero characters (`target_prefix = "0" * difficulty`, an
    ASCII hex-digest prefix check). A SHA-256 hex digest has 16 equally
    likely values per character, so each additional difficulty level
    narrows the valid-hash space by a factor of 16, not 2: the probability
    of a given nonce producing a valid hash is ~1/16**difficulty, and the
    expected work to find one scales the same way. Each block therefore
    contributes 16**difficulty "work units" to the chain's accumulated
    work -- not 2**difficulty, which would understate the true cost by a
    large and rapidly widening margin as difficulty increases (e.g. at
    difficulty 4 alone, 2**4=16 versus the correct 16**4=65536).

    This is the correct criterion for comparing two competing PoW chains:
    a shorter chain mined at higher difficulty can represent more real
    work than a longer chain mined at lower difficulty, so length alone is
    not a reliable signal once difficulty can vary.
    """
    return sum(16 ** max(block.difficulty, 0) for block in chain)


def expected_difficulty(
    chain_prefix: list[Block],
    consensus: ConsensusEngine,
    consensus_config: ConsensusConfig,
) -> int:
    """
    Independently derive the protocol-required difficulty for the block
    that would extend `chain_prefix`, using the exact same windowing
    logic `Blockchain.current_difficulty()` uses for mining.

    This is the enforcement half of retargeting: `ConsensusEngine
    .next_difficulty()` is a pure calculation with no opinion about
    trust, and by itself does not protect anything -- a validator that
    never calls it, or that only checks a block's difficulty against a
    trivial floor, accepts blocks at whatever difficulty they claim
    regardless of whether that claim matches the chain's actual retarget
    history. This function is what closes that gap: it recomputes the
    expected value purely from `chain_prefix` (never from a peer-supplied
    or block-embedded value), so a caller can require an exact match
    rather than trusting the block's self-declared difficulty.

    `chain_prefix` is the chain up to and including the block that would
    immediately precede the one being evaluated -- i.e. exactly what
    `Blockchain.current_difficulty()` would see as `self.chain` at the
    moment that next block was mined.
    """
    window = consensus_config.difficulty_adjustment_interval
    recent = chain_prefix[-window:] if len(chain_prefix) >= window else chain_prefix
    return consensus.next_difficulty(
        recent,
        consensus_config.difficulty,
        consensus_config.target_block_time_seconds,
        consensus_config.min_difficulty,
        consensus_config.max_difficulty,
        consensus_config.max_difficulty_adjustment_factor,
    )


def validate_genesis_identity(
    candidate_chain: list[Block], local_genesis: Block
) -> tuple[bool, str]:
    """
    Confirm a candidate chain shares this network's genesis block.

    A chain that is structurally valid on its own terms could still
    belong to an entirely different network (a different genesis
    timestamp, message, or difficulty produces a different genesis hash).
    Without this check, a peer -- malicious or simply misconfigured --
    could hand a node a foreign chain that would otherwise pass
    `validate_chain` outright. This must be checked before any work or
    length comparison is meaningful.
    """
    if not candidate_chain:
        return False, "Candidate chain is empty."
    if candidate_chain[0].hash != local_genesis.hash:
        return False, "Candidate chain's genesis block does not match this network."
    return True, ""


def validate_block_structure(
    block: Block, expected_coinbase_reward: Optional[float] = None
) -> tuple[bool, str]:
    """
    Validate a block's internal structure, independent of chain context.

    Args:
        expected_coinbase_reward: The exact amount this block's coinbase
            transaction must carry. When None (genesis, or a caller that
            deliberately doesn't yet have this context), coinbase amount
            enforcement is skipped -- callers validating a real,
            non-genesis block must supply this to close the issuance gap
            described in `blockchain.mining.Miner`'s docstring: without
            it, nothing stops a block from claiming an arbitrary reward.
    """
    if block.index < 0:
        return False, "Block index cannot be negative."
    if block.compute_merkle_root() != block.merkle_root:
        return False, "Merkle root does not match block transactions."
    if block.compute_hash() != block.hash:
        return False, "Block hash does not match recomputed hash."

    # The genesis block (index 0) carries a single fixed, zero-amount
    # informational entry rather than a real user or coinbase transaction
    # (see Block.genesis), so it is intentionally exempt from the normal
    # transaction rules -- it is not spendable and never affects balances.
    if block.index == 0:
        return True, ""

    coinbase_transactions = [tx for tx in block.transactions if tx.is_coinbase()]
    if len(coinbase_transactions) > 1:
        return False, "Block contains more than one coinbase transaction."
    if len(coinbase_transactions) == 0:
        return False, "Block is missing its required coinbase transaction."

    if expected_coinbase_reward is not None:
        coinbase_amount = coinbase_transactions[0].amount
        if coinbase_amount != expected_coinbase_reward:
            return False, (
                f"Coinbase reward {coinbase_amount} does not match the "
                f"protocol-required reward {expected_coinbase_reward}."
            )

    for tx in block.transactions:
        is_valid, reason = validate_transaction(tx)
        if not is_valid:
            return False, f"Invalid transaction {tx.tx_hash}: {reason}"

    return True, ""


def validate_block_against_chain(
    block: Block,
    chain_so_far: list[Block],
    consensus: ConsensusEngine,
    consensus_config: ConsensusConfig,
    block_reward: float,
) -> tuple[bool, str]:
    """
    Validate that `block` correctly extends the tip of `chain_so_far`.

    `chain_so_far` must be non-empty and end with the block `block` is
    claiming to extend (i.e. exactly the chain state as it existed the
    moment `block` was produced). This is what lets this function
    independently derive the protocol-required difficulty and coinbase
    reward for `block`'s position -- rather than trusting `block`'s own
    claims, or checking them only against a trivial floor -- closing the
    enforcement gap described in `expected_difficulty`'s docstring.
    """
    if not chain_so_far:
        return False, "chain_so_far must not be empty."
    previous_block = chain_so_far[-1]

    if block.index != previous_block.index + 1:
        return False, "Block index is not sequential."
    if block.previous_hash != previous_block.hash:
        return False, "previous_hash does not match previous block's hash."
    if block.timestamp <= previous_block.timestamp:
        return False, "Block timestamp must be after the previous block."

    required_difficulty = expected_difficulty(chain_so_far, consensus, consensus_config)
    if block.difficulty != required_difficulty:
        return False, (
            f"Block difficulty {block.difficulty} does not match the "
            f"protocol-required difficulty {required_difficulty} for this "
            "chain position."
        )
    if not consensus.validate(block, required_difficulty):
        return False, "Block does not satisfy consensus (proof-of-work) rules."

    structure_valid, reason = validate_block_structure(block, block_reward)
    if not structure_valid:
        return False, reason

    return True, ""


def validate_chain(
    chain: list[Block], consensus: ConsensusEngine, consensus_config: ConsensusConfig, block_reward: float
) -> tuple[bool, str]:
    """
    Validate an entire chain from genesis onward.

    Every block's difficulty and coinbase reward are independently
    re-derived from the preceding chain history and the protocol's own
    configuration -- never trusted from the block itself or from a peer
    -- and required to match exactly.

    Returns:
        (is_valid, reason) -- reason describes the first failure found.
    """
    if not chain:
        return False, "Chain is empty."

    genesis_valid, reason = validate_block_structure(chain[0])
    if not genesis_valid:
        return False, f"Genesis block invalid: {reason}"

    for i in range(1, len(chain)):
        is_valid, reason = validate_block_against_chain(
            chain[i], chain[:i], consensus, consensus_config, block_reward
        )
        if not is_valid:
            return False, f"Block {chain[i].index} invalid: {reason}"

    return True, ""
