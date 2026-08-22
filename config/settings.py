"""
Configuration module for SAYANJALI BLOCKCHAIN.

This module centralizes every tunable parameter of the network so that
future consensus, storage, or networking changes never require touching
business logic elsewhere in the codebase. Settings are loaded from
environment variables (with sane defaults) using a plain dataclass rather
than pulling in a heavier dependency, since the MVP has no need for it.

Usage:
    from config.settings import get_settings
    settings = get_settings()
    print(settings.difficulty)
"""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from functools import lru_cache
from pathlib import Path


BASE_DIR: Path = Path(__file__).resolve().parent.parent


def _env_int(key: str, default: int) -> int:
    """Read an integer environment variable, falling back to a default."""
    raw = os.getenv(key)
    if raw is None or raw.strip() == "":
        return default
    try:
        return int(raw)
    except ValueError:
        return default


def _env_str(key: str, default: str) -> str:
    """Read a string environment variable, falling back to a default."""
    raw = os.getenv(key)
    if raw is None or raw.strip() == "":
        return default
    return raw


@dataclass(frozen=True)
class GenesisConfig:
    """Parameters used to construct the genesis block."""

    index: int = 0
    previous_hash: str = "0" * 64
    timestamp: float = 1735689600.0  # 2025-01-01T00:00:00Z, fixed for determinism
    nonce: int = 0
    message: str = "SAYANJALI BLOCKCHAIN GENESIS BLOCK - SYJ TOKEN NETWORK"


@dataclass(frozen=True)
class ConsensusConfig:
    """
    Consensus-related parameters.

    The `algorithm` field is a string rather than an enum on purpose: it
    lets future modules (blockchain/consensus.py) register new algorithm
    names (e.g. "pos", "dpos") without changing this dataclass. The MVP
    only implements "pow" (Proof of Work).
    """

    algorithm: str = "pow"
    difficulty: int = field(default_factory=lambda: _env_int("SYJ_DIFFICULTY", 4))
    target_block_time_seconds: int = field(
        default_factory=lambda: _env_int("SYJ_TARGET_BLOCK_TIME", 30)
    )
    difficulty_adjustment_interval: int = field(
        default_factory=lambda: _env_int("SYJ_DIFFICULTY_ADJUSTMENT_INTERVAL", 10)
    )
    max_nonce: int = 2**32


@dataclass(frozen=True)
class MiningConfig:
    """Mining reward and coinbase parameters."""

    block_reward: float = field(
        default_factory=lambda: float(_env_str("SYJ_BLOCK_REWARD", "50.0"))
    )
    coinbase_address: str = "SYJ-COINBASE-0000000000000000000000000000"
    halving_interval: int = 210_000  # blocks; not enforced yet, reserved for later


@dataclass(frozen=True)
class NetworkConfig:
    """Node / API networking parameters."""

    network_name: str = field(
        default_factory=lambda: _env_str("SYJ_NETWORK_NAME", "sayanjali-mainnet-mvp")
    )
    host: str = field(default_factory=lambda: _env_str("SYJ_HOST", "0.0.0.0"))
    port: int = field(default_factory=lambda: _env_int("SYJ_PORT", 8000))
    chain_id: int = field(default_factory=lambda: _env_int("SYJ_CHAIN_ID", 1))


@dataclass(frozen=True)
class StorageConfig:
    """
    Persistence-layer parameters.

    `database_url` follows the SQLAlchemy URL format so swapping SQLite for
    PostgreSQL later is a one-line change (e.g.
    "postgresql+psycopg2://user:pass@host/db") with no code changes required
    in blockchain/storage.py, which talks to SQLAlchemy's engine interface
    exclusively.
    """

    database_dir: Path = BASE_DIR / "database"
    database_file: str = field(
        default_factory=lambda: _env_str("SYJ_DB_FILE", "sayanjali_chain.db")
    )

    @property
    def database_url(self) -> str:
        self.database_dir.mkdir(parents=True, exist_ok=True)
        return f"sqlite:///{self.database_dir / self.database_file}"


@dataclass(frozen=True)
class LoggingConfig:
    """Logging parameters."""

    log_dir: Path = BASE_DIR / "logs"
    log_file: str = "sayanjali.log"
    level: str = field(default_factory=lambda: _env_str("SYJ_LOG_LEVEL", "INFO"))
    max_bytes: int = 5 * 1024 * 1024  # 5 MB per log file
    backup_count: int = 5


@dataclass(frozen=True)
class FutureConsensusOptions:
    """
    Placeholder configuration reserved for future consensus algorithms.

    Nothing in the MVP reads these fields; they exist so that when Proof of
    Stake or Delegated Proof of Stake is implemented, the configuration
    surface already has a stable home and does not require restructuring
    `ConsensusConfig`.
    """

    pos_min_stake: float = 1000.0
    pos_unbonding_period_blocks: int = 1000
    dpos_validator_count: int = 21
    dpos_epoch_blocks: int = 100


def _env_list(key: str, default: list[str]) -> list[str]:
    """Read a comma-separated environment variable as a list of strings."""
    raw = os.getenv(key)
    if raw is None or raw.strip() == "":
        return list(default)
    return [item.strip() for item in raw.split(",") if item.strip()]


@dataclass(frozen=True)
class P2PConfig:
    """
    Peer-to-peer networking parameters (Phase 2).

    P2P endpoints are served on the same FastAPI application and port as
    the REST API (under the `/network/*` prefix) rather than a second
    listener, keeping the transport a single lightweight HTTP surface for
    this MVP. `advertised_address` exists separately from `host`/`port`
    because a node behind NAT or port-forwarding may need to tell peers a
    different externally-reachable address than the one it binds locally.
    """

    node_id_override: str = field(
        default_factory=lambda: _env_str("SYJ_NODE_ID", "")
    )
    advertised_address: str = field(
        default_factory=lambda: _env_str("SYJ_ADVERTISED_ADDRESS", "")
    )
    bootstrap_peers: list[str] = field(
        default_factory=lambda: _env_list("SYJ_BOOTSTRAP_PEERS", [])
    )
    max_peers: int = field(default_factory=lambda: _env_int("SYJ_MAX_PEERS", 25))
    sync_timeout_seconds: float = field(
        default_factory=lambda: float(_env_str("SYJ_SYNC_TIMEOUT", "10.0"))
    )
    propagation_timeout_seconds: float = field(
        default_factory=lambda: float(_env_str("SYJ_PROPAGATION_TIMEOUT", "5.0"))
    )
    max_chain_sync_bytes: int = field(
        default_factory=lambda: _env_int("SYJ_MAX_SYNC_BYTES", 8 * 1024 * 1024)
    )
    max_block_payload_bytes: int = field(
        default_factory=lambda: _env_int("SYJ_MAX_BLOCK_PAYLOAD_BYTES", 512 * 1024)
    )
    max_transaction_payload_bytes: int = field(
        default_factory=lambda: _env_int("SYJ_MAX_TX_PAYLOAD_BYTES", 16 * 1024)
    )
    rate_limit_requests: int = field(
        default_factory=lambda: _env_int("SYJ_PEER_RATE_LIMIT_REQUESTS", 60)
    )
    rate_limit_window_seconds: float = field(
        default_factory=lambda: float(_env_str("SYJ_PEER_RATE_LIMIT_WINDOW", "60.0"))
    )


@dataclass(frozen=True)
class Settings:
    """Aggregate settings object exposing every configuration group."""

    genesis: GenesisConfig = field(default_factory=GenesisConfig)
    consensus: ConsensusConfig = field(default_factory=ConsensusConfig)
    mining: MiningConfig = field(default_factory=MiningConfig)
    network: NetworkConfig = field(default_factory=NetworkConfig)
    storage: StorageConfig = field(default_factory=StorageConfig)
    logging: LoggingConfig = field(default_factory=LoggingConfig)
    future_consensus: FutureConsensusOptions = field(
        default_factory=FutureConsensusOptions
    )
    p2p: P2PConfig = field(default_factory=P2PConfig)

    # Convenience passthroughs used frequently across the codebase.
    @property
    def difficulty(self) -> int:
        return self.consensus.difficulty

    @property
    def reward(self) -> float:
        return self.mining.block_reward

    @property
    def host(self) -> str:
        return self.network.host

    @property
    def port(self) -> int:
        return self.network.port

    @property
    def database(self) -> str:
        return self.storage.database_url

    @property
    def network_name(self) -> str:
        return self.network.network_name

    @property
    def resolved_advertised_address(self) -> str:
        """
        Return the address this node should tell peers to reach it at.

        Falls back to constructing one from the bind host/port when
        SYJ_ADVERTISED_ADDRESS is not set. A bind host of 0.0.0.0 is not
        itself a reachable address, so it is rewritten to 127.0.0.1 for
        the fallback case (a node bound to all interfaces still needs to
        advertise a concrete, dialable address).
        """
        if self.p2p.advertised_address:
            return self.p2p.advertised_address.rstrip("/")
        host = self.network.host if self.network.host != "0.0.0.0" else "127.0.0.1"
        return f"http://{host}:{self.network.port}"


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    """
    Return a process-wide singleton Settings instance.

    Cached with lru_cache so every module that calls get_settings() shares
    the same configuration snapshot without needing a global variable or
    dependency-injection framework.
    """
    return Settings()
