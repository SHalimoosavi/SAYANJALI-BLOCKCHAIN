"""
FastAPI application entrypoint for SAYANJALI BLOCKCHAIN.

Run directly with:
    python -m api.main
or via uvicorn:
    uvicorn api.main:app --host 0.0.0.0 --port 8000
"""

from __future__ import annotations

from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from api.middleware import MaxBodySizeMiddleware
from api.network_routes import router as network_router
from api.routes import router
from blockchain.utils import get_logger
from config.settings import get_settings

logger = get_logger("api.main")
settings = get_settings()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Own the NetworkNode lifecycle with the FastAPI application lifetime."""
    from api.network_routes import get_network_node
    node = get_network_node()
    logger.info(
        "SAYANJALI BLOCKCHAIN node starting | network=%s | chain_id=%s | host=%s | port=%s | difficulty=%s",
        settings.network_name, settings.network.chain_id, settings.host, settings.port, settings.difficulty,
    )
    await node.start()
    try:
        yield
    finally:
        await node.stop()
        logger.info("SAYANJALI BLOCKCHAIN node stopped cleanly.")


app = FastAPI(
    title="SAYANJALI BLOCKCHAIN Node API",
    description=(
        "REST API for the SAYANJALI BLOCKCHAIN MVP, powering the SYJ Token "
        "network operated by SAYANJALI NEXUS PRIVATE LIMITED."
    ),
    version="0.2.0-mvp",
    lifespan=lifespan,
)

# CORS is permissive for the MVP so a locally-hosted explorer/dashboard on
# any origin (e.g. a Termux-hosted frontend) can talk to the node freely.
# Tighten this before any production/mainnet deployment.
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Phase 6.5: enforces actual request body byte limits before FastAPI/
# Pydantic ever parses a body, closing the gap where the old per-route
# Content-Length check ran too late to matter. Per-path overrides give
# small handshake/registration payloads a tight cap while block
# propagation gets the larger, block-specific limit; everything else
# (including GET requests, which carry no body) falls back to the
# generic default.
app.add_middleware(
    MaxBodySizeMiddleware,
    default_max_bytes=settings.p2p.max_request_body_bytes,
    path_overrides={
        "/network/blocks/receive": settings.p2p.max_block_payload_bytes,
        "/network/transactions/receive": settings.p2p.max_transaction_payload_bytes,
        "/network/peers/register": settings.p2p.max_handshake_payload_bytes,
        "/network/peers/challenge": settings.p2p.max_handshake_payload_bytes,
        "/network/peers/authenticate": settings.p2p.max_handshake_payload_bytes,
        "/network/sync": settings.p2p.max_handshake_payload_bytes,
    },
)

app.include_router(router)
app.include_router(network_router)


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "api.main:app",
        host=settings.host,
        port=settings.port,
        reload=False,
    )
