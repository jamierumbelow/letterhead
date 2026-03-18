package query

import (
	"strings"
	"testing"
	"time"
)

func TestParseDefaults(t *testing.T) {
	t.Parallel()

	q, err := Parse(nil, QueryFlags{})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if q.Limit != defaultLimit {
		t.Errorf("Limit = %d, want %d", q.Limit, defaultLimit)
	}
	if q.Offset != 0 {
		t.Errorf("Offset = %d, want 0", q.Offset)
	}
}

func TestParseWithTerms(t *testing.T) {
	t.Parallel()

	q, err := Parse([]string{"quarterly", "report"}, QueryFlags{})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(q.Terms) != 2 || q.Terms[0] != "quarterly" || q.Terms[1] != "report" {
		t.Errorf("Terms = %v, want [quarterly report]", q.Terms)
	}
}

func TestParseWithFlags(t *testing.T) {
	t.Parallel()

	hasAttach := true
	q, err := Parse([]string{"hello"}, QueryFlags{
		From:          []string{"alice@example.com"},
		To:            []string{"bob@example.com"},
		Subject:       "update",
		Labels:        []string{"INBOX", "IMPORTANT"},
		After:         "2026-01-01",
		Before:        "2026-12-31",
		HasAttachment: &hasAttach,
		ThreadID:      "thread_1",
		Limit:         50,
		Offset:        10,
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(q.From) != 1 || q.From[0] != "alice@example.com" {
		t.Errorf("From = %v", q.From)
	}
	if q.Subject != "update" {
		t.Errorf("Subject = %q", q.Subject)
	}
	if q.After == nil || q.After.Year() != 2026 || q.After.Month() != 1 {
		t.Errorf("After = %v", q.After)
	}
	if q.Before == nil || q.Before.Month() != 12 {
		t.Errorf("Before = %v", q.Before)
	}
	if q.HasAttachment == nil || !*q.HasAttachment {
		t.Errorf("HasAttachment = %v", q.HasAttachment)
	}
	if q.Limit != 50 {
		t.Errorf("Limit = %d, want 50", q.Limit)
	}
	if q.Offset != 10 {
		t.Errorf("Offset = %d, want 10", q.Offset)
	}
}

func TestParseInvalidDate(t *testing.T) {
	t.Parallel()

	_, err := Parse(nil, QueryFlags{After: "not-a-date"})
	if err == nil {
		t.Fatalf("Parse() expected error for invalid date")
	}
	if !strings.Contains(err.Error(), "--after") {
		t.Errorf("error = %v, want to mention --after", err)
	}

	_, err = Parse(nil, QueryFlags{Before: "nope"})
	if err == nil {
		t.Fatalf("Parse() expected error for invalid date")
	}
}

func TestToSQLFreetextSearch(t *testing.T) {
	t.Parallel()

	q := &Query{Terms: []string{"quarterly", "report"}, Limit: 20}
	sql, params := ToSQL(q)

	if !strings.Contains(sql, "messages_fts MATCH ?") {
		t.Errorf("SQL does not contain FTS match clause:\n%s", sql)
	}
	if !strings.Contains(sql, "LIMIT ? OFFSET ?") {
		t.Errorf("SQL does not contain LIMIT/OFFSET:\n%s", sql)
	}

	// Should have: fts_expr, limit, offset
	if len(params) != 3 {
		t.Errorf("params count = %d, want 3", len(params))
	}

	// First param should be the FTS expression
	if params[0] != "quarterly report" {
		t.Errorf("params[0] = %v, want %q", params[0], "quarterly report")
	}
}

func TestToSQLFromFilter(t *testing.T) {
	t.Parallel()

	q := &Query{From: []string{"alice@example.com"}, Limit: 10}
	sql, params := ToSQL(q)

	if !strings.Contains(sql, "from_addr LIKE ?") {
		t.Errorf("SQL does not contain from_addr filter:\n%s", sql)
	}

	found := false
	for _, p := range params {
		if s, ok := p.(string); ok && s == "%alice@example.com%" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("params %v does not contain from filter value", params)
	}
}

func TestToSQLDateFilter(t *testing.T) {
	t.Parallel()

	after := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q := &Query{After: &after, Limit: 20}
	sql, params := ToSQL(q)

	if !strings.Contains(sql, "internal_date >= ?") {
		t.Errorf("SQL does not contain date filter:\n%s", sql)
	}

	if len(params) < 3 {
		t.Fatalf("params count = %d, want >= 3", len(params))
	}

	if params[0] != after.UnixMilli() {
		t.Errorf("date param = %v, want %d", params[0], after.UnixMilli())
	}
}

func TestToSQLLabelFilter(t *testing.T) {
	t.Parallel()

	q := &Query{Labels: []string{"INBOX"}, Limit: 20}
	sql, _ := ToSQL(q)

	if !strings.Contains(sql, "message_labels") {
		t.Errorf("SQL does not contain label join:\n%s", sql)
	}
}

func TestToSQLAttachmentFilter(t *testing.T) {
	t.Parallel()

	hasAttach := true
	q := &Query{HasAttachment: &hasAttach, Limit: 20}
	sql, _ := ToSQL(q)

	if !strings.Contains(sql, "attachment_metadata_json != '[]'") {
		t.Errorf("SQL does not filter for attachments:\n%s", sql)
	}

	noAttach := false
	q2 := &Query{HasAttachment: &noAttach, Limit: 20}
	sql2, _ := ToSQL(q2)

	if !strings.Contains(sql2, "attachment_metadata_json = '[]'") {
		t.Errorf("SQL does not filter for no attachments:\n%s", sql2)
	}
}

func TestToSQLEmptyQuery(t *testing.T) {
	t.Parallel()

	q := &Query{Limit: 20}
	sql, params := ToSQL(q)

	if !strings.Contains(sql, "FROM messages") {
		t.Errorf("SQL does not select from messages:\n%s", sql)
	}
	if !strings.Contains(sql, "ORDER BY") {
		t.Errorf("SQL does not have ORDER BY:\n%s", sql)
	}

	// Only limit and offset params
	if len(params) != 2 {
		t.Errorf("params count = %d, want 2", len(params))
	}
}
