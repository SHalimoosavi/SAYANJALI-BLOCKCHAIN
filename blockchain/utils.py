"""
Shared utility functions for SAYANJALI BLOCKCHAIN.

Contains hashing helpers, deterministic JSON serialization, a Merkle root
implementation, and the structured logging setup used across every module.
Centralizing these here avoids subtle bugs where two modules hash the same
logical data differently (e.g. dict key ordering) and disagree on a block's
hash.
"""

from __future__ import annotations

import hashlib
import json
import logging
import logging.handlers
import sys
import time
from pathlib import Path
from typing import Any, Iterable

from config.settings import get_settings


def sha256(data: str) -> str:
    """Return the hex-encoded SHA-256 digest of a UTF-8 string."""
    return hashlib.sha256(data.encode("utf-8")).hexdigest()

def double_sha256(data: str) -> str:
    """Return SHA-256 applied twice, mirroring common blockchain practice."""
    return sha256(sha256(data))


def deterministic_json(payload: Any) -> str:
    """
    Serialize a payload to JSON with sorted keys and no extra whitespace.

    Deterministic serialization is required so that identical logical
    content always produces an identical hash, regardless of dict
    insertion order or platform.
    """
    return json.dumps(payload, sort_keys=True, separators=(",", ":"), default=str)


def current_timestamp() -> float:
    """Return the current UNIX timestamp as a float."""
    return time.time()


def merkle_root(transaction_hashes: Iterable[str]) -> str:
    """
    Compute a Merkle root from a list of transaction hashes.

    If the list is empty, returns the hash of an empty string (a common
    convention for empty blocks). If the list has an odd number of
    elements at any level, the last hash is duplicated, which is the
    standard Bitcoin-style approach.
    """
    hashes = list(transaction_hashes)
    if not hashes:
        return sha256("")

    while len(hashes) > 1:
        if len(hashes) % 2 == 1:
            hashes.append(hashes[-1])
        hashes = [
            sha256(hashes[i] + hashes[i + 1]) for i in range(0, len(hashes), 2)
        ]
    return hashes[0]


def is_valid_hex_hash(value: str, length: int = 64) -> bool:
    """Check whether a string looks like a valid hex-encoded SHA-256 hash."""
    if not isinstance(value, str) or len(value) != length:
        return False
    try:
        int(value, 16)
        return True
    except ValueError:
        return False


_LOGGERS: dict[str, logging.Logger] = {}


def get_logger(name: str) -> logging.Logger:
    """
    Return a configured logger writing to logs/sayanjali.log and stdout.

    Loggers are cached per name so repeated calls do not attach duplicate
    handlers. Log level and destination are controlled centrally via
    config.settings.LoggingConfig.
    """
    if name in _LOGGERS:
        return _LOGGERS[name]

    settings = get_settings()
    log_cfg = settings.logging
    log_cfg.log_dir.mkdir(parents=True, exist_ok=True)

    logger = logging.getLogger(name)
    logger.setLevel(getattr(logging, log_cfg.level.upper(), logging.INFO))
    logger.propagate = False

    if not logger.handlers:
        formatter = logging.Formatter(
            fmt="%(asctime)s | %(levelname)-8s | %(name)s | %(message)s",
            datefmt="%Y-%m-%d %H:%M:%S",
        )

        file_handler = logging.handlers.RotatingFileHandler(
            filename=log_cfg.log_dir / log_cfg.log_file,
            maxBytes=log_cfg.max_bytes,
            backupCount=log_cfg.backup_count,
            encoding="utf-8",
        )
        file_handler.setFormatter(formatter)
        file_handler.setLevel(logging.DEBUG)

        stream_handler = logging.StreamHandler(stream=sys.stdout)
        stream_handler.setFormatter(formatter)
        stream_handler.setLevel(logging.INFO)

        logger.addHandler(file_handler)
        logger.addHandler(stream_handler)

    _LOGGERS[name] = logger
    return logger


class SayanjaliError(Exception):
    """Base exception for all custom SAYANJALI BLOCKCHAIN errors."""


class ValidationError(SayanjaliError):
    """Raised when block, transaction, or chain validation fails."""


class StorageError(SayanjaliError):
    """Raised when a persistence operation fails."""


class WalletError(SayanjaliError):
    """Raised for wallet creation, import, or signing failures."""
