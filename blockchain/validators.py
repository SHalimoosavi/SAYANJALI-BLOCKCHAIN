"""Deterministic transaction, block, chain, and monetary validation rules."""

from __future__ import annotations

from typing import Optional

from blockchain.block import Block
from blockchain.consensus import ConsensusEngine
from blockchain.native_asset import MAX_SUPPLY_BASE_UNITS, validate_base_units
from blockchain.transaction import Transaction
from blockchain.utils import get_logger
from config.settings import ConsensusConfig

logger = get_logger("blockchain.validators")


def validate_transaction(transaction: Transaction) -> tuple[bool, str]:
    """Validate transaction structure, exact integer amount, and signature."""
    amount_ok, amount_reason = validate_base_units(transaction.amount_base_units)
    if not amount_ok:
        return False, amount_reason
    if not transaction.is_coinbase() and transaction.sender == transaction.receiver:
        return False, "Sender and receiver must differ."
    if not transaction.verify():
        return False, "Transaction signature verification failed."
    return True, ""


def chain_work(chain: list[Block]) -> int:
    """Compute accumulated PoW work using hexadecimal leading-zero difficulty."""
    return sum(16 ** max(block.difficulty, 0) for block in chain)


def expected_difficulty(
    chain_prefix: list[Block],
    consensus: ConsensusEngine,
    consensus_config: ConsensusConfig,
) -> int:
    """Derive the protocol-required difficulty for the next block."""
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
    """Confirm a candidate chain belongs to this network's genesis."""
    if not candidate_chain:
        return False, "Candidate chain is empty."
    if candidate_chain[0].hash != local_genesis.hash:
        return False, "Candidate chain's genesis block does not match this network."
    return True, ""


def _state_from_chain(
    chain: list[Block], expected_coinbase_reward: int, max_supply_base_units: int
) -> tuple[dict[str, int], int, str]:
    """Replay monetary state deterministically and enforce supply/balance rules."""
    balances: dict[str, int] = {}
    total_supply = 0
    for block in chain:
        if block.index == 0:
            continue
        coinbases = [tx for tx in block.transactions if tx.is_coinbase()]
        if len(coinbases) != 1:
            return {}, total_supply, "Each non-genesis block must contain exactly one coinbase."
        coinbase = coinbases[0]
        expected_reward = min(expected_coinbase_reward, max_supply_base_units - total_supply)
        if expected_reward <= 0:
            return {}, total_supply, "No SYJ issuance remains; further coinbase issuance is forbidden."
        if coinbase.amount_base_units != expected_reward:
            return {}, total_supply, "Coinbase reward does not match protocol reward or remaining supply."
        next_supply = total_supply + coinbase.amount_base_units
        if next_supply > max_supply_base_units:
            return {}, total_supply, "Maximum SYJ supply would be exceeded."
        total_supply = next_supply
        balances[coinbase.receiver] = balances.get(coinbase.receiver, 0) + coinbase.amount_base_units
        for tx in block.transactions:
            if tx.is_coinbase():
                continue
            valid, reason = validate_transaction(tx)
            if not valid:
                return {}, total_supply, f"Invalid transaction {tx.tx_hash}: {reason}"
            sender_balance = balances.get(tx.sender, 0)
            if tx.amount_base_units > sender_balance:
                return {}, total_supply, (
                    f"Insufficient balance for {tx.sender}: "
                    f"needs {tx.amount_base_units}, has {sender_balance}."
                )
            balances[tx.sender] = sender_balance - tx.amount_base_units
            balances[tx.receiver] = balances.get(tx.receiver, 0) + tx.amount_base_units
    return balances, total_supply, ""


def validate_block_structure(
    block: Block, expected_coinbase_reward: Optional[int] = None,
    max_supply_base_units: int = MAX_SUPPLY_BASE_UNITS,
) -> tuple[bool, str]:
    """Validate block integrity, transaction validity, and coinbase issuance."""
    if block.index < 0:
        return False, "Block index cannot be negative."
    if block.compute_merkle_root() != block.merkle_root:
        return False, "Merkle root does not match block transactions."
    if block.compute_hash() != block.hash:
        return False, "Block hash does not match recomputed hash."
    if block.index == 0:
        return True, ""

    coinbase_transactions = [tx for tx in block.transactions if tx.is_coinbase()]
    if len(coinbase_transactions) > 1:
        return False, "Block contains more than one coinbase transaction."
    if len(coinbase_transactions) == 0:
        return False, "Block is missing its required coinbase transaction."
    coinbase = coinbase_transactions[0]

    if expected_coinbase_reward is not None and coinbase.amount_base_units != expected_coinbase_reward:
        return False, (
            f"Coinbase reward {coinbase.amount_base_units} base units does not match "
            f"the protocol-required reward {expected_coinbase_reward}."
        )
    if coinbase.amount_base_units > max_supply_base_units:
        return False, "Coinbase reward exceeds maximum SYJ supply."

    for tx in block.transactions:
        valid, reason = validate_transaction(tx)
        if not valid:
            return False, f"Invalid transaction {tx.tx_hash}: {reason}"
    return True, ""


def validate_block_against_chain(
    block: Block,
    chain_so_far: list[Block],
    consensus: ConsensusEngine,
    consensus_config: ConsensusConfig,
    block_reward: int,
    max_supply_base_units: int = MAX_SUPPLY_BASE_UNITS,
) -> tuple[bool, str]:
    """Validate a block as the exact next state transition of a chain."""
    if not chain_so_far:
        return False, "chain_so_far must not be empty."
    previous_block = chain_so_far[-1]
    if block.index != previous_block.index + 1:
        return False, "Block index is not sequential."
    if block.previous_hash != previous_block.hash:
        return False, "previous_hash does not match previous block's hash."
    if block.timestamp <= previous_block.timestamp:
        return False, "Block timestamp must be after the previous block."

    _, current_supply, state_reason = _state_from_chain(
        chain_so_far, block_reward, max_supply_base_units
    )
    if state_reason:
        return False, f"Existing chain state invalid: {state_reason}"

    # Validate the candidate's monetary state transition before accepting
    # its proof-of-work. This prevents a correctly-mined block from
    # bypassing balance/supply rules merely by carrying a valid hash.
    _, candidate_supply, candidate_state_reason = _state_from_chain(
        chain_so_far + [block], block_reward, max_supply_base_units
    )
    if candidate_state_reason:
        return False, f"Invalid monetary state transition: {candidate_state_reason}"

    required_difficulty = expected_difficulty(chain_so_far, consensus, consensus_config)
    if block.difficulty != required_difficulty:
        return False, (
            f"Block difficulty {block.difficulty} does not match the protocol-required "
            f"difficulty {required_difficulty} for this chain position."
        )
    if not consensus.validate(block, required_difficulty):
        return False, "Block does not satisfy consensus (proof-of-work) rules."

    expected_reward = min(block_reward, max_supply_base_units - current_supply)
    if expected_reward <= 0:
        return False, "No SYJ issuance remains; a new block cannot contain a coinbase reward."
    structure_valid, reason = validate_block_structure(
        block, expected_reward, max_supply_base_units
    )
    if not structure_valid:
        return False, reason

    coinbase = next(tx for tx in block.transactions if tx.is_coinbase())
    if candidate_supply > max_supply_base_units:
        return False, "Block would exceed the maximum SYJ supply."
    return True, ""


def validate_chain(
    chain: list[Block],
    consensus: ConsensusEngine,
    consensus_config: ConsensusConfig,
    block_reward: int,
    max_supply_base_units: int = MAX_SUPPLY_BASE_UNITS,
) -> tuple[bool, str]:
    """Validate an entire chain, including deterministic monetary state replay."""
    if not chain:
        return False, "Chain is empty."
    genesis_valid, reason = validate_block_structure(chain[0])
    if not genesis_valid:
        return False, f"Genesis block invalid: {reason}"

    for i in range(1, len(chain)):
        valid, reason = validate_block_against_chain(
            chain[i], chain[:i], consensus, consensus_config,
            block_reward, max_supply_base_units,
        )
        if not valid:
            return False, f"Block {chain[i].index} invalid: {reason}"

    _, total_supply, state_reason = _state_from_chain(
        chain, block_reward, max_supply_base_units
    )
    if state_reason:
        return False, f"Monetary state invalid: {state_reason}"
    if total_supply > max_supply_base_units:
        return False, "Maximum SYJ supply exceeded."
    return True, ""
