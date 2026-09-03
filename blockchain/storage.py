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
import time
import uuid
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
from blockchain.native_asset import to_base_units
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
    Column("amount_base_units", Integer, nullable=False),
    Column("timestamp", Float, nullable=False),
    Column("sender_public_key", Text, nullable=True),
    Column("signature", Text, nullable=True),
)

wallets_table = Table(
    "wallets",
    metadata,
    Column("address", String(64), primary_key=True),
    Column("balance_base_units", Integer, nullable=False, default=0),
)

peers_table = Table(
    "peers",
    metadata,
    Column("address", String(256), primary_key=True),
    Column("node_id", String(64), nullable=True),
    Column("status", String(16), nullable=False, default="unknown"),
    Column("last_seen", Float, nullable=True),
    Column("registered_at", Float, nullable=False),
)

node_identity_table = Table(
    "node_identity",
    metadata,
    Column("id", Integer, primary_key=True),
    Column("node_id", String(64), nullable=False),
    Column("created_at", Float, nullable=False),
)

p2p_identity_table = Table(
    "p2p_identity",
    metadata,
    Column("node_id", String(64), primary_key=True),
    Column("private_key_hex", String(128), nullable=False),
    Column("public_key_hex", String(256), nullable=False),
    Column("created_at", Float, nullable=False),
)

peer_credentials_table = Table(
    "peer_credentials",
    metadata,
    Column("address", String(256), primary_key=True),
    Column("node_id", String(64), nullable=False),
    Column("public_key_hex", String(256), nullable=False),
    Column("trusted", Integer, nullable=False, default=0),
    Column("authenticated_at", Float, nullable=False),
)


class Storage:
    """
    Persistence gateway for the blockchain, transaction history, and
    cached wallet balances.
    """

    def __init__(self, database_url: str) -> None:
        try:
            self.engine: Engine = create_engine(database_url, future=True)
            self._migrate_legacy_monetary_schema()
            metadata.create_all(self.engine)
        except Exception as exc:  # noqa: BLE001
            raise StorageError(f"Failed to initialize database: {exc}") from exc
        logger.info("Storage initialized at %s", database_url)

    def _migrate_legacy_monetary_schema(self) -> None:
        """Migrate legacy float monetary tables to integer base-unit tables."""
        from sqlalchemy import inspect, text

        inspector = inspect(self.engine)
        tables = set(inspector.get_table_names())

        if "transactions" in tables:
            columns = {c["name"] for c in inspector.get_columns("transactions")}
            if "amount" in columns and "amount_base_units" not in columns:
                with self.engine.begin() as conn:
                    conn.execute(text("ALTER TABLE transactions RENAME TO transactions_legacy"))
                    metadata.create_all(conn)
                    rows = conn.execute(text(
                        "SELECT tx_hash, block_index, sender, receiver, amount, timestamp, "
                        "sender_public_key, signature FROM transactions_legacy"
                    )).fetchall()
                    for row in rows:
                        conn.execute(
                            text(
                                "INSERT INTO transactions "
                                "(tx_hash, block_index, sender, receiver, amount_base_units, timestamp, "
                                "sender_public_key, signature) VALUES "
                                "(:h, :bi, :s, :r, :a, :ts, :pk, :sig)"
                            ),
                            {
                                "h": row[0], "bi": row[1], "s": row[2], "r": row[3],
                                "a": to_base_units(str(row[4])), "ts": row[5],
                                "pk": row[6], "sig": row[7],
                            },
                        )
                    conn.execute(text("DROP TABLE transactions_legacy"))

        inspector = inspect(self.engine)
        tables = set(inspector.get_table_names())
        if "wallets" in tables:
            columns = {c["name"] for c in inspector.get_columns("wallets")}
            if "balance" in columns and "balance_base_units" not in columns:
                with self.engine.begin() as conn:
                    conn.execute(text("ALTER TABLE wallets RENAME TO wallets_legacy"))
                    metadata.create_all(conn)
                    rows = conn.execute(text("SELECT address, balance FROM wallets_legacy")).fetchall()
                    for address, balance in rows:
                        conn.execute(
                            text("INSERT INTO wallets (address, balance_base_units) VALUES (:a, :b)"),
                            {"a": address, "b": to_base_units(str(balance))},
                        )
                    conn.execute(text("DROP TABLE wallets_legacy"))

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
                            amount_base_units=tx.amount_base_units,
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

    def get_balance(self, address: str) -> int:
        """
        Return the cached balance for `address` in integer base units,
        computing it from transaction history if no cache row exists yet.
        """
        with self.engine.connect() as conn:
            row = conn.execute(
                select(wallets_table.c.balance_base_units).where(
                    wallets_table.c.address == address
                )
            ).fetchone()
        if row is not None:
            return int(row[0])
        return self._compute_balance_from_history(address)

    def _compute_balance_from_history(self, address: str) -> int:
        """Recompute an address's balance from the full transaction ledger."""
        with self.engine.connect() as conn:
            incoming = conn.execute(
                select(transactions_table.c.amount_base_units).where(
                    transactions_table.c.receiver == address
                )
            ).fetchall()
            outgoing = conn.execute(
                select(transactions_table.c.amount_base_units).where(
                    transactions_table.c.sender == address
                )
            ).fetchall()
        total_in = sum(row[0] for row in incoming)
        total_out = sum(row[0] for row in outgoing)
        return total_in - total_out

    def apply_balance_delta(self, address: str, delta: int) -> None:
        """Atomically adjust an address's cached balance by `delta`."""
        with self.engine.begin() as conn:
            self._apply_balance_delta_conn(conn, address, delta)

    @staticmethod
    def _apply_balance_delta_conn(conn, address: str, delta: int) -> None:
        """
        Adjust an address's cached balance by `delta` using an already-open
        connection/transaction. Factored out of `apply_balance_delta` so
        `reorganize_from` can apply many deltas atomically within one
        transaction instead of one commit per address.
        """
        row = conn.execute(
            select(wallets_table.c.balance_base_units).where(wallets_table.c.address == address)
        ).fetchone()
        if row is None:
            conn.execute(wallets_table.insert().values(address=address, balance_base_units=delta))
        else:
            new_balance = int(row[0]) + delta
            conn.execute(
                wallets_table.update()
                .where(wallets_table.c.address == address)
                .values(balance_base_units=new_balance)
            )

    def update_balances_for_block(self, block: Block) -> None:
        """Apply every transaction in `block` to the wallet balance cache."""
        for tx in block.transactions:
            if not tx.is_coinbase():
                self.apply_balance_delta(tx.sender, -tx.amount_base_units)
            self.apply_balance_delta(tx.receiver, tx.amount_base_units)

    def reorganize_from(self, fork_index: int, new_blocks: list[Block]) -> None:
        """
        Replace every block from `fork_index` onward with `new_blocks`,
        atomically, including reversing and reapplying wallet balances.

        Used when adopting a competing chain that diverges below the
        current tip (a reorg), rather than one that simply extends it.
        Blocks below `fork_index` are left untouched. All of this happens
        in a single transaction so a crash mid-reorg cannot leave the
        database in a state with neither chain fully persisted.
        """
        try:
            with self.engine.begin() as conn:
                removed_rows = conn.execute(
                    select(blocks_table)
                    .where(blocks_table.c.index >= fork_index)
                    .order_by(blocks_table.c.index.desc())
                ).fetchall()

                # Reverse balance effects of the blocks being discarded,
                # most recent first, mirroring how they were applied.
                for row in removed_rows:
                    data = row._mapping
                    for tx_dict in json.loads(data["transactions_json"]):
                        tx = Transaction.from_dict(tx_dict)
                        if not tx.is_coinbase():
                            self._apply_balance_delta_conn(conn, tx.sender, tx.amount_base_units)
                        self._apply_balance_delta_conn(conn, tx.receiver, -tx.amount_base_units)

                conn.execute(
                    transactions_table.delete().where(
                        transactions_table.c.block_index >= fork_index
                    )
                )
                conn.execute(
                    blocks_table.delete().where(blocks_table.c.index >= fork_index)
                )

                for block in new_blocks:
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
                                amount_base_units=tx.amount_base_units,
                                timestamp=tx.timestamp,
                                sender_public_key=tx.sender_public_key,
                                signature=tx.signature,
                            )
                        )
                        if not tx.is_coinbase():
                            self._apply_balance_delta_conn(conn, tx.sender, -tx.amount_base_units)
                        self._apply_balance_delta_conn(conn, tx.receiver, tx.amount_base_units)
        except Exception as exc:  # noqa: BLE001
            raise StorageError(f"Failed to reorganize chain from index {fork_index}: {exc}") from exc

    # ------------------------------------------------------------------ #
    # Node identity
    # ------------------------------------------------------------------ #

    def get_or_create_node_id(self) -> str:
        """
        Return this node's persistent identity, generating and storing a
        new one on first use so the node presents a stable identity to
        peers across restarts.
        """
        with self.engine.begin() as conn:
            row = conn.execute(select(node_identity_table)).fetchone()
            if row is not None:
                return row._mapping["node_id"]

            new_id = uuid.uuid4().hex
            conn.execute(
                node_identity_table.insert().values(
                    node_id=new_id, created_at=time.time()
                )
            )
            return new_id

    # ------------------------------------------------------------------ #
    # Peer registry
    # ------------------------------------------------------------------ #

    def upsert_peer(
        self,
        address: str,
        node_id: Optional[str] = None,
        status: str = "unknown",
        last_seen: Optional[float] = None,
    ) -> None:
        """Insert a new peer or update an existing one's known fields."""
        with self.engine.begin() as conn:
            row = conn.execute(
                select(peers_table).where(peers_table.c.address == address)
            ).fetchone()
            if row is None:
                conn.execute(
                    peers_table.insert().values(
                        address=address,
                        node_id=node_id,
                        status=status,
                        last_seen=last_seen,
                        registered_at=time.time(),
                    )
                )
            else:
                update_values: dict = {"status": status}
                if node_id is not None:
                    update_values["node_id"] = node_id
                if last_seen is not None:
                    update_values["last_seen"] = last_seen
                conn.execute(
                    peers_table.update()
                    .where(peers_table.c.address == address)
                    .values(**update_values)
                )

    def list_peers(self) -> list[dict]:
        """Return every known peer as a plain dict, most recently seen first."""
        with self.engine.connect() as conn:
            rows = conn.execute(
                select(peers_table).order_by(peers_table.c.registered_at.desc())
            ).fetchall()
        return [dict(row._mapping) for row in rows]

    def get_peer(self, address: str) -> Optional[dict]:
        """Return a single peer record by address, or None if unknown."""
        with self.engine.connect() as conn:
            row = conn.execute(
                select(peers_table).where(peers_table.c.address == address)
            ).fetchone()
        return dict(row._mapping) if row is not None else None

    def peer_count(self) -> int:
        """Return the number of known peers."""
        with self.engine.connect() as conn:
            rows = conn.execute(select(peers_table.c.address)).fetchall()
        return len(rows)

    def remove_peer(self, address: str) -> None:
        """Remove a peer from the registry."""
        with self.engine.begin() as conn:
            conn.execute(peers_table.delete().where(peers_table.c.address == address))

    # ------------------------------------------------------------------ #
    # P2P identity (Phase 6.5) -- cryptographic binding for this node's
    # own persistent node_id, kept in a dedicated table, never mixed with
    # wallet or chain data.
    # ------------------------------------------------------------------ #

    def get_or_create_p2p_identity(self, node_id: str) -> tuple[str, str]:
        """
        Return (private_key_hex, public_key_hex) for this node's P2P
        identity, generating and persisting a new SECP256k1 keypair bound
        to `node_id` on first use.

        `node_id` is passed in rather than generated here because it must
        match the pre-existing `node_identity` table's value -- the
        node's human-facing identifier is unchanged by Phase 6.5; this
        method only adds a verifiable keypair behind it.
        """
        from blockchain.network.identity import P2PIdentity

        with self.engine.begin() as conn:
            row = conn.execute(
                select(p2p_identity_table).where(
                    p2p_identity_table.c.node_id == node_id
                )
            ).fetchone()
            if row is not None:
                data = row._mapping
                return data["private_key_hex"], data["public_key_hex"]

            identity = P2PIdentity.generate(node_id)
            conn.execute(
                p2p_identity_table.insert().values(
                    node_id=node_id,
                    private_key_hex=identity.private_key_hex,
                    public_key_hex=identity.public_key_hex,
                    created_at=time.time(),
                )
            )
            return identity.private_key_hex, identity.public_key_hex

    # ------------------------------------------------------------------ #
    # Peer credentials (Phase 6.5) -- what has been cryptographically
    # verified about a peer, distinct from the basic `peers` discovery
    # registry. An address only appears here once it has successfully
    # completed the authenticated handshake at least once.
    # ------------------------------------------------------------------ #

    def get_peer_credential(self, address: str) -> Optional[dict]:
        """Return the stored credential for `address`, or None if never authenticated."""
        with self.engine.connect() as conn:
            row = conn.execute(
                select(peer_credentials_table).where(
                    peer_credentials_table.c.address == address
                )
            ).fetchone()
        return dict(row._mapping) if row is not None else None

    def upsert_peer_credential(
        self, address: str, node_id: str, public_key_hex: str, trusted: bool
    ) -> None:
        """
        Record or update a peer's authenticated credential.

        Overwrites any prior credential for this address unconditionally
        -- callers are responsible for deciding *whether* an overwrite is
        appropriate (see `blockchain.network.handshake`'s identity-change
        detection, which must run and be acted on before this is called
        for an address that already has a differing stored credential).
        """
        with self.engine.begin() as conn:
            row = conn.execute(
                select(peer_credentials_table).where(
                    peer_credentials_table.c.address == address
                )
            ).fetchone()
            values = {
                "node_id": node_id,
                "public_key_hex": public_key_hex,
                "trusted": int(trusted),
                "authenticated_at": time.time(),
            }
            if row is None:
                conn.execute(
                    peer_credentials_table.insert().values(address=address, **values)
                )
            else:
                conn.execute(
                    peer_credentials_table.update()
                    .where(peer_credentials_table.c.address == address)
                    .values(**values)
                )

    def is_peer_trusted(self, address: str) -> bool:
        """Return True if `address` has a stored, trusted credential."""
        credential = self.get_peer_credential(address)
        return bool(credential and credential["trusted"])
