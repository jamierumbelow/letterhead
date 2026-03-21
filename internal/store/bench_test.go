package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jamierumbelow/letterhead/internal/query"
	"github.com/jamierumbelow/letterhead/pkg/types"
)

// BenchmarkMsgUpsert measures upsert throughput.
// Baseline: ~500 msg/sec on SQLite with WAL mode.
func BenchmarkMsgUpsert(b *testing.B) {
	db := openTestDB(b)
	s := New(db)
	ctx := context.Background()

	b.ResetTimer()
	for i := range b.N {
		msg := &types.Message{
			GmailID:    fmt.Sprintf("bench_msg_%d", i),
			ThreadID:   fmt.Sprintf("bench_thread_%d", i%100),
			HistoryID:  uint64(i),
			InternalDate: time.Now().UnixMilli(),
			ReceivedAt: time.Now().UTC(),
			Subject:    fmt.Sprintf("Benchmark message %d", i),
			Snippet:    "This is a benchmark message for performance testing",
			From:       types.Address{Email: "bench@example.com", Name: "Bench User"},
			To:         []types.Address{{Email: "recipient@example.com"}},
			Labels:     []string{"INBOX"},
			PlainBody:  fmt.Sprintf("Benchmark body content %d with some searchable text", i),
		}
		if err := s.UpsertMessage(ctx, msg); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFTSSearch measures FTS5 search performance.
// Baseline: <5ms for 1k messages with a common term.
func BenchmarkFTSSearch(b *testing.B) {
	db := openTestDB(b)
	s := New(db)
	ctx := context.Background()

	// Seed with 500 messages
	for i := range 500 {
		msg := &types.Message{
			GmailID:      fmt.Sprintf("fts_msg_%d", i),
			ThreadID:     fmt.Sprintf("fts_thread_%d", i%50),
			HistoryID:    uint64(i),
			InternalDate: int64(i * 1000),
			ReceivedAt:   time.Unix(int64(i), 0).UTC(),
			Subject:      fmt.Sprintf("Project update %d", i),
			From:         types.Address{Email: "sender@example.com"},
			PlainBody:    fmt.Sprintf("Budget review for quarter %d with detailed analysis", i%4),
		}
		if err := s.UpsertMessage(ctx, msg); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for range b.N {
		q := &query.Query{Terms: []string{"budget"}, Limit: 20}
		if _, err := s.SearchThreads(ctx, q); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkThreadGrouping measures thread grouping performance.
// Baseline: <10ms for 500 messages across 50 threads.
func BenchmarkThreadGrouping(b *testing.B) {
	db := openTestDB(b)
	s := New(db)
	ctx := context.Background()

	// Seed: 500 messages across 50 threads (10 per thread)
	for i := range 500 {
		msg := &types.Message{
			GmailID:      fmt.Sprintf("tg_msg_%d", i),
			ThreadID:     fmt.Sprintf("tg_thread_%d", i%50),
			HistoryID:    uint64(i),
			InternalDate: int64(i * 1000),
			ReceivedAt:   time.Unix(int64(i), 0).UTC(),
			Subject:      fmt.Sprintf("Thread %d message", i%50),
			From:         types.Address{Email: fmt.Sprintf("user%d@example.com", i%10)},
			PlainBody:    "Thread message body content",
		}
		if err := s.UpsertMessage(ctx, msg); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for range b.N {
		q := &query.Query{Limit: 50}
		if _, err := s.SearchThreads(ctx, q); err != nil {
			b.Fatal(err)
		}
	}
}
