"""
SAYANJALI BLOCKCHAIN core package.

Exposes the primary classes needed to construct and operate a node:
Block, Blockchain, Transaction, Wallet, Mempool, and the consensus/mining
engines. Kept intentionally thin -- submodules should be imported directly
in application code (api/, cli/) to avoid heavy import-time side effects.
"""

__version__ = "0.1.0-mvp"
