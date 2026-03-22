package store

import (
	"context"
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
		"messages_fts",
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

	if count != len(migrations) {
		t.Fatalf("migration count = %d, want %d", count, len(migrations))
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

func TestMigration004OnEmptyDatabase(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "letterhead.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	// Verify the messages table has account_id column.
	if !columnExists(t, db, "messages", "account_id") {
		t.Fatal("messages table missing account_id column after migration 004")
	}

	// Verify composite PK by inserting same gmail_id with different account_ids.
	now := int64(1700000000)
	for _, acct := range []string{"alice@example.com", "bob@example.com"} {
		_, err := db.Exec(`INSERT INTO messages (account_id, gmail_id, thread_id, history_id, internal_date, received_at,
			subject, snippet, from_addr, from_name, plain_body, html_body, attachment_metadata_json,
			raw_size_bytes, created_at, updated_at) VALUES (?, 'msg1', 'thr1', 1, 100, 100, '', '', '', '', '', '', '[]', 0, ?, ?)`,
			acct, now, now)
		if err != nil {
			t.Fatalf("insert for account %q error = %v", acct, err)
		}
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM messages WHERE gmail_id = 'msg1'`).Scan(&count); err != nil {
		t.Fatalf("count query error = %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 rows with same gmail_id, got %d", count)
	}
}

func TestMigration004BackfillsAccountID(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "letterhead.db")

	// Open a raw database and apply only migrations 1-3.
	db, err := sql.Open(sqliteDriver, dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	// Enable WAL and foreign keys.
	for _, pragma := range []string{"PRAGMA journal_mode = WAL", "PRAGMA foreign_keys = ON"} {
		if _, err := db.Exec(pragma); err != nil {
			t.Fatalf("pragma error = %v", err)
		}
	}

	ctx := context.Background()

	// Create migration table.
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at INTEGER NOT NULL
	)`); err != nil {
		t.Fatalf("create schema_migrations error = %v", err)
	}

	// Apply migrations 1-3 manually.
	for _, m := range migrations[:3] {
		data, err := migrationFiles.ReadFile(m.Path)
		if err != nil {
			t.Fatalf("read migration %d error = %v", m.Version, err)
		}
		if _, err := db.ExecContext(ctx, string(data)); err != nil {
			t.Fatalf("apply migration %d error = %v", m.Version, err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, 0)`, m.Version); err != nil {
			t.Fatalf("record migration %d error = %v", m.Version, err)
		}
	}

	// Insert pre-migration data: sync_state row and some messages.
	now := int64(1700000000)
	if _, err := db.ExecContext(ctx, `INSERT INTO sync_state (account_id, history_id, bootstrap_complete, messages_synced) VALUES ('user@test.com', 100, 0, 5)`); err != nil {
		t.Fatalf("insert sync_state error = %v", err)
	}
	for _, gmailID := range []string{"msg_a", "msg_b", "msg_c"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO messages (gmail_id, thread_id, history_id, internal_date, received_at,
			subject, snippet, from_addr, from_name, plain_body, html_body, attachment_metadata_json,
			raw_size_bytes, created_at, updated_at) VALUES (?, 'thr1', 1, 100, 100, 'subj', '', '', '', '', '', '[]', 0, ?, ?)`,
			gmailID, now, now); err != nil {
			t.Fatalf("insert message %s error = %v", gmailID, err)
		}
	}

	// Now apply migration 4.
	m004 := migrations[3]
	data, err := migrationFiles.ReadFile(m004.Path)
	if err != nil {
		t.Fatalf("read migration 004 error = %v", err)
	}
	if _, err := db.ExecContext(ctx, string(data)); err != nil {
		t.Fatalf("apply migration 004 error = %v", err)
	}

	// Verify all messages now have account_id = 'user@test.com'.
	rows, err := db.QueryContext(ctx, `SELECT account_id, gmail_id FROM messages ORDER BY gmail_id`)
	if err != nil {
		t.Fatalf("select messages error = %v", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var accountID, gmailID string
		if err := rows.Scan(&accountID, &gmailID); err != nil {
			t.Fatalf("scan error = %v", err)
		}
		if accountID != "user@test.com" {
			t.Errorf("message %s account_id = %q, want %q", gmailID, accountID, "user@test.com")
		}
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err() = %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 messages after migration, got %d", count)
	}
}

func TestMessagesTableHasAccountIDColumn(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "letterhead.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	for _, table := range []string{"messages", "message_labels", "message_recipients"} {
		if !columnExists(t, db, table, "account_id") {
			t.Errorf("table %q missing account_id column", table)
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
