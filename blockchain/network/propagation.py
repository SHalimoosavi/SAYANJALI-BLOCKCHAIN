"""
Block and transaction propagation for SAYANJALI BLOCKCHAIN's P2P layer.

Handles both directions:
    - Broadcasting something this node produced (mined a block, accepted
      a transaction into its mempool) out to known peers, authenticated.
    - Receiving something a peer broadcast, verifying the peer's
      authentication, validating the payload through the existing Phase 1
      rules, and deciding whether to accept and further relay it.

Phase 6.5: block and transaction propagation now require the sender to
present a valid, freshly-signed auth envelope (see
`blockchain.network.handshake`). An unauthenticated or invalidly
authenticated request is rejected before its payload is ever parsed into
a `Block`/`Transaction` object -- authentication is checked first, ahead
of any other processing, per the required pipeline ordering.

Loop prevention remains the bounded seen-hash cache described in Phase 2:
each node remembers a bounded set of recently seen block/transaction
hashes and never reprocesses or rebroadcasts something it has already
seen, and never rebroadcasts back to whichever peer it just received
something from.

Phase 6.5 concurrency fix: the chain-mutating section of `receive_block`
(append + persist + balance update + mempool cleanup) is now protected by
`Blockchain.mutation_lock`, closing the race window where two concurrent
threads (FastAPI runs synchronous routes in a threadpool) could both pass
the duplicate/extends-tip checks before either had actually appended,
risking a duplicate append or an unhandled storage IntegrityError.
"""

from __future__ import annotations

from typing import Optional

from blockchain.block import Block
from blockchain.network.handshake import build_auth_envelope, verify_auth_envelope
from blockchain.network.node import NetworkNode
from blockchain.transaction import Transaction
from blockchain.utils import get_logger
from blockchain.validators import validate_block_against_chain

logger = get_logger("blockchain.network.propagation")


# --------------------------------------------------------------------- #
# Authentication helper shared by the receive_* functions
# --------------------------------------------------------------------- #


def authenticate_request(
    node: NetworkNode,
    auth_envelope: Optional[dict],
    operational_payload: Optional[dict],
    claimed_address: Optional[str],
) -> tuple[bool, str]:
    """
    Verify an inbound request's auth envelope and, on success, record or
    confirm the sender's trusted credential.

    This is the single authentication gate every state-changing
    `/network/*` endpoint routes through -- see api/network_routes.py.
    Identity-change detection (Phase 6.5 item 11) happens here: if the
    claimed address already has a stored credential under a *different*
    node_id/public_key, the mismatch is logged and the address's trust is
    not silently extended to the new identity even though the signature
    itself may be independently valid (a legitimate key rotation would
    need to go through `/network/peers/authenticate` again explicitly,
    the same as a first-time peer).
    """
    if auth_envelope is None:
        return False, "Authentication required: no auth envelope provided."

    known_credential = None
    if claimed_address:
        known_credential = node.blockchain.storage.get_peer_credential(claimed_address)

    is_valid, reason = verify_auth_envelope(
        auth_envelope,
        operational_payload,
        node.auth_context,
        node.replay_cache,
        node.settings.p2p.auth_freshness_window_seconds,
        known_credential,
    )
    if not is_valid:
        return False, reason

    if claimed_address:
        if known_credential is not None and (
            known_credential["node_id"] != auth_envelope["node_id"]
            or known_credential["public_key_hex"] != auth_envelope["public_key"]
        ):
            return False, (
                "Identity mismatch: this address previously authenticated under a "
                "different identity. Re-run /network/peers/authenticate explicitly."
            )
        if known_credential is None:
            return False, (
                "Address is not a trusted peer yet. "
                "Complete /network/peers/authenticate first."
            )

    return True, ""


# --------------------------------------------------------------------- #
# Outbound: broadcasting things this node produced
# --------------------------------------------------------------------- #


def broadcast_block(
    node: NetworkNode, block: Block, exclude_address: str | None = None
) -> list[str]:
    """
    Send a block, authenticated, to every known *trusted* peer except
    `exclude_address` (the peer it was just received from, if any --
    prevents an immediate bounce back to the sender).

    Untrusted (discovery-only) peers are skipped: propagating chain data
    to a peer that has never completed authentication would mean a
    passive, unauthenticated listener could harvest blocks/transactions
    simply by registering an address, without ever proving control of
    any identity.

    Returns the list of peer addresses that acknowledged receipt.
    """
    node.seen_blocks.add(block.hash)
    block_dict = block.to_dict()
    acknowledged = []

    for address in node.peers.addresses():
        if address == exclude_address:
            continue
        if not node.is_trusted_peer(address):
            continue
        auth_envelope = build_auth_envelope(node.auth_context, block_dict)
        result = node.client.send_block(address, block_dict, node.self_address, auth_envelope)
        if result is not None:
            acknowledged.append(address)
        else:
            logger.info("Block %s propagation to %s failed.", block.hash, address)

    return acknowledged


def broadcast_transaction(
    node: NetworkNode, transaction: Transaction, exclude_address: str | None = None
) -> list[str]:
    """
    Send a transaction, authenticated, to every known trusted peer except
    `exclude_address`. Returns the list of peer addresses that
    acknowledged receipt.
    """
    node.seen_transactions.add(transaction.tx_hash)
    tx_dict = transaction.to_dict()
    acknowledged = []

    for address in node.peers.addresses():
        if address == exclude_address:
            continue
        if not node.is_trusted_peer(address):
            continue
        auth_envelope = build_auth_envelope(node.auth_context, tx_dict)
        result = node.client.send_transaction(
            address, tx_dict, node.self_address, auth_envelope
        )
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
    node: NetworkNode,
    block_dict: dict,
    from_peer: str | None,
    auth_envelope: Optional[dict] = None,
) -> tuple[bool, str, bool]:
    """
    Process a block received from a peer.

    Authentication is checked first, before the block payload is parsed
    or any other work is done -- an unauthenticated or invalidly
    authenticated submission never reaches block parsing/validation at
    all, closing off that path as a resource-exhaustion vector for
    anonymous callers.

    Returns:
        (accepted, reason, should_rebroadcast) -- `should_rebroadcast` is
        True only when the block was newly accepted and other peers have
        not yet been shown to have it, i.e. exactly when relaying it
        onward is useful.
    """
    is_authenticated, auth_reason = authenticate_request(
        node, auth_envelope, block_dict, from_peer
    )
    if not is_authenticated:
        return False, auth_reason, False

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

    with node.blockchain.mutation_lock:
        # Re-check under the lock: another thread may have appended a
        # block at this exact index between our check above and now.
        if block.index != node.blockchain.latest_block.index + 1:
            return False, (
                "Block does not extend the local chain tip; a full sync is required."
            ), False

        is_valid, reason = validate_block_against_chain(
            block, node.blockchain.chain, node.blockchain.consensus,
            node.blockchain.settings.consensus, node.blockchain.settings.mining.block_reward,
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


def receive_transaction(
    node: NetworkNode,
    tx_dict: dict,
    from_peer: str | None,
    auth_envelope: Optional[dict] = None,
) -> tuple[bool, str, bool]:
    """
    Process a transaction received from a peer.

    Authentication is checked first, before the transaction payload is
    parsed, mirroring `receive_block`'s ordering.

    Returns:
        (accepted, reason, should_rebroadcast).
    """
    is_authenticated, auth_reason = authenticate_request(
        node, auth_envelope, tx_dict, from_peer
    )
    if not is_authenticated:
        return False, auth_reason, False

    try:
        transaction = Transaction.from_dict(tx_dict)
    except (KeyError, TypeError, ValueError) as exc:
        return False, f"Malformed transaction payload: {exc}", False

    if transaction.tx_hash in node.seen_transactions:
        return False, "Duplicate transaction (already seen).", False

    with node.blockchain.mutation_lock:
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
