package database

import (
	"context"
	"database/sql"
	"fmt"
)

// migrations run in order; version = index+1. Append only, never edit or reorder:
// databases created before versioning already have the tasks table, so every
// step must be safe to run against it.
var migrations = []func(ctx context.Context, tx *sql.Tx, postgres bool) error{
	// 1: base table (no-op on databases created by the pre-versioning code).
	func(ctx context.Context, tx *sql.Tx, postgres bool) error {
		id := "INTEGER PRIMARY KEY AUTOINCREMENT"
		if postgres {
			id = "SERIAL PRIMARY KEY"
		}
		_, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS tasks (
			id `+id+`,
			name TEXT NOT NULL,
			service TEXT NOT NULL,
			time TEXT NOT NULL,
			duration TEXT,
			days_of_week TEXT NOT NULL,
			is_recurring BOOLEAN NOT NULL,
			description TEXT,
			hosts TEXT
		)`)
		return err
	},
	// 2: duration, missing from the oldest databases.
	func(ctx context.Context, tx *sql.Tx, postgres bool) error {
		return addColumnIfMissing(ctx, tx, postgres, "duration", "TEXT")
	},
	// 3: date of one-off tasks.
	func(ctx context.Context, tx *sql.Tx, postgres bool) error {
		return addColumnIfMissing(ctx, tx, postgres, "date", "TEXT")
	},
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)"); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	var current int
	if err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	for i := current; i < len(migrations); i++ {
		version := i + 1
		if err := s.applyMigration(ctx, version, migrations[i]); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
	}
	return nil
}

func (s *Store) applyMigration(ctx context.Context, version int, m func(context.Context, *sql.Tx, bool) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }() // no-op after Commit
	if err := m(ctx, tx, s.postgres); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
		return err
	}
	return tx.Commit()
}

func addColumnIfMissing(ctx context.Context, tx *sql.Tx, postgres bool, column, typ string) error {
	var n int
	query := "SELECT COUNT(*) FROM pragma_table_info('tasks') WHERE name = $1"
	if postgres {
		query = "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'tasks' AND column_name = $1"
	}
	if err := tx.QueryRowContext(ctx, query, column).Scan(&n); err != nil {
		return fmt.Errorf("check column %s: %w", column, err)
	}
	if n > 0 {
		return nil
	}
	// column and typ are compile-time constants, never user input.
	_, err := tx.ExecContext(ctx, "ALTER TABLE tasks ADD COLUMN "+column+" "+typ)
	return err
}
