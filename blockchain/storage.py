"""
Persistent storage layer for SAYANJALI BLOCKCHAIN.

Built on SQLAlchemy Core (not raw sqlite3) specifically so that swapping
the backing database from SQLite to PostgreSQL later is a one-line change
to `config.settings.StorageConfig.database_url` -- no code in this module
or its callers needs to change, since everything talks to SQLAlchemy's
engine/connection interface rather than SQLite-specific APIs.

Three tables are maintained:
    blocks        - one row per block (header fields + serialized tx list)
    transactions  - one row per transaction, for fast lookups/history
    wallets       - cached wallet balances (a materialized view over the
                    transaction history, rebuilt as blocks are applied)
"""

from __future__ import annotations

import json
from typing import Optional

from sqlalchemy import (
    Column,
    Float,
    Integer,
    MetaData,
    String,
    Table,
    Text,
    create_engine,
    select,
)
from sqlalchemy.engine import Engine

from blockchain.block import Block
from blockchain.transaction import Transaction
from blockchain.utils import StorageError, get_logger

logger = get_logger("blockchain.storage")

metadata = MetaData()

blocks_table = Table(
    "blocks",
    metadata,
    Column("index", Integer, primary_key=True),
    Column("previous_hash", String(64), nullable=False),
    Column("hash", String(64), nullable=False, unique=True),
    Column("timestamp", Float, nullable=False),
    Column("nonce", Integer, nullable=False),
    Column("difficulty", Integer, nullable=False, default=0),
    Column("merkle_root", String(64), nullable=False),
    Column("transactions_json", Text, nullable=False),
)

transactions_table = Table(
    "transactions",
    metadata,
    Column("tx_hash", String(64), primary_key=True),
    Column("block_index", Integer, nullable=True),
    Column("sender", String(64), nullable=False),
    Column("receiver", String(64), nullable=False),
    Column("amount", Float, nullable=False),
    Column("timestamp", Float, nullable=False),
    Column("sender_public_key", Text, nullable=True),
    Column("signature", Text, nullable=True),
)

wallets_table = Table(
    "wallets",
    metadata,
    Column("address", String(64), primary_key=True),
    Column("balance", Float, nullable=False, default=0.0),
)


class Storage:
    """
    Persistence gateway for the blockchain, transaction history, and
    cached wallet balances.
    """

    def __init__(self, database_url: str) -> None:
        try:
            self.engine: Engine = create_engine(database_url, future=True)
            metadata.create_all(self.engine)
        except Exception as exc:  # noqa: BLE001
            raise StorageError(f"Failed to initialize database: {exc}") from exc
        logger.info("Storage initialized at %s", database_url)

    # ------------------------------------------------------------------ #
    # Block persistence
    # ------------------------------------------------------------------ #

    def save_block(self, block: Block) -> None:
        """Persist a block and all of its transactions."""
        try:
            with self.engine.begin() as conn:
                conn.execute(
                    blocks_table.insert().values(
                        **{
                            "index": block.index,
                            "previous_hash": block.previous_hash,
                            "hash": block.hash,
                            "timestamp": block.timestamp,
                            "nonce": block.nonce,
                            "difficulty": block.difficulty,
                            "merkle_root": block.merkle_root,
                            "transactions_json": json.dumps(
                                [tx.to_dict() for tx in block.transactions]
                            ),
                        }
                    )
                )
                for tx in block.transactions:
                    conn.execute(
                        transactions_table.insert().values(
                            tx_hash=tx.tx_hash,
                            block_index=block.index,
                            sender=tx.sender,
                            receiver=tx.receiver,
                            amount=tx.amount,
                            timestamp=tx.timestamp,
                            sender_public_key=tx.sender_public_key,
                            signature=tx.signature,
                        )
                    )
        except Exception as exc:  # noqa: BLE001
            raise StorageError(f"Failed to save block {block.index}: {exc}") from exc

    def load_chain(self) -> list[Block]:
        """Load every block from storage, ordered by index, as Block objects."""
        try:
            with self.engine.connect() as conn:
                rows = conn.execute(
                    select(blocks_table).order_by(blocks_table.c.index)
                ).fetchall()
        except Exception as exc:  # noqa: BLE001
            raise StorageError(f"Failed to load chain: {exc}") from exc

        chain: list[Block] = []
        for row in rows:
            data = row._mapping
            block_dict = {
                "index": data["index"],
                "previous_hash": data["previous_hash"],
                "hash": data["hash"],
                "timestamp": data["timestamp"],
                "nonce": data["nonce"],
                "difficulty": data["difficulty"],
                "merkle_root": data["merkle_root"],
                "transactions": json.loads(data["transactions_json"]),
            }
            chain.append(Block.from_dict(block_dict))
        return chain

    def get_block(self, index: int) -> Optional[Block]:
        """Fetch a single block by its index, or None if not found."""
        try:
            with self.engine.connect() as conn:
                row = conn.execute(
                    select(blocks_table).where(blocks_table.c.index == index)
                ).fetchone()
        except Exception as exc:  # noqa: BLE001
            raise StorageError(f"Failed to load block {index}: {exc}") from exc

        if row is None:
            return None
        data = row._mapping
        return Block.from_dict(
            {
                "index": data["index"],
                "previous_hash": data["previous_hash"],
                "hash": data["hash"],
                "timestamp": data["timestamp"],
                "nonce": data["nonce"],
                "difficulty": data["difficulty"],
                "merkle_root": data["merkle_root"],
                "transactions": json.loads(data["transactions_json"]),
            }
        )

    def chain_length(self) -> int:
        """Return the number of blocks currently persisted."""
        with self.engine.connect() as conn:
            result = conn.execute(select(blocks_table.c.index)).fetchall()
        return len(result)

    # ------------------------------------------------------------------ #
    # Wallet balance cache
    # ------------------------------------------------------------------ #

    def get_balance(self, address: str) -> float:
        """
        Return the cached balance for `address`, computing it from the
        full transaction history if no cache row exists yet.
        """
        with self.engine.connect() as conn:
            row = conn.execute(
                select(wallets_table.c.balance).where(
                    wallets_table.c.address == address
                )
            ).fetchone()
        if row is not None:
            return float(row[0])
        return self._compute_balance_from_history(address)

    def _compute_balance_from_history(self, address: str) -> float:
        """Recompute an address's balance from the full transaction ledger."""
        with self.engine.connect() as conn:
            incoming = conn.execute(
                select(transactions_table.c.amount).where(
                    transactions_table.c.receiver == address
                )
            ).fetchall()
            outgoing = conn.execute(
                select(transactions_table.c.amount).where(
                    transactions_table.c.sender == address
                )
            ).fetchall()
        total_in = sum(row[0] for row in incoming)
        total_out = sum(row[0] for row in outgoing)
        return total_in - total_out

    def apply_balance_delta(self, address: str, delta: float) -> None:
        """Atomically adjust an address's cached balance by `delta`."""
        with self.engine.begin() as conn:
            row = conn.execute(
                select(wallets_table.c.balance).where(
                    wallets_table.c.address == address
                )
            ).fetchone()
            if row is None:
                conn.execute(
                    wallets_table.insert().values(address=address, balance=delta)
                )
            else:
                new_balance = float(row[0]) + delta
                conn.execute(
                    wallets_table.update()
                    .where(wallets_table.c.address == address)
                    .values(balance=new_balance)
                )

    def update_balances_for_block(self, block: Block) -> None:
        """Apply every transaction in `block` to the wallet balance cache."""
        for tx in block.transactions:
            if not tx.is_coinbase():
                self.apply_balance_delta(tx.sender, -tx.amount)
            self.apply_balance_delta(tx.receiver, tx.amount)
