"""
API route definitions for SAYANJALI BLOCKCHAIN.

Every endpoint required by the project specification is implemented here:
    GET  /chain
    GET  /block/{index}
    GET  /wallet/{address}
    POST /wallet/create
    POST /transaction/create
    POST /transaction/submit
    POST /mine
    GET  /transactions/pending
    GET  /status
    GET  /health

Phase 6.5: POST /transaction/sign was removed entirely -- private key
material must never reach this API. Signing happens exclusively
client-side (see blockchain.wallet.Wallet.sign). /transaction/create
still exists to hand back a canonical, timestamped envelope for the
client to sign locally before calling /transaction/submit.
"""

from __future__ import annotations

from fastapi import APIRouter, HTTPException

from api.schemas import (
    BlockOut,
    ChainOut,
    HealthResponse,
    MineRequest,
    MineResponse,
    StatusResponse,
    TransactionCreateRequest,
    TransactionOut,
    TransactionSubmitRequest,
    TransactionSubmitResponse,
    WalletBalanceResponse,
    WalletCreateResponse,
)
from blockchain.blockchain import Blockchain
from blockchain.transaction import Transaction
from blockchain.utils import ValidationError, get_logger
from blockchain.wallet import Wallet, is_valid_address

logger = get_logger("api.routes")
router = APIRouter()

# A single in-process Blockchain instance backs this node. FastAPI's
# dependency-injection system is intentionally not used here to keep the
# MVP simple; api/main.py constructs this instance once at startup.
_blockchain: Blockchain | None = None


def get_blockchain() -> Blockchain:
    """Return the module-level Blockchain instance, initializing if needed."""
    global _blockchain
    if _blockchain is None:
        _blockchain = Blockchain()
    return _blockchain


def _block_to_out(block) -> BlockOut:
    """Convert an internal Block into its API representation."""
    return BlockOut(
        index=block.index,
        previous_hash=block.previous_hash,
        timestamp=block.timestamp,
        nonce=block.nonce,
        difficulty=block.difficulty,
        merkle_root=block.merkle_root,
        hash=block.hash,
        transactions=[
            TransactionOut(
                sender=tx.sender,
                receiver=tx.receiver,
                amount=tx.amount,
                timestamp=tx.timestamp,
                sender_public_key=tx.sender_public_key,
                signature=tx.signature,
                tx_hash=tx.tx_hash,
            )
            for tx in block.transactions
        ],
    )


@router.get("/health", response_model=HealthResponse, tags=["system"])
def health() -> HealthResponse:
    """Liveness probe. Always returns ok if the process is running."""
    return HealthResponse()


@router.get("/status", response_model=StatusResponse, tags=["system"])
def status() -> StatusResponse:
    """Return a summary of the node's current chain state."""
    chain = get_blockchain()
    return StatusResponse(**chain.status())


@router.get("/chain", response_model=ChainOut, tags=["chain"])
def get_chain() -> ChainOut:
    """Return the full blockchain."""
    chain = get_blockchain()
    return ChainOut(
        length=chain.length,
        chain=[_block_to_out(b) for b in chain.chain],
    )


@router.get("/block/{index}", response_model=BlockOut, tags=["chain"])
def get_block(index: int) -> BlockOut:
    """Return a single block by its index."""
    chain = get_blockchain()
    block = chain.get_block(index)
    if block is None:
        raise HTTPException(status_code=404, detail=f"Block {index} not found.")
    return _block_to_out(block)


@router.post("/wallet/create", response_model=WalletCreateResponse, tags=["wallet"])
def create_wallet() -> WalletCreateResponse:
    """Generate a brand-new wallet (key pair + address)."""
    wallet = Wallet.create()
    return WalletCreateResponse(
        address=wallet.address,
        public_key=wallet.public_key_hex,
        private_key=wallet.private_key_hex,
    )


@router.get("/wallet/{address}", response_model=WalletBalanceResponse, tags=["wallet"])
def get_wallet_balance(address: str) -> WalletBalanceResponse:
    """Return the confirmed balance for a wallet address."""
    if not is_valid_address(address):
        raise HTTPException(status_code=400, detail="Malformed wallet address.")
    chain = get_blockchain()
    balance = chain.get_balance(address)
    return WalletBalanceResponse(address=address, balance=balance)


@router.post("/transaction/create", response_model=TransactionOut, tags=["transaction"])
def create_transaction(payload: TransactionCreateRequest) -> TransactionOut:
    """
    Build an unsigned transaction envelope from sender/receiver/amount.

    This does not touch the mempool and never handles private key
    material -- it only returns the exact payload the client must sign
    entirely locally (see blockchain.wallet.Wallet.sign) before calling
    POST /transaction/submit.
    """
    if not is_valid_address(payload.receiver):
        raise HTTPException(status_code=400, detail="Malformed receiver address.")

    tx = Transaction(
        sender=payload.sender, receiver=payload.receiver, amount=payload.amount
    )
    return TransactionOut(**tx.to_dict())


# Phase 6.5 security decision: the previous POST /transaction/sign endpoint
# (which accepted a raw private_key over HTTP) has been removed entirely,
# not merely deprecated. Signing now happens exclusively client-side --
# see blockchain.wallet.Wallet.sign / blockchain.transaction.Transaction.sign,
# which cli/main.py's create-transaction command already used even before
# this change. This endpoint's removal means private key material can no
# longer reach this API by any documented path; only a fully-signed
# transaction (POST /transaction/submit) is ever accepted.


@router.post(
    "/transaction/submit",
    response_model=TransactionSubmitResponse,
    tags=["transaction"],
)
def submit_transaction(payload: TransactionSubmitRequest) -> TransactionSubmitResponse:
    """Submit a fully-signed transaction to the mempool."""
    tx = Transaction(
        sender=payload.sender,
        receiver=payload.receiver,
        amount=payload.amount,
        timestamp=payload.timestamp,
        sender_public_key=payload.sender_public_key,
        signature=payload.signature,
    )
    chain = get_blockchain()
    accepted, reason = chain.submit_transaction(tx)

    if accepted:
        # Phase 2 hook: propagate newly accepted transactions to known
        # peers. Imported lazily to avoid a circular import, since
        # api.network_routes imports get_blockchain from this module.
        # A broadcast failure never affects whether the transaction was
        # accepted locally -- it is best-effort only.
        try:
            from api.network_routes import get_network_node
            from blockchain.network.propagation import broadcast_transaction

            broadcast_transaction(get_network_node(), tx)
        except Exception as exc:  # noqa: BLE001
            logger.warning("Transaction broadcast failed: %s", exc)

    return TransactionSubmitResponse(
        accepted=accepted,
        tx_hash=tx.tx_hash if accepted else None,
        reason=reason or None,
    )


@router.get(
    "/transactions/pending", response_model=list[TransactionOut], tags=["transaction"]
)
def get_pending_transactions() -> list[TransactionOut]:
    """Return every transaction currently waiting in the mempool."""
    chain = get_blockchain()
    return [TransactionOut(**tx.to_dict()) for tx in chain.get_pending_transactions()]


@router.post("/mine", response_model=MineResponse, tags=["mining"])
def mine(payload: MineRequest) -> MineResponse:
    """Mine a new block, rewarding `miner_address` with the block reward."""
    if not is_valid_address(payload.miner_address):
        raise HTTPException(status_code=400, detail="Malformed miner address.")

    chain = get_blockchain()
    try:
        block = chain.mine_pending_transactions(payload.miner_address)
    except ValidationError as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc

    # Phase 2 hook: propagate the newly mined block to known peers.
    # Best-effort -- a broadcast failure never undoes local mining.
    try:
        from api.network_routes import get_network_node
        from blockchain.network.propagation import broadcast_block

        broadcast_block(get_network_node(), block)
    except Exception as exc:  # noqa: BLE001
        logger.warning("Block broadcast failed: %s", exc)

    return MineResponse(block=_block_to_out(block))
