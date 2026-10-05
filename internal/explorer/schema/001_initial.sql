CREATE TABLE IF NOT EXISTS blocks (
    id INTEGER PRIMARY KEY,
    hash TEXT NOT NULL UNIQUE,
    height INTEGER NOT NULL,
    previous_hash TEXT NOT NULL,
    timestamp REAL NOT NULL,
    nonce TEXT NOT NULL,
    difficulty INTEGER NOT NULL,
    merkle_root TEXT NOT NULL,
    chain_work TEXT NOT NULL,
    tx_count INTEGER NOT NULL CHECK (tx_count >= 0),
    is_canonical INTEGER NOT NULL CHECK (is_canonical IN (0, 1))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_blocks_canonical_height
ON blocks(height) WHERE is_canonical = 1;
CREATE INDEX IF NOT EXISTS idx_blocks_height ON blocks(height);
CREATE INDEX IF NOT EXISTS idx_blocks_previous_hash ON blocks(previous_hash);
CREATE INDEX IF NOT EXISTS idx_blocks_canonical_height_desc ON blocks(height DESC) WHERE is_canonical = 1;

CREATE TABLE IF NOT EXISTS transactions (
    id INTEGER PRIMARY KEY,
    block_hash TEXT NOT NULL REFERENCES blocks(hash) ON DELETE CASCADE,
    block_height INTEGER NOT NULL,
    tx_index INTEGER NOT NULL CHECK (tx_index >= 0),
    version INTEGER NOT NULL CHECK (version IN (0, 2)),
    network_id TEXT NOT NULL,
    nonce TEXT NOT NULL,
    sender TEXT NOT NULL,
    receiver TEXT NOT NULL,
    amount_base_units INTEGER NOT NULL CHECK (amount_base_units >= 0),
    timestamp REAL NOT NULL,
    sender_public_key TEXT NOT NULL,
    signature TEXT NOT NULL,
    tx_hash TEXT NOT NULL,
    tx_id TEXT NOT NULL,
    identity_hash TEXT NOT NULL,
    is_canonical INTEGER NOT NULL CHECK (is_canonical IN (0, 1)),
    UNIQUE(block_hash, tx_index),
    CHECK ((version = 0 AND length(tx_hash) > 0 AND length(tx_id) = 0)
        OR (version = 2 AND length(tx_id) > 0 AND length(tx_hash) = 0))
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_canonical_identity
ON transactions(identity_hash) WHERE is_canonical = 1;
CREATE INDEX IF NOT EXISTS idx_transactions_block_height_index ON transactions(block_height, tx_index);
CREATE INDEX IF NOT EXISTS idx_transactions_sender ON transactions(sender);
CREATE INDEX IF NOT EXISTS idx_transactions_receiver ON transactions(receiver);
CREATE INDEX IF NOT EXISTS idx_transactions_canonical_height ON transactions(block_height) WHERE is_canonical = 1;

CREATE TABLE IF NOT EXISTS addresses (
    address TEXT PRIMARY KEY,
    initial_balance INTEGER NOT NULL CHECK (initial_balance >= 0),
    received INTEGER NOT NULL CHECK (received >= 0),
    sent INTEGER NOT NULL CHECK (sent >= 0),
    balance INTEGER NOT NULL CHECK (balance >= 0),
    tx_count INTEGER NOT NULL CHECK (tx_count >= 0)
);

CREATE TABLE IF NOT EXISTS address_transactions (
    id INTEGER PRIMARY KEY,
    address TEXT NOT NULL REFERENCES addresses(address) ON DELETE CASCADE,
    block_hash TEXT NOT NULL REFERENCES blocks(hash) ON DELETE CASCADE,
    block_height INTEGER NOT NULL,
    tx_index INTEGER NOT NULL,
    tx_identity TEXT NOT NULL,
    direction TEXT NOT NULL CHECK (direction IN ('in', 'out')),
    amount_base_units INTEGER NOT NULL CHECK (amount_base_units >= 0),
    is_canonical INTEGER NOT NULL CHECK (is_canonical IN (0, 1)),
    UNIQUE(block_hash, tx_index, address, direction)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_address_transactions_canonical_identity
ON address_transactions(address, tx_identity, direction) WHERE is_canonical = 1;
CREATE INDEX IF NOT EXISTS idx_address_transactions_address_height
ON address_transactions(address, block_height, tx_index);
CREATE INDEX IF NOT EXISTS idx_address_transactions_canonical_address
ON address_transactions(address, block_height) WHERE is_canonical = 1;

CREATE TABLE IF NOT EXISTS network_snapshots (
    id INTEGER PRIMARY KEY,
    height INTEGER NOT NULL,
    tip_hash TEXT NOT NULL,
    observed_at INTEGER NOT NULL,
    peer_count INTEGER NOT NULL CHECK (peer_count >= 0),
    payload_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_network_snapshots_observed_at ON network_snapshots(observed_at DESC);

CREATE TABLE IF NOT EXISTS sync_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    status TEXT NOT NULL,
    indexed_height INTEGER NOT NULL,
    indexed_tip_hash TEXT NOT NULL,
    checkpoint_height INTEGER,
    checkpoint_hash TEXT,
    last_error TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sync_state_singleton ON sync_state(id);
