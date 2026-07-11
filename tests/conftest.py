"""
Shared pytest fixtures for SAYANJALI BLOCKCHAIN tests.

Each test gets a fresh Blockchain backed by a temporary SQLite file so
tests never share or pollute state, and so the developer's own
database/sayanjali_chain.db is never touched by the test suite.
"""

from __future__ import annotations

import os
from pathlib import Path

import pytest


@pytest.fixture(autouse=True)
def isolated_settings(tmp_path: Path, monkeypatch: pytest.MonkeyPatch):
    """
    Point every test at an isolated, temporary SQLite database and clear
    the cached Settings singleton before and after each test so tests
    never leak configuration or data between one another.
    """
    from config import settings as settings_module

    monkeypatch.setenv("SYJ_DB_FILE", f"test_{os.urandom(4).hex()}.db")
    monkeypatch.setenv("SYJ_DIFFICULTY", "2")  # keep PoW fast in tests

    settings_module.get_settings.cache_clear()
    yield
    settings_module.get_settings.cache_clear()


@pytest.fixture
def blockchain():
    """Return a fresh Blockchain instance for a single test."""
    from blockchain.blockchain import Blockchain

    return Blockchain()
