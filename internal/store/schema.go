package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const sqliteDriver = "sqlite3"

const databaseFileName = "letterhead.db"

var (
	//go:embed migrations/*.sql
	migrationFiles embed.FS

	ErrEmptyDatabasePath = errors.New("database path is required")
)

type Migration struct {
	Version int
	Path    string
}

var migrations = []Migration{
	{Version: 1, Path: "migrations/001_initial.sql"},
}

func Open(path string) (*sql.DB, error) {
	if path == "" {
		return nil, ErrEmptyDatabasePath
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}

	db, err := sql.Open(sqliteDriver, path)
	if err != nil {
		return nil, err
	}

	if err := configure(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := ApplyMigrations(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func DatabasePath(archiveRoot string) string {
	return filepath.Join(archiveRoot, databaseFileName)
}

func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	if err := ensureMigrationTable(ctx, db); err != nil {
		return err
	}

	applied, err := appliedMigrationVersions(ctx, db)
	if err != nil {
		return err
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	for _, migration := range migrations {
		if applied[migration.Version] {
			continue
		}

		statement, err := migrationFiles.ReadFile(migration.Path)
		if err != nil {
			return err
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, string(statement)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", migration.Version, err)
		}

		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			migration.Version,
			time.Now().UTC().Unix(),
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", migration.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func configure(db *sql.DB) error {
	pragmas := []string{
		`PRAGMA journal_mode = WAL;`,
		`PRAGMA foreign_keys = ON;`,
		`PRAGMA busy_timeout = 5000;`,
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return err
		}
	}

	return nil
}

func ensureMigrationTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(
		ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at INTEGER NOT NULL
		);`,
	)

	return err
}

func appliedMigrationVersions(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}

		applied[version] = true
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return applied, nil
}
