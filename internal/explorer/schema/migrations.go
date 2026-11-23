package schema

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
)

//go:embed 001_initial.sql
var migration001 embed.FS

const latestVersion = 1

func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
        version INTEGER PRIMARY KEY,
        applied_at INTEGER NOT NULL
    )`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var applied int
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&applied)
	if err != nil {
		return fmt.Errorf("read migration version: %w", err)
	}
	if applied > latestVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", applied, latestVersion)
	}
	if applied < 1 {
		sqlBytes, err := migration001.ReadFile("001_initial.sql")
		if err != nil {
			return fmt.Errorf("read migration 001: %w", err)
		}
		if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration 001: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(1, unixepoch())`); err != nil {
			return fmt.Errorf("record migration 001: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}
