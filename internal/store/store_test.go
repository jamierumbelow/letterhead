package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/jamierumbelow/letterhead/pkg/types"
)

func openTestDB(t testing.TB) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "letterhead.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func testMessage() *types.Message {
	return &types.Message{
		GmailID:      "msg_001",
		ThreadID:     "thread_001",
		HistoryID:    42,
		InternalDate: 1742280000000,
		ReceivedAt:   time.Unix(1742280000, 0).UTC(),
		Subject:      "Quarterly update",
		Snippet:      "Latest numbers attached.",
		From:         types.Address{Email: "alice@example.com", Name: "Alice"},
		To:           []types.Address{{Email: "bob@example.com", Name: "Bob"}},
		CC:           []types.Address{{Email: "carol@example.com", Name: "Carol"}},
		Labels:       []string{"INBOX", "IMPORTANT"},
		PlainBody:    "Here are the quarterly numbers.",
		HTMLBody:     "<p>Here are the quarterly numbers.</p>",
		Attachments: []types.AttachmentMeta{
			{Filename: "report.pdf", MIMEType: "application/pdf", SizeBytes: 2048, PartID: "1"},
		},
	}
}

func TestUpsertAndGetMessage(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	msg := testMessage()
	if err := s.UpsertMessage(ctx, msg); err != nil {
		t.Fatalf("UpsertMessage() error = %v", err)
	}

	got, err := s.GetMessage(ctx, "", "msg_001")
	if err != nil {
		t.Fatalf("GetMessage() error = %v", err)
	}

	if got.GmailID != msg.GmailID {
		t.Errorf("GmailID = %q, want %q", got.GmailID, msg.GmailID)
	}
	if got.ThreadID != msg.ThreadID {
		t.Errorf("ThreadID = %q, want %q", got.ThreadID, msg.ThreadID)
	}
	if got.HistoryID != msg.HistoryID {
		t.Errorf("HistoryID = %d, want %d", got.HistoryID, msg.HistoryID)
	}
	if got.Subject != msg.Subject {
		t.Errorf("Subject = %q, want %q", got.Subject, msg.Subject)
	}
	if got.From.Email != msg.From.Email {
		t.Errorf("From.Email = %q, want %q", got.From.Email, msg.From.Email)
	}
	if got.From.Name != msg.From.Name {
		t.Errorf("From.Name = %q, want %q", got.From.Name, msg.From.Name)
	}
	if got.PlainBody != msg.PlainBody {
		t.Errorf("PlainBody = %q, want %q", got.PlainBody, msg.PlainBody)
	}
	if got.HTMLBody != msg.HTMLBody {
		t.Errorf("HTMLBody = %q, want %q", got.HTMLBody, msg.HTMLBody)
	}

	// Labels
	if len(got.Labels) != 2 {
		t.Fatalf("Labels count = %d, want 2", len(got.Labels))
	}
	if got.Labels[0] != "IMPORTANT" || got.Labels[1] != "INBOX" {
		t.Errorf("Labels = %v, want [IMPORTANT INBOX]", got.Labels)
	}

	// Recipients
	if len(got.To) != 1 || got.To[0].Email != "bob@example.com" {
		t.Errorf("To = %v, want [{bob@example.com Bob}]", got.To)
	}
	if len(got.CC) != 1 || got.CC[0].Email != "carol@example.com" {
		t.Errorf("CC = %v, want [{carol@example.com Carol}]", got.CC)
	}

	// Attachments
	if len(got.Attachments) != 1 || got.Attachments[0].Filename != "report.pdf" {
		t.Errorf("Attachments = %v, want [{report.pdf ...}]", got.Attachments)
	}
}

func TestUpsertMessageUpdatesExisting(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	msg := testMessage()
	if err := s.UpsertMessage(ctx, msg); err != nil {
		t.Fatalf("first UpsertMessage() error = %v", err)
	}

	msg.Subject = "Updated subject"
	msg.Labels = []string{"STARRED"}
	msg.To = []types.Address{{Email: "dave@example.com", Name: "Dave"}}
	msg.CC = nil

	if err := s.UpsertMessage(ctx, msg); err != nil {
		t.Fatalf("second UpsertMessage() error = %v", err)
	}

	got, err := s.GetMessage(ctx, "", "msg_001")
	if err != nil {
		t.Fatalf("GetMessage() error = %v", err)
	}

	if got.Subject != "Updated subject" {
		t.Errorf("Subject = %q, want %q", got.Subject, "Updated subject")
	}
	if len(got.Labels) != 1 || got.Labels[0] != "STARRED" {
		t.Errorf("Labels = %v, want [STARRED]", got.Labels)
	}
	if len(got.To) != 1 || got.To[0].Email != "dave@example.com" {
		t.Errorf("To = %v, want [{dave@example.com Dave}]", got.To)
	}
	if len(got.CC) != 0 {
		t.Errorf("CC = %v, want []", got.CC)
	}
}

func TestGetMessageNotFound(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	_, err := s.GetMessage(ctx, "", "nonexistent")
	if err != sql.ErrNoRows {
		t.Fatalf("GetMessage() error = %v, want sql.ErrNoRows", err)
	}
}

func TestMessageExists(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	exists, err := s.MessageExists(ctx, "", "msg_001")
	if err != nil {
		t.Fatalf("MessageExists() error = %v", err)
	}
	if exists {
		t.Fatalf("MessageExists() = true before insert")
	}

	if err := s.UpsertMessage(ctx, testMessage()); err != nil {
		t.Fatalf("UpsertMessage() error = %v", err)
	}

	exists, err = s.MessageExists(ctx, "", "msg_001")
	if err != nil {
		t.Fatalf("MessageExists() error = %v", err)
	}
	if !exists {
		t.Fatalf("MessageExists() = false after insert")
	}
}

func TestListMessageIDsInThread(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	// Insert two messages in the same thread with different dates
	msg1 := testMessage()
	msg1.GmailID = "msg_a"
	msg1.InternalDate = 200

	msg2 := testMessage()
	msg2.GmailID = "msg_b"
	msg2.InternalDate = 100 // earlier

	for _, m := range []*types.Message{msg1, msg2} {
		if err := s.UpsertMessage(ctx, m); err != nil {
			t.Fatalf("UpsertMessage(%s) error = %v", m.GmailID, err)
		}
	}

	ids, err := s.ListMessageIDsInThread(ctx, "", "thread_001")
	if err != nil {
		t.Fatalf("ListMessageIDsInThread() error = %v", err)
	}

	if len(ids) != 2 {
		t.Fatalf("len(ids) = %d, want 2", len(ids))
	}
	// Should be ordered by internal_date ASC
	if ids[0] != "msg_b" || ids[1] != "msg_a" {
		t.Errorf("ids = %v, want [msg_b msg_a]", ids)
	}
}

func TestListMessageIDsInThreadEmpty(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	ids, err := s.ListMessageIDsInThread(ctx, "", "nonexistent")
	if err != nil {
		t.Fatalf("ListMessageIDsInThread() error = %v", err)
	}
	if ids != nil {
		t.Errorf("ids = %v, want nil", ids)
	}
}

func TestCountMessagesAndThreads(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	count, err := s.CountMessages(ctx, "")
	if err != nil {
		t.Fatalf("CountMessages() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("CountMessages() = %d, want 0", count)
	}

	threads, err := s.CountThreads(ctx, "")
	if err != nil {
		t.Fatalf("CountThreads() error = %v", err)
	}
	if threads != 0 {
		t.Fatalf("CountThreads() = %d, want 0", threads)
	}

	msg1 := testMessage()
	msg2 := testMessage()
	msg2.GmailID = "msg_002"
	msg2.ThreadID = "thread_002"

	msg3 := testMessage()
	msg3.GmailID = "msg_003"
	// same thread as msg1

	for _, m := range []*types.Message{msg1, msg2, msg3} {
		if err := s.UpsertMessage(ctx, m); err != nil {
			t.Fatalf("UpsertMessage(%s) error = %v", m.GmailID, err)
		}
	}

	count, err = s.CountMessages(ctx, "")
	if err != nil {
		t.Fatalf("CountMessages() error = %v", err)
	}
	if count != 3 {
		t.Errorf("CountMessages() = %d, want 3", count)
	}

	threads, err = s.CountThreads(ctx, "")
	if err != nil {
		t.Fatalf("CountThreads() error = %v", err)
	}
	if threads != 2 {
		t.Errorf("CountThreads() = %d, want 2", threads)
	}
}

func TestSyncState(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	// No state yet
	_, err := s.GetSyncState(ctx, "user@example.com")
	if err != sql.ErrNoRows {
		t.Fatalf("GetSyncState() error = %v, want sql.ErrNoRows", err)
	}

	// Set initial state
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	st := &SyncState{
		AccountID:         "user@example.com",
		HistoryID:         1000,
		BootstrapComplete: false,
		MessagesSynced:    50,
		LastSyncAt:        &now,
	}
	if err := s.SetSyncState(ctx, st); err != nil {
		t.Fatalf("SetSyncState() error = %v", err)
	}

	got, err := s.GetSyncState(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("GetSyncState() error = %v", err)
	}
	if got.HistoryID != 1000 {
		t.Errorf("HistoryID = %d, want 1000", got.HistoryID)
	}
	if got.BootstrapComplete {
		t.Errorf("BootstrapComplete = true, want false")
	}
	if got.MessagesSynced != 50 {
		t.Errorf("MessagesSynced = %d, want 50", got.MessagesSynced)
	}
	if got.LastSyncAt == nil || !got.LastSyncAt.Equal(now) {
		t.Errorf("LastSyncAt = %v, want %v", got.LastSyncAt, now)
	}

	// Update
	st.HistoryID = 2000
	st.BootstrapComplete = true
	st.MessagesSynced = 100
	if err := s.SetSyncState(ctx, st); err != nil {
		t.Fatalf("SetSyncState() update error = %v", err)
	}

	got, err = s.GetSyncState(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("GetSyncState() error = %v", err)
	}
	if got.HistoryID != 2000 {
		t.Errorf("HistoryID = %d, want 2000", got.HistoryID)
	}
	if !got.BootstrapComplete {
		t.Errorf("BootstrapComplete = false, want true")
	}
}

func TestSyncStateNilLastSync(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	st := &SyncState{
		AccountID: "user@example.com",
		HistoryID: 1,
	}
	if err := s.SetSyncState(ctx, st); err != nil {
		t.Fatalf("SetSyncState() error = %v", err)
	}

	got, err := s.GetSyncState(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("GetSyncState() error = %v", err)
	}
	if got.LastSyncAt != nil {
		t.Errorf("LastSyncAt = %v, want nil", got.LastSyncAt)
	}
}

func TestSyncRun(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	started := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	run := &SyncRun{
		AccountID: "user@example.com",
		StartedAt: started,
		Mode:      "inbox",
		Status:    "running",
	}

	id, err := s.StartSyncRun(ctx, run)
	if err != nil {
		t.Fatalf("StartSyncRun() error = %v", err)
	}
	if id < 1 {
		t.Fatalf("StartSyncRun() id = %d, want >= 1", id)
	}

	if err := s.FinishSyncRun(ctx, id, "ok", 42, ""); err != nil {
		t.Fatalf("FinishSyncRun() error = %v", err)
	}

	// Verify via raw query
	var status string
	var count int
	if err := db.QueryRowContext(ctx, `SELECT status, messages_synced FROM sync_runs WHERE id = ?`, id).Scan(&status, &count); err != nil {
		t.Fatalf("query sync_runs error = %v", err)
	}
	if status != "ok" || count != 42 {
		t.Errorf("sync run = (%q, %d), want (ok, 42)", status, count)
	}
}

func TestSyncRunWithError(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	run := &SyncRun{
		AccountID: "user@example.com",
		StartedAt: time.Now().UTC(),
		Mode:      "inbox",
		Status:    "running",
	}

	id, err := s.StartSyncRun(ctx, run)
	if err != nil {
		t.Fatalf("StartSyncRun() error = %v", err)
	}

	if err := s.FinishSyncRun(ctx, id, "error", 5, "rate limit exceeded"); err != nil {
		t.Fatalf("FinishSyncRun() error = %v", err)
	}

	var errMsg string
	if err := db.QueryRowContext(ctx, `SELECT error_msg FROM sync_runs WHERE id = ?`, id).Scan(&errMsg); err != nil {
		t.Fatalf("query error = %v", err)
	}
	if errMsg != "rate limit exceeded" {
		t.Errorf("error_msg = %q, want %q", errMsg, "rate limit exceeded")
	}
}

func TestUpsertMessageNilAttachments(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	msg := testMessage()
	msg.Attachments = nil

	if err := s.UpsertMessage(ctx, msg); err != nil {
		t.Fatalf("UpsertMessage() error = %v", err)
	}

	got, err := s.GetMessage(ctx, "", msg.GmailID)
	if err != nil {
		t.Fatalf("GetMessage() error = %v", err)
	}

	if len(got.Attachments) != 0 {
		t.Errorf("Attachments = %v, want empty", got.Attachments)
	}
}

func TestUpsertMessageBCCRecipients(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	msg := testMessage()
	msg.BCC = []types.Address{{Email: "eve@example.com", Name: "Eve"}}

	if err := s.UpsertMessage(ctx, msg); err != nil {
		t.Fatalf("UpsertMessage() error = %v", err)
	}

	got, err := s.GetMessage(ctx, "", msg.GmailID)
	if err != nil {
		t.Fatalf("GetMessage() error = %v", err)
	}

	if len(got.BCC) != 1 || got.BCC[0].Email != "eve@example.com" {
		t.Errorf("BCC = %v, want [{eve@example.com Eve}]", got.BCC)
	}
}
