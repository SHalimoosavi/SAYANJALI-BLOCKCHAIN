package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/explorer/model"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/explorer/schema"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

type SyncState struct {
	Status           string
	IndexedHeight    int64
	IndexedTipHash   string
	CheckpointHeight *int64
	CheckpointHash   *string
	LastError        string
	UpdatedAt        int64
}

type Repository struct {
	writer *sql.DB
	reader *sql.DB
}

func New(ctx context.Context, path string) (*Repository, error) {
	if path == "" {
		return nil, errors.New("database path is empty")
	}

	dsn := sqliteDSN(path, false)
	writer, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite writer: %w", err)
	}

	writer.SetMaxOpenConns(1)
	writer.SetMaxIdleConns(1)
	writer.SetConnMaxLifetime(0)

	if err := writer.PingContext(ctx); err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("ping sqlite writer: %w", err)
	}

	if err := schema.Migrate(ctx, writer); err != nil {
		_ = writer.Close()
		return nil, err
	}

	reader, err := sql.Open("sqlite", sqliteDSN(path, true))
	if err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("open sqlite reader: %w", err)
	}

	reader.SetMaxOpenConns(4)
	reader.SetMaxIdleConns(4)
	reader.SetConnMaxLifetime(0)

	if err := reader.PingContext(ctx); err != nil {
		_ = reader.Close()
		_ = writer.Close()
		return nil, fmt.Errorf("ping sqlite reader: %w", err)
	}

	return &Repository{
		writer: writer,
		reader: reader,
	}, nil
}

func sqliteDSN(path string, readOnly bool) string {
	mode := ""
	if readOnly {
		mode = "&mode=ro"
	}

	return "file:" + path +
		"?_pragma=foreign_keys(1)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=synchronous(NORMAL)" +
		mode
}

func (r *Repository) Close() error {
	var errs []string

	if r.reader != nil {
		if err := r.reader.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if r.writer != nil {
		if err := r.writer.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

func (r *Repository) Writer() *sql.DB {
	return r.writer
}

func (r *Repository) Reader() *sql.DB {
	return r.reader
}

func (r *Repository) InsertBlock(ctx context.Context, b model.Block) error {
	if b.Hash == "" {
		return errors.New("block hash is empty")
	}
	if b.Height < 0 {
		return errors.New("block height is negative")
	}
	if b.ChainWork == "" {
		return errors.New("block chain_work is empty")
	}
	if b.Nonce > model.MaxSQLiteInteger {
		return fmt.Errorf("block nonce %d exceeds SQLite INTEGER range", b.Nonce)
	}
	if len(b.Transactions) > math.MaxInt64 {
		return errors.New("transaction count exceeds SQLite INTEGER range")
	}

	txCount, err := distinctTransactionCount(b.Transactions)
	if err != nil {
		return err
	}

	tx, err := r.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin insert block: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var existingID int64
	err = tx.QueryRowContext(
		ctx,
		`SELECT id FROM blocks WHERE hash = ?`,
		b.Hash,
	).Scan(&existingID)

	switch {
	case err == nil:
		var (
			height                               int64
			previousHash, nonce, merkleRoot      string
			chainWork                            string
			timestamp                            float64
			difficulty, existingCount, canonical int
		)

		if err := tx.QueryRowContext(ctx, `
			SELECT height, previous_hash, timestamp, nonce, difficulty,
			       merkle_root, chain_work, tx_count, is_canonical
			FROM blocks
			WHERE id = ?
		`, existingID).Scan(
			&height,
			&previousHash,
			&timestamp,
			&nonce,
			&difficulty,
			&merkleRoot,
			&chainWork,
			&existingCount,
			&canonical,
		); err != nil {
			return err
		}

		if height != b.Height ||
			previousHash != b.PreviousHash ||
			timestamp != b.Timestamp ||
			nonce != strconv.FormatUint(b.Nonce, 10) ||
			difficulty != b.Difficulty ||
			merkleRoot != b.MerkleRoot ||
			chainWork != b.ChainWork ||
			existingCount != distinctCountValue(distinctTransactionCountUnchecked(b.Transactions)) ||
			canonical != boolInt(b.IsCanonical) {
			return fmt.Errorf("duplicate block %s conflicts with existing block data", b.Hash)
		}

		for i, expected := range b.Transactions {
			var (
				version                    int
				networkID, storedNonce     string
				sender, receiver           string
				senderPublicKey, signature string
				txHash, txID, identity     string
				amount                     int64
				txTimestamp                float64
			)

			if err := tx.QueryRowContext(ctx, `
				SELECT version, network_id, nonce, sender, receiver,
				       amount_base_units, timestamp, sender_public_key,
				       signature, tx_hash, tx_id, identity_hash
				FROM transactions
				WHERE block_hash = ? AND tx_index = ?
			`, b.Hash, i).Scan(
				&version,
				&networkID,
				&storedNonce,
				&sender,
				&receiver,
				&amount,
				&txTimestamp,
				&senderPublicKey,
				&signature,
				&txHash,
				&txID,
				&identity,
			); err != nil {
				return fmt.Errorf("read existing duplicate transaction %d: %w", i, err)
			}

			expectedIdentity, err := expected.IdentityHash()
			if err != nil {
				return err
			}

			if version != int(expected.Version) ||
				networkID != expected.NetworkID ||
				storedNonce != strconv.FormatUint(expected.Nonce, 10) ||
				sender != expected.Sender ||
				receiver != expected.Receiver ||
				amount < 0 ||
				uint64(amount) != expected.AmountBaseUnits ||
				txTimestamp != expected.Timestamp ||
				senderPublicKey != expected.SenderPublicKey ||
				signature != expected.Signature ||
				txHash != expected.TxHash ||
				txID != expected.TxID ||
				identity != expectedIdentity {
				return fmt.Errorf(
					"duplicate block %s conflicts with transaction %d",
					b.Hash,
					i,
				)
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit idempotent block insert: %w", err)
		}

		return nil

	case !errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("check duplicate block: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO blocks(
			hash, height, previous_hash, timestamp, nonce, difficulty,
			merkle_root, chain_work, tx_count, is_canonical
		)
		VALUES(?,?,?,?,?,?,?,?,?,?)
	`,
		b.Hash,
		b.Height,
		b.PreviousHash,
		b.Timestamp,
		strconv.FormatUint(b.Nonce, 10),
		b.Difficulty,
		b.MerkleRoot,
		b.ChainWork,
		txCount,
		boolInt(b.IsCanonical),
	); err != nil {
		return fmt.Errorf("insert block %s: %w", b.Hash, err)
	}

	for i, t := range b.Transactions {
		identity, err := t.IdentityHash()
		if err != nil {
			return fmt.Errorf("transaction %d: %w", i, err)
		}

		if t.AmountBaseUnits > model.MaxSQLiteInteger {
			return fmt.Errorf(
				"transaction %d amount exceeds SQLite INTEGER range",
				i,
			)
		}

		if t.Nonce > model.MaxSQLiteInteger {
			return fmt.Errorf(
				"transaction %d nonce exceeds SQLite INTEGER range",
				i,
			)
		}

		canonical := b.IsCanonical

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO transactions(
				block_hash, block_height, tx_index, version, network_id,
				nonce, sender, receiver, amount_base_units, timestamp,
				sender_public_key, signature, tx_hash, tx_id,
				identity_hash, is_canonical
			)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		`,
			b.Hash,
			b.Height,
			i,
			t.Version,
			t.NetworkID,
			strconv.FormatUint(t.Nonce, 10),
			t.Sender,
			t.Receiver,
			t.AmountBaseUnits,
			t.Timestamp,
			t.SenderPublicKey,
			t.Signature,
			t.TxHash,
			t.TxID,
			identity,
			boolInt(canonical),
		); err != nil {
			return fmt.Errorf("insert transaction %d: %w", i, err)
		}

		if !isIssuanceSender(t.Sender) {
			if err := r.insertAddressTx(
				ctx,
				tx,
				b,
				i,
				identity,
				t.Sender,
				model.DirectionOut,
				t.AmountBaseUnits,
				canonical,
			); err != nil {
				return err
			}
		}

		if err := r.insertAddressTx(
			ctx,
			tx,
			b,
			i,
			identity,
			t.Receiver,
			model.DirectionIn,
			t.AmountBaseUnits,
			canonical,
		); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit block insert: %w", err)
	}

	return nil
}

func distinctTransactionCount(txs []model.Transaction) (int, error) {
	seen := make(map[string]struct{}, len(txs))

	for i, t := range txs {
		identity, err := t.IdentityHash()
		if err != nil {
			return 0, fmt.Errorf("transaction %d: %w", i, err)
		}

		if _, exists := seen[identity]; exists {
			return 0, fmt.Errorf(
				"duplicate transaction identity %q in block",
				identity,
			)
		}

		seen[identity] = struct{}{}
	}

	if len(seen) > math.MaxInt64 {
		return 0, errors.New("distinct transaction count exceeds SQLite INTEGER range")
	}

	return len(seen), nil
}

func distinctTransactionCountUnchecked(txs []model.Transaction) map[string]struct{} {
	seen := make(map[string]struct{}, len(txs))
	for _, t := range txs {
		identity, err := t.IdentityHash()
		if err != nil {
			continue
		}
		seen[identity] = struct{}{}
	}
	return seen
}

func distinctCountValue(seen map[string]struct{}) int {
	return len(seen)
}

func isIssuanceSender(sender string) bool {
	return sender == "" ||
		sender == protocol.CoinbaseSender ||
		sender == protocol.GenesisAllocationSender
}

func (r *Repository) insertAddressTx(
	ctx context.Context,
	tx *sql.Tx,
	b model.Block,
	index int,
	identity,
	address,
	direction string,
	amount uint64,
	canonical bool,
) error {
	if address == "" {
		return errors.New("address is empty")
	}

	if amount > model.MaxSQLiteInteger {
		return errors.New("address transaction amount exceeds SQLite INTEGER range")
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO addresses(
			address, initial_balance, received, sent, balance, tx_count
		)
		VALUES(?,0,0,0,0,0)
	`, address); err != nil {
		return fmt.Errorf("ensure address %s: %w", address, err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO address_transactions(
			address, block_hash, block_height, tx_index,
			tx_identity, direction, amount_base_units, is_canonical
		)
		VALUES(?,?,?,?,?,?,?,?)
	`,
		address,
		b.Hash,
		b.Height,
		index,
		identity,
		direction,
		amount,
		boolInt(canonical),
	); err != nil {
		return fmt.Errorf(
			"insert address transaction for %s: %w",
			address,
			err,
		)
	}

	return nil
}

func (r *Repository) SetCanonical(
	ctx context.Context,
	canonicalHashes []string,
) error {
	if len(canonicalHashes) == 0 {
		return errors.New("canonical hash set is empty")
	}

	tx, err := r.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin canonical update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	type canonicalBlock struct {
		hash         string
		height       int64
		previousHash string
	}

	blocks := make([]canonicalBlock, 0, len(canonicalHashes))
	seen := make(map[string]struct{}, len(canonicalHashes))

	for i, hash := range canonicalHashes {
		if hash == "" {
			return fmt.Errorf("canonical block at position %d has empty hash", i)
		}

		if _, exists := seen[hash]; exists {
			return fmt.Errorf(
				"duplicate canonical block hash %q",
				hash,
			)
		}
		seen[hash] = struct{}{}

		var b canonicalBlock
		if err := tx.QueryRowContext(ctx, `
			SELECT hash, height, previous_hash
			FROM blocks
			WHERE hash = ?
		`, hash).Scan(
			&b.hash,
			&b.height,
			&b.previousHash,
		); err != nil {
			return fmt.Errorf(
				"canonical block %s: %w",
				hash,
				err,
			)
		}

		blocks = append(blocks, b)
	}

	if blocks[0].height != 0 {
		return fmt.Errorf(
			"canonical chain must start at genesis height 0, got height %d",
			blocks[0].height,
		)
	}

	if blocks[0].previousHash != "" &&
		blocks[0].previousHash != protocol.GenesisPreviousHash {
		return fmt.Errorf(
			"genesis block %s has invalid previous hash %q",
			blocks[0].hash,
			blocks[0].previousHash,
		)
	}

	for i := 1; i < len(blocks); i++ {
		prev := blocks[i-1]
		current := blocks[i]

		if current.height != prev.height+1 {
			return fmt.Errorf(
				"canonical chain is non-contiguous between %s (height %d) and %s (height %d)",
				prev.hash,
				prev.height,
				current.hash,
				current.height,
			)
		}

		if current.previousHash != prev.hash {
			return fmt.Errorf(
				"canonical block %s previous_hash=%q does not match prior canonical block %s",
				current.hash,
				current.previousHash,
				prev.hash,
			)
		}
	}

	affected, err := affectedAddressesTx(
		ctx,
		tx,
		canonicalHashes,
	)
	if err != nil {
		return err
	}

	// Clear old canonical rows first so partial unique indexes can never
	// temporarily contain both sides of a reorg.
	if _, err := tx.ExecContext(ctx, `
		UPDATE address_transactions
		SET is_canonical = 0
		WHERE is_canonical = 1
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE transactions
		SET is_canonical = 0
		WHERE is_canonical = 1
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE blocks
		SET is_canonical = 0
		WHERE is_canonical = 1
	`); err != nil {
		return err
	}

	for _, b := range blocks {
		if _, err := tx.ExecContext(ctx, `
			UPDATE blocks
			SET is_canonical = 1
			WHERE hash = ?
		`, b.hash); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE transactions
			SET is_canonical = 1
			WHERE block_hash = ?
		`, b.hash); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE address_transactions
			SET is_canonical = 1
			WHERE block_hash = ?
		`, b.hash); err != nil {
			return err
		}
	}

	if err := recomputeAddressesTx(ctx, tx, affected); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit canonical update: %w", err)
	}

	return nil
}

func affectedAddressesTx(
	ctx context.Context,
	tx *sql.Tx,
	canonicalHashes []string,
) ([]string, error) {
	if len(canonicalHashes) == 0 {
		return nil, errors.New("canonical hash set is empty")
	}

	placeholders := strings.TrimRight(
		strings.Repeat("?,", len(canonicalHashes)),
		",",
	)

	args := make([]any, 0, len(canonicalHashes))
	for _, hash := range canonicalHashes {
		args = append(args, hash)
	}

	query := `
		SELECT DISTINCT address
		FROM address_transactions
		WHERE is_canonical = 1
		OR block_hash IN (` + placeholders + `)
		ORDER BY address
	`

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find affected addresses: %w", err)
	}
	defer rows.Close()

	var addresses []string
	for rows.Next() {
		var address string
		if err := rows.Scan(&address); err != nil {
			return nil, err
		}
		addresses = append(addresses, address)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return addresses, nil
}

func (r *Repository) RecomputeAddresses(
	ctx context.Context,
	affected []string,
) error {
	tx, err := r.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := recomputeAddressesTx(ctx, tx, affected); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit address recompute: %w", err)
	}

	return nil
}

func recomputeAddressesTx(
	ctx context.Context,
	tx *sql.Tx,
	affected []string,
) error {
	seen := make(map[string]struct{}, len(affected))

	for _, address := range affected {
		if address == "" {
			continue
		}

		if _, ok := seen[address]; ok {
			continue
		}
		seen[address] = struct{}{}

		var initialSQL int64
		if err := tx.QueryRowContext(ctx, `
			SELECT initial_balance
			FROM addresses
			WHERE address = ?
		`, address).Scan(&initialSQL); err != nil {
			return fmt.Errorf(
				"read initial balance for address %s: %w",
				address,
				err,
			)
		}

		if initialSQL < 0 {
			return fmt.Errorf(
				"negative initial balance for address %s",
				address,
			)
		}

		var received, sent uint64

		rows, err := tx.QueryContext(ctx, `
			SELECT direction, amount_base_units
			FROM address_transactions
			WHERE address = ?
			  AND is_canonical = 1
		`, address)
		if err != nil {
			return err
		}

		for rows.Next() {
			var (
				direction string
				amount    int64
			)

			if err := rows.Scan(&direction, &amount); err != nil {
				_ = rows.Close()
				return err
			}

			if amount < 0 {
				_ = rows.Close()
				return fmt.Errorf(
					"negative address transaction amount for %s",
					address,
				)
			}

			switch direction {
			case model.DirectionIn:
				received, err = model.CheckedAdd(
					received,
					uint64(amount),
				)
			case model.DirectionOut:
				sent, err = model.CheckedAdd(
					sent,
					uint64(amount),
				)
			default:
				_ = rows.Close()
				return fmt.Errorf(
					"invalid address transaction direction %q for %s",
					direction,
					address,
				)
			}

			if err != nil {
				_ = rows.Close()
				return fmt.Errorf(
					"aggregate overflow for %s: %w",
					address,
					err,
				)
			}
		}

		if err := rows.Close(); err != nil {
			return err
		}

		balance, err := model.CheckedBalance(
			uint64(initialSQL),
			received,
			sent,
		)
		if err != nil {
			return fmt.Errorf("address %s: %w", address, err)
		}

		if received > model.MaxSQLiteInteger ||
			sent > model.MaxSQLiteInteger ||
			balance > model.MaxSQLiteInteger {
			return fmt.Errorf(
				"aggregate for %s exceeds SQLite INTEGER range",
				address,
			)
		}

		var txCount int64
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(DISTINCT tx_identity)
			FROM address_transactions
			WHERE address = ?
			  AND is_canonical = 1
		`, address).Scan(&txCount); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE addresses
			SET received = ?,
			    sent = ?,
			    balance = ?,
			    tx_count = ?
			WHERE address = ?
		`,
			int64(received),
			int64(sent),
			int64(balance),
			txCount,
			address,
		); err != nil {
			return err
		}
	}

	return nil
}

func (r *Repository) SetInitialBalance(
	ctx context.Context,
	address string,
	amount uint64,
) error {
	if address == "" {
		return errors.New("address is empty")
	}

	if amount > model.MaxSQLiteInteger {
		return fmt.Errorf(
			"initial balance exceeds SQLite INTEGER range",
		)
	}

	tx, err := r.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO addresses(
			address, initial_balance, received, sent, balance, tx_count
		)
		VALUES(?, ?, 0, 0, 0, 0)
		ON CONFLICT(address) DO UPDATE
		SET initial_balance = excluded.initial_balance
	`,
		address,
		int64(amount),
	); err != nil {
		return err
	}

	if err := recomputeAddressesTx(
		ctx,
		tx,
		[]string{address},
	); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (r *Repository) ResetToCheckpoint(
	ctx context.Context,
	checkpointHeight int64,
	checkpointHash string,
) error {
	if checkpointHeight < 0 {
		return errors.New("checkpoint height is negative")
	}

	if checkpointHash == "" {
		return errors.New("checkpoint hash is empty")
	}

	tx, err := r.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	if err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM blocks
		WHERE height = ?
		  AND hash = ?
		  AND is_canonical = 1
	`,
		checkpointHeight,
		checkpointHash,
	).Scan(&exists); err != nil {
		return fmt.Errorf("checkpoint is not canonical: %w", err)
	}

	addressRows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT address
		FROM address_transactions
		WHERE is_canonical = 1
		ORDER BY address
	`)
	if err != nil {
		return err
	}

	var affected []string
	for addressRows.Next() {
		var address string
		if err := addressRows.Scan(&address); err != nil {
			_ = addressRows.Close()
			return err
		}
		affected = append(affected, address)
	}

	if err := addressRows.Close(); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM blocks
		WHERE height > ?
	`, checkpointHeight); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE blocks
		SET is_canonical = 1
		WHERE hash = ?
	`, checkpointHash); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE transactions
		SET is_canonical = 1
		WHERE block_hash = ?
	`, checkpointHash); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE address_transactions
		SET is_canonical = 1
		WHERE block_hash = ?
	`, checkpointHash); err != nil {
		return err
	}

	if err := recomputeAddressesTx(ctx, tx, affected); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sync_state(
			id, status, indexed_height, indexed_tip_hash,
			checkpoint_height, checkpoint_hash, last_error, updated_at
		)
		VALUES(
			1, 'checkpoint_reset', ?, ?, ?, ?, '', unixepoch()
		)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			indexed_height = excluded.indexed_height,
			indexed_tip_hash = excluded.indexed_tip_hash,
			checkpoint_height = excluded.checkpoint_height,
			checkpoint_hash = excluded.checkpoint_hash,
			updated_at = excluded.updated_at
	`,
		checkpointHeight,
		checkpointHash,
		checkpointHeight,
		checkpointHash,
	); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (r *Repository) GetBlocks(
	ctx context.Context,
	heightFrom,
	heightTo int64,
	limit,
	offset int,
) ([]model.Block, error) {
	if heightFrom < 0 {
		heightFrom = 0
	}

	if heightTo < heightFrom {
		return []model.Block{}, nil
	}

	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}

	if offset < 0 {
		offset = 0
	}

	rows, err := r.reader.QueryContext(ctx, `
		SELECT hash,height,previous_hash,timestamp,nonce,difficulty,
		       merkle_root,chain_work,is_canonical
		FROM blocks
		WHERE is_canonical = 1
		  AND height BETWEEN ? AND ?
		ORDER BY height ASC
		LIMIT ? OFFSET ?
	`,
		heightFrom,
		heightTo,
		limit,
		offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.Block, 0, limit)

	for rows.Next() {
		var (
			b         model.Block
			nonce     string
			canonical int
		)

		if err := rows.Scan(
			&b.Hash,
			&b.Height,
			&b.PreviousHash,
			&b.Timestamp,
			&nonce,
			&b.Difficulty,
			&b.MerkleRoot,
			&b.ChainWork,
			&canonical,
		); err != nil {
			return nil, err
		}

		n, err := strconv.ParseUint(nonce, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid stored nonce: %w", err)
		}

		b.Nonce = n
		b.IsCanonical = canonical == 1
		out = append(out, b)
	}

	return out, rows.Err()
}

func (r *Repository) GetSyncState(
	ctx context.Context,
) (SyncState, error) {
	var s SyncState
	var checkpointHeight sql.NullInt64
	var checkpointHash sql.NullString

	err := r.reader.QueryRowContext(ctx, `
		SELECT status,indexed_height,indexed_tip_hash,
		       checkpoint_height,checkpoint_hash,last_error,updated_at
		FROM sync_state
		WHERE id = 1
	`).Scan(
		&s.Status,
		&s.IndexedHeight,
		&s.IndexedTipHash,
		&checkpointHeight,
		&checkpointHash,
		&s.LastError,
		&s.UpdatedAt,
	)
	if err != nil {
		return s, err
	}

	if checkpointHeight.Valid {
		s.CheckpointHeight = &checkpointHeight.Int64
	}

	if checkpointHash.Valid {
		s.CheckpointHash = &checkpointHash.String
	}

	return s, nil
}

func (r *Repository) SetSyncState(
	ctx context.Context,
	s SyncState,
) error {
	_, err := r.writer.ExecContext(ctx, `
		INSERT INTO sync_state(
			id,status,indexed_height,indexed_tip_hash,
			checkpoint_height,checkpoint_hash,last_error,updated_at
		)
		VALUES(1,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			status=excluded.status,
			indexed_height=excluded.indexed_height,
			indexed_tip_hash=excluded.indexed_tip_hash,
			checkpoint_height=excluded.checkpoint_height,
			checkpoint_hash=excluded.checkpoint_hash,
			last_error=excluded.last_error,
			updated_at=excluded.updated_at
	`,
		s.Status,
		s.IndexedHeight,
		s.IndexedTipHash,
		s.CheckpointHeight,
		s.CheckpointHash,
		s.LastError,
		s.UpdatedAt,
	)

	return err
}

func (r *Repository) CheckInvariants(ctx context.Context) error {
	var bad int

	// Genesis has no parent in the explorer table. Every other canonical
	// block must have an existing canonical parent.
	if err := r.reader.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM blocks b
		LEFT JOIN blocks p ON b.previous_hash = p.hash
		WHERE b.is_canonical = 1
		  AND b.height > 0
		  AND (
			  p.hash IS NULL
			  OR p.is_canonical = 0
		  )
	`).Scan(&bad); err != nil {
		return err
	}

	if bad != 0 {
		return fmt.Errorf(
			"canonical parent invariant violated: %d rows",
			bad,
		)
	}

	if err := r.reader.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM blocks
		WHERE is_canonical = 1
		  AND height = 0
		  AND previous_hash NOT IN ('', ?)
	`, protocol.GenesisPreviousHash).Scan(&bad); err != nil {
		return err
	}

	if bad != 0 {
		return fmt.Errorf(
			"genesis previous-hash invariant violated: %d rows",
			bad,
		)
	}

	if err := r.checkAggregateInvariants(ctx); err != nil {
		return err
	}

	if err := r.reader.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM blocks
		WHERE tx_count != (
			SELECT COUNT(DISTINCT t.identity_hash)
			FROM transactions t
			WHERE t.block_hash = blocks.hash
		)
	`).Scan(&bad); err != nil {
		return err
	}

	if bad != 0 {
		return fmt.Errorf(
			"block tx_count invariant violated: %d blocks",
			bad,
		)
	}

	if err := r.reader.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM address_transactions
		WHERE direction NOT IN ('in','out')
	`).Scan(&bad); err != nil {
		return err
	}

	if bad != 0 {
		return fmt.Errorf(
			"unknown address transaction direction: %d rows",
			bad,
		)
	}

	if err := r.reader.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM transactions
		WHERE is_canonical=1
		  AND (
			  (version=0 AND (tx_hash='' OR tx_id!=''))
			  OR
			  (version=2 AND (tx_id='' OR tx_hash!=''))
		  )
	`).Scan(&bad); err != nil {
		return err
	}

	if bad != 0 {
		return fmt.Errorf(
			"transaction version identity invariant violated: %d rows",
			bad,
		)
	}

	return nil
}

func (r *Repository) checkAggregateInvariants(
	ctx context.Context,
) error {
	rows, err := r.reader.QueryContext(ctx, `
		SELECT address,initial_balance,received,sent,balance,tx_count
		FROM addresses
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			address                 string
			initial, received, sent int64
			balance, txCount        int64
		)

		if err := rows.Scan(
			&address,
			&initial,
			&received,
			&sent,
			&balance,
			&txCount,
		); err != nil {
			return err
		}

		if initial < 0 ||
			received < 0 ||
			sent < 0 ||
			balance < 0 ||
			txCount < 0 {
			return fmt.Errorf(
				"negative aggregate for %s",
				address,
			)
		}

		var rec, snd uint64

		r2, err := r.reader.QueryContext(ctx, `
			SELECT direction,amount_base_units
			FROM address_transactions
			WHERE address = ?
			  AND is_canonical = 1
		`, address)
		if err != nil {
			return err
		}

		for r2.Next() {
			var (
				direction string
				amount    int64
			)

			if err := r2.Scan(&direction, &amount); err != nil {
				_ = r2.Close()
				return err
			}

			if amount < 0 {
				_ = r2.Close()
				return fmt.Errorf(
					"negative address transaction amount for %s",
					address,
				)
			}

			switch direction {
			case model.DirectionIn:
				rec, err = model.CheckedAdd(rec, uint64(amount))
			case model.DirectionOut:
				snd, err = model.CheckedAdd(snd, uint64(amount))
			default:
				_ = r2.Close()
				return fmt.Errorf(
					"unknown address transaction direction %q for %s",
					direction,
					address,
				)
			}

			if err != nil {
				_ = r2.Close()
				return fmt.Errorf(
					"aggregate overflow for %s: %w",
					address,
					err,
				)
			}
		}

		if err := r2.Close(); err != nil {
			return err
		}

		expected, err := model.CheckedBalance(
			uint64(initial),
			rec,
			snd,
		)
		if err != nil {
			return fmt.Errorf(
				"aggregate invariant for %s: %w",
				address,
				err,
			)
		}

		if expected != uint64(balance) ||
			rec != uint64(received) ||
			snd != uint64(sent) {
			return fmt.Errorf(
				"aggregate mismatch for %s: stored initial=%d received=%d sent=%d balance=%d, expected received=%d sent=%d balance=%d",
				address,
				initial,
				received,
				sent,
				balance,
				rec,
				snd,
				expected,
			)
		}

		var expectedTx int64
		if err := r.reader.QueryRowContext(ctx, `
			SELECT COUNT(DISTINCT tx_identity)
			FROM address_transactions
			WHERE address = ?
			  AND is_canonical = 1
		`, address).Scan(&expectedTx); err != nil {
			return err
		}

		if expectedTx != txCount {
			return fmt.Errorf(
				"tx_count mismatch for %s: stored=%d expected=%d",
				address,
				txCount,
				expectedTx,
			)
		}
	}

	return rows.Err()
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
