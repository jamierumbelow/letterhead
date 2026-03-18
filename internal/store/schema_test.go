package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpenAppliesInitialSchema(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "letterhead.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	expectedTables := []string{
		"messages",
		"message_labels",
		"message_recipients",
		"sync_state",
		"sync_runs",
		"schema_migrations",
	}

	for _, tableName := range expectedTables {
		if !tableExists(t, db, tableName) {
			t.Fatalf("table %q does not exist", tableName)
		}
	}

	journalMode := pragmaString(t, db, "journal_mode")
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "letterhead.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	db.Close()

	db, err = Open(dbPath)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("QueryRow() error = %v", err)
	}

	if count != 1 {
		t.Fatalf("migration count = %d, want 1", count)
	}
}

func TestMessagesTableHasExpectedColumns(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "letterhead.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	expectedColumns := []string{
		"gmail_id",
		"thread_id",
		"history_id",
		"internal_date",
		"received_at",
		"attachment_metadata_json",
		"raw_size_bytes",
	}

	for _, columnName := range expectedColumns {
		if !columnExists(t, db, "messages", columnName) {
			t.Fatalf("column %q does not exist on messages", columnName)
		}
	}
}

func tableExists(t *testing.T, db *sql.DB, tableName string) bool {
	t.Helper()

	var name string
	if err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
		tableName,
	).Scan(&name); err != nil {
		return false
	}

	return name == tableName
}

func columnExists(t *testing.T, db *sql.DB, tableName, columnName string) bool {
	t.Helper()

	rows, err := db.Query(`PRAGMA table_info(` + tableName + `)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(%q) error = %v", tableName, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal any
			pk         int
		)

		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			t.Fatalf("rows.Scan() error = %v", err)
		}

		if name == columnName {
			return true
		}
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err() error = %v", err)
	}

	return false
}

func pragmaString(t *testing.T, db *sql.DB, name string) string {
	t.Helper()

	var value string
	if err := db.QueryRow(`PRAGMA ` + name).Scan(&value); err != nil {
		t.Fatalf("PRAGMA %s error = %v", name, err)
	}

	return value
}
