package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFTSTriggersStayInSyncWithMessages(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "letterhead.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	_, err = db.Exec(
		`INSERT INTO messages (
			gmail_id, thread_id, history_id, internal_date, received_at, subject, snippet,
			from_addr, from_name, plain_body, html_body, attachment_metadata_json,
			raw_size_bytes, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"msg_1", "thread_1", 1, 1742280000000, 1742280000, "Quarterly update",
		"Latest numbers attached.", "sender@example.com", "A. Sender",
		"Quarterly numbers attached.", "", "[]", 1024, 1742280000, 1742280000,
	)
	if err != nil {
		t.Fatalf("INSERT error = %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM messages_fts WHERE messages_fts MATCH 'quarterly'`).Scan(&count); err != nil {
		t.Fatalf("fts query error = %v", err)
	}

	if count != 1 {
		t.Fatalf("fts match count = %d, want 1", count)
	}

	_, err = db.Exec(
		`UPDATE messages
		 SET subject = ?, plain_body = ?, snippet = ?, from_name = ?, from_addr = ?
		 WHERE gmail_id = ?`,
		"Budget review",
		"Budget notes attached.",
		"Budget notes attached.",
		"Finance Team",
		"finance@example.com",
		"msg_1",
	)
	if err != nil {
		t.Fatalf("UPDATE error = %v", err)
	}

	if err := db.QueryRow(`SELECT COUNT(*) FROM messages_fts WHERE messages_fts MATCH 'quarterly'`).Scan(&count); err != nil {
		t.Fatalf("fts query error = %v", err)
	}

	if count != 0 {
		t.Fatalf("fts stale match count = %d, want 0", count)
	}

	if err := db.QueryRow(`SELECT COUNT(*) FROM messages_fts WHERE messages_fts MATCH 'budget'`).Scan(&count); err != nil {
		t.Fatalf("fts query error = %v", err)
	}

	if count != 1 {
		t.Fatalf("fts updated match count = %d, want 1", count)
	}

	_, err = db.Exec(`DELETE FROM messages WHERE gmail_id = ?`, "msg_1")
	if err != nil {
		t.Fatalf("DELETE error = %v", err)
	}

	if err := db.QueryRow(`SELECT COUNT(*) FROM messages_fts WHERE messages_fts MATCH 'budget'`).Scan(&count); err != nil {
		t.Fatalf("fts query error = %v", err)
	}

	if count != 0 {
		t.Fatalf("fts delete match count = %d, want 0", count)
	}
}

func TestRebuildFTS(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "letterhead.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	_, err = db.Exec(
		`INSERT INTO messages (
			gmail_id, thread_id, history_id, internal_date, received_at, subject, snippet,
			from_addr, from_name, plain_body, html_body, attachment_metadata_json,
			raw_size_bytes, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"msg_1", "thread_1", 1, 1742280000000, 1742280000, "Quarterly update",
		"Latest numbers attached.", "sender@example.com", "A. Sender",
		"Quarterly numbers attached.", "", "[]", 1024, 1742280000, 1742280000,
	)
	if err != nil {
		t.Fatalf("INSERT error = %v", err)
	}

	if _, err := db.Exec(`DELETE FROM messages_fts`); err != nil {
		t.Fatalf("DELETE FROM messages_fts error = %v", err)
	}

	if err := RebuildFTS(context.Background(), db); err != nil {
		t.Fatalf("RebuildFTS() error = %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM messages_fts WHERE messages_fts MATCH 'quarterly'`).Scan(&count); err != nil {
		t.Fatalf("fts query error = %v", err)
	}

	if count != 1 {
		t.Fatalf("fts rebuilt match count = %d, want 1", count)
	}
}
