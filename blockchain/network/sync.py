"""
Chain synchronization for SAYANJALI BLOCKCHAIN's P2P layer.

Implements the deterministic sync procedure: fetch a peer's chain,
validate it completely, compare accumulated proof-of-work against the
local chain, and adopt it only if it is both valid and superior. All
validation and adoption logic is delegated to `blockchain.validators` and
`Blockchain.replace_chain` -- this module only orchestrates the network
round-trip and the decision of whether to call them.

Phase 6.5 hardening applied here:
    - The outbound target address is validated through the same
      centralized `address_security.validate_peer_address()` every other
      outbound call uses (closing the SSRF gap where `/network/sync`
      previously accepted an arbitrary, unvalidated `peer_address`).
    - A resource-limit gate (block count) runs before any block is
      parsed, so an oversized or malicious response cannot force this
      node to spend unbounded CPU/memory on `Block.from_dict()` calls
      before being rejected.
    - `chain_work()` on a candidate is only ever used as a pre-filter to
      decide whether full validation is worth attempting; a chain is
      never actually adopted (see `Blockchain.replace_chain`) without
      passing complete structural and consensus validation, so a
      fabricated `difficulty` value can make this node *attempt*
      validation but can never make it *succeed* without genuine proof
      of work. The resource-limit gate is what bounds the cost of that
      attempt, not a reordering of the work-comparison itself.
"""

from __future__ import annotations

from dataclasses import dataclass

from blockchain.block import Block
from blockchain.network.address_security import validate_peer_address
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


def _parse_candidate_chain(
    raw_blocks: list, max_blocks: int
) -> tuple[list[Block] | None, str]:
    """
    Parse a peer-supplied list of block dicts into `Block` objects.

    The block-count check happens before any parsing is attempted, so an
    oversized response is rejected at the cost of a single `len()` call
    rather than however many `Block.from_dict()` calls it would take to
    discover the same thing partway through.

    Returns:
        (blocks_or_none, reason) -- reason is empty on success.
    """
    if len(raw_blocks) > max_blocks:
        return None, (
            f"Candidate chain has {len(raw_blocks)} blocks, exceeding the "
            f"configured maximum of {max_blocks}."
        )
    try:
        return [Block.from_dict(b) for b in raw_blocks], ""
    except (KeyError, TypeError, ValueError) as exc:
        return None, f"Candidate chain contained malformed blocks: {exc}"


def sync_with_peer(node: NetworkNode, peer_address: str) -> SyncResult:
    """
    Attempt to synchronize this node's chain from a single peer.

    Follows the required sequence exactly: validate the target address,
    request the peer's chain, enforce resource limits, validate it
    completely, reject it outright if invalid or inferior, and only ever
    adopt a chain that is both valid and represents more accumulated work
    than the local chain. A locally valid chain is never replaced with an
    inferior or invalid one, regardless of what a peer sends.
    """
    local_length_before = node.blockchain.length

    address_ok, address_reason = validate_peer_address(
        peer_address, node.settings.p2p.allow_private_peer_addresses
    )
    if not address_ok:
        return SyncResult(
            peer_address, False, f"Invalid peer address: {address_reason}",
            local_length_before, local_length_before,
        )

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

    candidate_chain, parse_reason = _parse_candidate_chain(
        raw_blocks, node.settings.p2p.max_sync_blocks
    )
    if candidate_chain is None:
        return SyncResult(
            peer_address, False, parse_reason, local_length_before, local_length_before,
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
