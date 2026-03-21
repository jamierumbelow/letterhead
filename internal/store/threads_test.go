package store

import (
	"context"
	"testing"
	"time"

	"github.com/jamierumbelow/letterhead/internal/query"
	"github.com/jamierumbelow/letterhead/pkg/types"
)

func TestSearchThreadsFreetextMatch(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	msgs := []*types.Message{
		{
			GmailID: "msg_1", ThreadID: "thread_1", HistoryID: 1,
			InternalDate: 100, ReceivedAt: time.Unix(100, 0).UTC(),
			Subject: "Budget review", Snippet: "Quarterly budget",
			From: types.Address{Email: "alice@example.com", Name: "Alice"},
			To:        []types.Address{{Email: "bob@example.com", Name: "Bob"}},
			Labels:    []string{"INBOX"},
			PlainBody: "Quarterly budget numbers attached",
		},
		{
			GmailID: "msg_2", ThreadID: "thread_2", HistoryID: 2,
			InternalDate: 200, ReceivedAt: time.Unix(200, 0).UTC(),
			Subject: "Lunch plans", Snippet: "Where to eat",
			From: types.Address{Email: "carol@example.com", Name: "Carol"},
			PlainBody: "Let's get pizza",
		},
	}

	for _, m := range msgs {
		if err := s.UpsertMessage(ctx, m); err != nil {
			t.Fatalf("UpsertMessage() error = %v", err)
		}
	}

	q := &query.Query{Terms: []string{"budget"}, Limit: 20}
	results, err := s.SearchThreads(ctx, q)
	if err != nil {
		t.Fatalf("SearchThreads() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("results count = %d, want 1", len(results))
	}

	if results[0].ThreadID != "thread_1" {
		t.Errorf("ThreadID = %q, want thread_1", results[0].ThreadID)
	}
	if results[0].Subject != "Budget review" {
		t.Errorf("Subject = %q", results[0].Subject)
	}
}

func TestSearchThreadsNoResults(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	q := &query.Query{Terms: []string{"nonexistent"}, Limit: 20}
	results, err := s.SearchThreads(ctx, q)
	if err != nil {
		t.Fatalf("SearchThreads() error = %v", err)
	}

	if results != nil {
		t.Errorf("results = %v, want nil", results)
	}
}

func TestSearchThreadsAllMessages(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	msgs := []*types.Message{
		{
			GmailID: "msg_1", ThreadID: "thread_1", HistoryID: 1,
			InternalDate: 300, ReceivedAt: time.Unix(300, 0).UTC(),
			Subject: "Latest update", From: types.Address{Email: "a@example.com"},
			PlainBody: "body one",
		},
		{
			GmailID: "msg_2", ThreadID: "thread_2", HistoryID: 2,
			InternalDate: 100, ReceivedAt: time.Unix(100, 0).UTC(),
			Subject: "Older update", From: types.Address{Email: "b@example.com"},
			PlainBody: "body two",
		},
	}

	for _, m := range msgs {
		if err := s.UpsertMessage(ctx, m); err != nil {
			t.Fatalf("UpsertMessage() error = %v", err)
		}
	}

	// Empty query — returns all threads, latest first
	q := &query.Query{Limit: 20}
	results, err := s.SearchThreads(ctx, q)
	if err != nil {
		t.Fatalf("SearchThreads() error = %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("results count = %d, want 2", len(results))
	}

	// Should be ordered by latest_at DESC
	if results[0].ThreadID != "thread_1" {
		t.Errorf("first result ThreadID = %q, want thread_1", results[0].ThreadID)
	}
}

func TestSearchThreadsFromFilter(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	msgs := []*types.Message{
		{
			GmailID: "msg_1", ThreadID: "thread_1", HistoryID: 1,
			InternalDate: 100, ReceivedAt: time.Unix(100, 0).UTC(),
			Subject: "From alice", From: types.Address{Email: "alice@example.com", Name: "Alice"},
			PlainBody: "hello",
		},
		{
			GmailID: "msg_2", ThreadID: "thread_2", HistoryID: 2,
			InternalDate: 200, ReceivedAt: time.Unix(200, 0).UTC(),
			Subject: "From bob", From: types.Address{Email: "bob@example.com", Name: "Bob"},
			PlainBody: "hello",
		},
	}
	for _, m := range msgs {
		if err := s.UpsertMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
	}

	q := &query.Query{From: []string{"alice"}, Limit: 20}
	results, err := s.SearchThreads(ctx, q)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].ThreadID != "thread_1" {
		t.Errorf("ThreadID = %q, want thread_1", results[0].ThreadID)
	}
}

func TestSearchThreadsThreadGrouping(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	// Two messages in the same thread
	msgs := []*types.Message{
		{
			GmailID: "msg_1", ThreadID: "thread_1", HistoryID: 1,
			InternalDate: 100, ReceivedAt: time.Unix(100, 0).UTC(),
			Subject: "Thread start", From: types.Address{Email: "alice@example.com"},
			PlainBody: "first message",
		},
		{
			GmailID: "msg_2", ThreadID: "thread_1", HistoryID: 2,
			InternalDate: 200, ReceivedAt: time.Unix(200, 0).UTC(),
			Subject: "Re: Thread start", From: types.Address{Email: "bob@example.com"},
			PlainBody: "reply message",
		},
	}
	for _, m := range msgs {
		if err := s.UpsertMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
	}

	q := &query.Query{Terms: []string{"message"}, Limit: 20}
	results, err := s.SearchThreads(ctx, q)
	if err != nil {
		t.Fatal(err)
	}

	// Should return one result (grouped by thread)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (thread grouping)", len(results))
	}
	if results[0].MessageCount != 2 {
		t.Errorf("MessageCount = %d, want 2", results[0].MessageCount)
	}
}

func TestSearchThreadsDateFilter(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	earlyDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	lateDate := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	msgs := []*types.Message{
		{
			GmailID: "msg_1", ThreadID: "thread_1", HistoryID: 1,
			InternalDate: earlyDate.UnixMilli(), ReceivedAt: earlyDate,
			Subject: "Old msg", From: types.Address{Email: "a@example.com"},
			PlainBody: "old",
		},
		{
			GmailID: "msg_2", ThreadID: "thread_2", HistoryID: 2,
			InternalDate: lateDate.UnixMilli(), ReceivedAt: lateDate,
			Subject: "New msg", From: types.Address{Email: "b@example.com"},
			PlainBody: "new",
		},
	}
	for _, m := range msgs {
		if err := s.UpsertMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
	}

	cutoff := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	q := &query.Query{After: &cutoff, Limit: 20}
	results, err := s.SearchThreads(ctx, q)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].ThreadID != "thread_2" {
		t.Errorf("ThreadID = %q, want thread_2", results[0].ThreadID)
	}
}

func TestSearchThreadsLabelFilter(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	msgs := []*types.Message{
		{
			GmailID: "msg_1", ThreadID: "thread_1", HistoryID: 1,
			InternalDate: 100, ReceivedAt: time.Unix(100, 0).UTC(),
			Subject: "Inbox msg", From: types.Address{Email: "a@example.com"},
			Labels: []string{"INBOX"}, PlainBody: "body",
		},
		{
			GmailID: "msg_2", ThreadID: "thread_2", HistoryID: 2,
			InternalDate: 200, ReceivedAt: time.Unix(200, 0).UTC(),
			Subject: "Starred msg", From: types.Address{Email: "b@example.com"},
			Labels: []string{"STARRED"}, PlainBody: "body",
		},
	}
	for _, m := range msgs {
		if err := s.UpsertMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
	}

	q := &query.Query{Labels: []string{"INBOX"}, Limit: 20}
	results, err := s.SearchThreads(ctx, q)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].ThreadID != "thread_1" {
		t.Errorf("ThreadID = %q, want thread_1", results[0].ThreadID)
	}
}

func TestSearchThreadsHasAttachment(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	yes := true
	msgs := []*types.Message{
		{
			GmailID: "msg_1", ThreadID: "thread_1", HistoryID: 1,
			InternalDate: 100, ReceivedAt: time.Unix(100, 0).UTC(),
			Subject: "No attach", From: types.Address{Email: "a@example.com"},
			PlainBody: "body",
		},
		{
			GmailID: "msg_2", ThreadID: "thread_2", HistoryID: 2,
			InternalDate: 200, ReceivedAt: time.Unix(200, 0).UTC(),
			Subject: "With attach", From: types.Address{Email: "b@example.com"},
			PlainBody: "body",
			Attachments: []types.AttachmentMeta{
				{Filename: "doc.pdf", MIMEType: "application/pdf", SizeBytes: 1024},
			},
		},
	}
	for _, m := range msgs {
		if err := s.UpsertMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
	}

	q := &query.Query{HasAttachment: &yes, Limit: 20}
	results, err := s.SearchThreads(ctx, q)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].ThreadID != "thread_2" {
		t.Errorf("ThreadID = %q, want thread_2", results[0].ThreadID)
	}
}
