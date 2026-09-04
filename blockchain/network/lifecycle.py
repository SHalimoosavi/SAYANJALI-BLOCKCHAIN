"""Explicit lifecycle states for a testnet node."""
from __future__ import annotations
from enum import Enum

class NodeLifecycle(str, Enum):
    STARTING = "STARTING"
    RUNNING = "RUNNING"
    SYNCING = "SYNCING"
    DEGRADED = "DEGRADED"
    STOPPING = "STOPPING"
    STOPPED = "STOPPED"
