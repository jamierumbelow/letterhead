package store

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jamierumbelow/letterhead/pkg/types"
)

func TestGetMessagesInThread(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	// Insert three messages: two in thread_1, one in thread_2
	msgs := []*types.Message{
		{
			GmailID: "msg_a", ThreadID: "thread_1", HistoryID: 1,
			InternalDate: 300, ReceivedAt: time.Unix(300, 0).UTC(),
			Subject: "Re: Hello", From: types.Address{Email: "bob@example.com", Name: "Bob"},
			To: []types.Address{{Email: "alice@example.com", Name: "Alice"}},
			Labels: []string{"INBOX"}, PlainBody: "Reply body",
		},
		{
			GmailID: "msg_b", ThreadID: "thread_1", HistoryID: 1,
			InternalDate: 100, ReceivedAt: time.Unix(100, 0).UTC(),
			Subject: "Hello", From: types.Address{Email: "alice@example.com", Name: "Alice"},
			To: []types.Address{{Email: "bob@example.com", Name: "Bob"}},
			Labels: []string{"INBOX", "SENT"}, PlainBody: "Original body",
		},
		{
			GmailID: "msg_c", ThreadID: "thread_2", HistoryID: 1,
			InternalDate: 200, ReceivedAt: time.Unix(200, 0).UTC(),
			Subject: "Other thread", From: types.Address{Email: "carol@example.com", Name: "Carol"},
			PlainBody: "Different thread",
		},
	}

	for _, m := range msgs {
		if err := s.UpsertMessage(ctx, m); err != nil {
			t.Fatalf("UpsertMessage(%s) error = %v", m.GmailID, err)
		}
	}

	// Get thread_1 messages
	result, err := s.GetMessagesInThread(ctx, "thread_1")
	if err != nil {
		t.Fatalf("GetMessagesInThread() error = %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("len(result) = %d, want 2", len(result))
	}

	// Should be ordered by internal_date ASC
	if result[0].GmailID != "msg_b" {
		t.Errorf("result[0].GmailID = %q, want msg_b", result[0].GmailID)
	}
	if result[1].GmailID != "msg_a" {
		t.Errorf("result[1].GmailID = %q, want msg_a", result[1].GmailID)
	}

	// Check labels are populated
	if len(result[0].Labels) != 2 {
		t.Errorf("result[0].Labels = %v, want [INBOX SENT]", result[0].Labels)
	}

	// Check recipients are populated
	if len(result[0].To) != 1 || result[0].To[0].Email != "bob@example.com" {
		t.Errorf("result[0].To = %v, want [{bob@example.com Bob}]", result[0].To)
	}
}

func TestGetMessagesInThreadEmpty(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	result, err := s.GetMessagesInThread(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetMessagesInThread() error = %v", err)
	}
	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

func TestGetThreadSummary(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	msgs := []*types.Message{
		{
			GmailID: "msg_1", ThreadID: "thread_1", HistoryID: 1,
			InternalDate: 100, ReceivedAt: time.Unix(100, 0).UTC(),
			Subject: "Hello", Snippet: "First message",
			From: types.Address{Email: "alice@example.com", Name: "Alice"},
			To:     []types.Address{{Email: "bob@example.com", Name: "Bob"}},
			Labels: []string{"INBOX"},
		},
		{
			GmailID: "msg_2", ThreadID: "thread_1", HistoryID: 2,
			InternalDate: 200, ReceivedAt: time.Unix(200, 0).UTC(),
			Subject: "Re: Hello", Snippet: "Reply here",
			From: types.Address{Email: "bob@example.com", Name: "Bob"},
			To:     []types.Address{{Email: "alice@example.com", Name: "Alice"}},
			Labels: []string{"INBOX", "IMPORTANT"},
		},
	}

	for _, m := range msgs {
		if err := s.UpsertMessage(ctx, m); err != nil {
			t.Fatalf("UpsertMessage() error = %v", err)
		}
	}

	summary, err := s.GetThreadSummary(ctx, "thread_1")
	if err != nil {
		t.Fatalf("GetThreadSummary() error = %v", err)
	}

	if summary.ThreadID != "thread_1" {
		t.Errorf("ThreadID = %q, want thread_1", summary.ThreadID)
	}
	if summary.MessageCount != 2 {
		t.Errorf("MessageCount = %d, want 2", summary.MessageCount)
	}
	// Subject and snippet should come from the latest message
	if summary.Subject != "Re: Hello" {
		t.Errorf("Subject = %q, want %q", summary.Subject, "Re: Hello")
	}
	if summary.Snippet != "Reply here" {
		t.Errorf("Snippet = %q, want %q", summary.Snippet, "Reply here")
	}
	// Should have both Alice and Bob as participants
	if len(summary.Participants) < 2 {
		t.Errorf("Participants count = %d, want >= 2", len(summary.Participants))
	}
	// Labels should be deduplicated union
	if len(summary.LabelNames) != 2 {
		t.Errorf("LabelNames = %v, want [IMPORTANT INBOX]", summary.LabelNames)
	}
	// MessageIDs should be ordered by internal_date
	if len(summary.MessageIDs) != 2 || summary.MessageIDs[0] != "msg_1" {
		t.Errorf("MessageIDs = %v, want [msg_1 msg_2]", summary.MessageIDs)
	}
}

func TestGetThreadSummaryNotFound(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	_, err := s.GetThreadSummary(ctx, "nonexistent")
	if err != sql.ErrNoRows {
		t.Fatalf("GetThreadSummary() error = %v, want sql.ErrNoRows", err)
	}
}
