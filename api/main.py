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

from api.network_routes import router as network_router
from api.routes import router
from blockchain.utils import get_logger
from config.settings import get_settings

logger = get_logger("api.main")
settings = get_settings()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Log node startup and shutdown around the application's lifetime."""
    logger.info(
        "SAYANJALI BLOCKCHAIN node starting | network=%s | host=%s | port=%s | difficulty=%s",
        settings.network_name,
        settings.host,
        settings.port,
        settings.difficulty,
    )
    yield
    logger.info("SAYANJALI BLOCKCHAIN node shutting down.")


app = FastAPI(
    title="SAYANJALI BLOCKCHAIN Node API",
    description=(
        "REST API for the SAYANJALI BLOCKCHAIN MVP, powering the SYJ Token "
        "network operated by SAYANJALI NEXUS PRIVATE LIMITED."
    ),
    version="0.1.0-mvp",
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
