"""
Chain synchronization for SAYANJALI BLOCKCHAIN's P2P layer.

Implements the deterministic sync procedure: fetch a peer's chain,
validate it completely, compare accumulated proof-of-work against the
local chain, and adopt it only if it is both valid and superior. All
validation and adoption logic is delegated to
`blockchain.validators` and `Blockchain.replace_chain` -- this module
only orchestrates the network round-trip and the decision of whether to
call them.
"""

from __future__ import annotations

from dataclasses import dataclass

from blockchain.block import Block
from blockchain.network.node import NetworkNode
from blockchain.utils import get_logger

logger = get_logger("blockchain.network.sync")


@dataclass
class SyncResult:
    """Outcome of a single sync attempt against one peer."""

    peer_address: str
    accepted: bool
    reason: str
    local_length_before: int
    local_length_after: int


def _parse_candidate_chain(raw_blocks: list) -> list[Block] | None:
    """
    Parse a peer-supplied list of block dicts into `Block` objects.

    Returns None (rather than raising) if any block is malformed, so
    callers can treat a parse failure the same as any other invalid-chain
    rejection rather than special-casing exceptions.
    """
    try:
        return [Block.from_dict(b) for b in raw_blocks]
    except (KeyError, TypeError, ValueError) as exc:
        logger.warning("Rejected malformed candidate chain: %s", exc)
        return None


def sync_with_peer(node: NetworkNode, peer_address: str) -> SyncResult:
    """
    Attempt to synchronize this node's chain from a single peer.

    Follows the required sequence exactly: request the peer's chain,
    validate it completely, reject it outright if invalid or inferior,
    and only ever adopt a chain that is both valid and represents more
    accumulated work than the local chain. A locally valid chain is never
    replaced with an inferior or invalid one, regardless of what a peer
    sends.
    """
    local_length_before = node.blockchain.length
    payload = node.sync_client.get_chain(peer_address)

    if payload is None:
        return SyncResult(
            peer_address, False, "Peer unreachable or returned an error.",
            local_length_before, local_length_before,
        )

    raw_blocks = payload.get("chain")
    if not isinstance(raw_blocks, list) or not raw_blocks:
        return SyncResult(
            peer_address, False, "Peer response did not contain a chain.",
            local_length_before, local_length_before,
        )

    candidate_chain = _parse_candidate_chain(raw_blocks)
    if candidate_chain is None:
        return SyncResult(
            peer_address, False, "Candidate chain contained malformed blocks.",
            local_length_before, local_length_before,
        )

    accepted, reason = node.blockchain.replace_chain(candidate_chain)
    local_length_after = node.blockchain.length

    if accepted:
        logger.info(
            "Synced from peer %s: chain adopted (length %s -> %s)",
            peer_address,
            local_length_before,
            local_length_after,
        )
    else:
        logger.info("Sync from peer %s rejected: %s", peer_address, reason)

    return SyncResult(
        peer_address, accepted, reason, local_length_before, local_length_after
    )


def sync_with_all_peers(node: NetworkNode) -> list[SyncResult]:
    """
    Attempt synchronization against every known peer in turn.

    Peers are tried in the order returned by the registry; each result is
    independent, so one unreachable or inferior peer does not prevent
    trying the rest. Returns one `SyncResult` per known peer.
    """
    results = []
    for address in node.peers.addresses():
        results.append(sync_with_peer(node, address))
    return results
