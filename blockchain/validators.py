"""
Validation logic for SAYANJALI BLOCKCHAIN.

Centralizes the rules that determine whether a block, a chain, or a
transaction is acceptable. Keeping validation separate from Blockchain
itself makes it straightforward to unit test rules in isolation and to
reuse them from the API layer (e.g. validating a submitted transaction
before it ever touches the mempool).
"""

from __future__ import annotations

from blockchain.block import Block
from blockchain.consensus import ConsensusEngine
from blockchain.transaction import Transaction
from blockchain.utils import get_logger

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


def validate_block_structure(block: Block) -> tuple[bool, str]:
    """Validate a block's internal structure, independent of chain context."""
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

    coinbase_count = sum(1 for tx in block.transactions if tx.is_coinbase())
    if coinbase_count > 1:
        return False, "Block contains more than one coinbase transaction."

    for tx in block.transactions:
        is_valid, reason = validate_transaction(tx)
        if not is_valid:
            return False, f"Invalid transaction {tx.tx_hash}: {reason}"

    return True, ""


def validate_block_against_chain(
    block: Block,
    previous_block: Block,
    minimum_difficulty: int,
    consensus: ConsensusEngine,
) -> tuple[bool, str]:
    """
    Validate that `block` correctly extends `previous_block`.

    `minimum_difficulty` is a floor, not an exact expected value: because
    difficulty legitimately retargets over time based on block timing
    (see ConsensusEngine.next_difficulty), a historical block mined during
    an easier period can have a lower difficulty than the network's
    *current* configured difficulty without being invalid. What matters is
    that the block's own recorded difficulty (authenticated by the header
    hash) is at least this floor, and that its hash actually satisfies
    that recorded difficulty.
    """
    if block.index != previous_block.index + 1:
        return False, "Block index is not sequential."
    if block.previous_hash != previous_block.hash:
        return False, "previous_hash does not match previous block's hash."
    if block.timestamp <= previous_block.timestamp:
        return False, "Block timestamp must be after the previous block."
    if not consensus.validate(block, minimum_difficulty):
        return False, "Block does not satisfy consensus (proof-of-work) rules."

    structure_valid, reason = validate_block_structure(block)
    if not structure_valid:
        return False, reason

    return True, ""


def validate_chain(
    chain: list[Block], minimum_difficulty: int, consensus: ConsensusEngine
) -> tuple[bool, str]:
    """
    Validate an entire chain from genesis onward.

    Args:
        minimum_difficulty: The lowest difficulty any block in the chain
            is allowed to have recorded. Pass 1 (or the network's lowest
            historical difficulty) rather than the current live difficulty
            when validating history that may span multiple retargets.

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
            chain[i], chain[i - 1], minimum_difficulty, consensus
        )
        if not is_valid:
            return False, f"Block {chain[i].index} invalid: {reason}"

    return True, ""
