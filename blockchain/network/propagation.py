"""
Block and transaction propagation for SAYANJALI BLOCKCHAIN's P2P layer.

Handles both directions:
    - Broadcasting something this node produced (mined a block, accepted
      a transaction into its mempool) out to known peers.
    - Receiving something a peer broadcast, validating it through the
      existing Phase 1 rules, and deciding whether to accept and
      further relay it.

Loop prevention is deliberately simple for this MVP: each node remembers
a bounded set of recently seen block/transaction hashes and never
reprocesses or rebroadcasts something it has already seen, and never
rebroadcasts back to whichever peer it just received something from.
This is sufficient to stop the obvious infinite-rebroadcast loop between
two or a few directly connected nodes; it is not a substitute for a real
gossip protocol's anti-entropy guarantees at larger scale (see the
Security section of the README).
"""

from __future__ import annotations

from blockchain.block import Block
from blockchain.network.node import NetworkNode
from blockchain.transaction import Transaction
from blockchain.utils import get_logger
from blockchain.validators import validate_block_against_chain

logger = get_logger("blockchain.network.propagation")


# --------------------------------------------------------------------- #
# Outbound: broadcasting things this node produced
# --------------------------------------------------------------------- #


def broadcast_block(
    node: NetworkNode, block: Block, exclude_address: str | None = None
) -> list[str]:
    """
    Send a block to every known peer except `exclude_address` (the peer
    it was just received from, if any -- prevents an immediate bounce
    back to the sender).

    Returns the list of peer addresses that acknowledged receipt.
    """
    node.seen_blocks.add(block.hash)
    block_dict = block.to_dict()
    acknowledged = []

    for address in node.peers.addresses():
        if address == exclude_address:
            continue
        result = node.client.send_block(address, block_dict, node.self_address)
        if result is not None:
            acknowledged.append(address)
        else:
            logger.info("Block %s propagation to %s failed.", block.hash, address)

    return acknowledged


def broadcast_transaction(
    node: NetworkNode, transaction: Transaction, exclude_address: str | None = None
) -> list[str]:
    """
    Send a transaction to every known peer except `exclude_address`.

    Returns the list of peer addresses that acknowledged receipt.
    """
    node.seen_transactions.add(transaction.tx_hash)
    tx_dict = transaction.to_dict()
    acknowledged = []

    for address in node.peers.addresses():
        if address == exclude_address:
            continue
        result = node.client.send_transaction(address, tx_dict, node.self_address)
        if result is not None:
            acknowledged.append(address)
        else:
            logger.info(
                "Transaction %s propagation to %s failed.", transaction.tx_hash, address
            )

    return acknowledged


# --------------------------------------------------------------------- #
# Inbound: receiving things a peer broadcast
# --------------------------------------------------------------------- #


def receive_block(
    node: NetworkNode, block_dict: dict, from_peer: str | None
) -> tuple[bool, str, bool]:
    """
    Process a block received from a peer.

    Returns:
        (accepted, reason, should_rebroadcast) -- `should_rebroadcast` is
        True only when the block was newly accepted and other peers have
        not yet been shown to have it, i.e. exactly when relaying it
        onward is useful.
    """
    try:
        block = Block.from_dict(block_dict)
    except (KeyError, TypeError, ValueError) as exc:
        return False, f"Malformed block payload: {exc}", False

    if block.hash in node.seen_blocks:
        return False, "Duplicate block (already seen).", False

    existing = node.blockchain.get_block(block.index)
    if existing is not None and existing.hash == block.hash:
        node.seen_blocks.add(block.hash)
        return False, "Duplicate block (already on chain).", False

    node.seen_blocks.add(block.hash)

    if block.index == node.blockchain.latest_block.index + 1:
        # The common case: the block extends our current tip directly.
        is_valid, reason = validate_block_against_chain(
            block, node.blockchain.latest_block, 1, node.blockchain.consensus
        )
        if not is_valid:
            return False, f"Invalid block: {reason}", False

        node.blockchain.chain.append(block)
        node.blockchain.storage.save_block(block)
        node.blockchain.storage.update_balances_for_block(block)
        node.blockchain.mempool.remove_transactions(
            [tx.tx_hash for tx in block.transactions]
        )
        logger.info(
            "Accepted block %s from peer %s (extends tip)", block.hash, from_peer
        )
        return True, "", True

    # The block does not directly extend our tip -- it may belong to a
    # fork that is ahead of us, or we may simply be behind. Attempting a
    # full chain replacement here would require the peer's entire chain,
    # which this endpoint does not carry; instead we signal that a sync
    # is warranted and let the caller (the API route) trigger one against
    # `from_peer` if provided. We do not accept the lone block itself in
    # this case, since accepting an unlinked block would violate chain
    # integrity.
    return False, (
        "Block does not extend the local chain tip; a full sync is required."
    ), False


def receive_transaction(
    node: NetworkNode, tx_dict: dict, from_peer: str | None
) -> tuple[bool, str, bool]:
    """
    Process a transaction received from a peer.

    Returns:
        (accepted, reason, should_rebroadcast).
    """
    try:
        transaction = Transaction.from_dict(tx_dict)
    except (KeyError, TypeError, ValueError) as exc:
        return False, f"Malformed transaction payload: {exc}", False

    if transaction.tx_hash in node.seen_transactions:
        return False, "Duplicate transaction (already seen).", False

    if node.blockchain.mempool.contains(transaction.tx_hash):
        node.seen_transactions.add(transaction.tx_hash)
        return False, "Duplicate transaction (already pending).", False

    node.seen_transactions.add(transaction.tx_hash)

    accepted, reason = node.blockchain.submit_transaction(transaction)
    if not accepted:
        return False, reason, False

    logger.info(
        "Accepted transaction %s from peer %s", transaction.tx_hash, from_peer
    )
    return True, "", True
