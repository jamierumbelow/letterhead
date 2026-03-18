package query

import (
	"fmt"
	"strings"
	"time"
)

const defaultLimit = 20

// Query represents a structured search request.
type Query struct {
	Terms         []string   // freetext search terms
	From          []string   // filter by sender address
	To            []string   // filter by recipient address
	Subject       string     // filter by subject (substring)
	Labels        []string   // filter by label
	After         *time.Time // messages after this time
	Before        *time.Time // messages before this time
	HasAttachment *bool      // filter by attachment presence
	ThreadID      string     // filter by specific thread
	Limit         int        // max results (default 20)
	Offset        int        // pagination offset
}

// QueryFlags mirrors the cobra flags for the find command.
type QueryFlags struct {
	From          []string
	To            []string
	Subject       string
	Labels        []string
	After         string
	Before        string
	HasAttachment *bool
	ThreadID      string
	Limit         int
	Offset        int
}

// Parse builds a Query from free-text arguments and structured flags.
func Parse(args []string, flags QueryFlags) (*Query, error) {
	q := &Query{
		Terms:         args,
		From:          flags.From,
		To:            flags.To,
		Subject:       flags.Subject,
		Labels:        flags.Labels,
		HasAttachment: flags.HasAttachment,
		ThreadID:      flags.ThreadID,
		Limit:         flags.Limit,
		Offset:        flags.Offset,
	}

	if flags.After != "" {
		t, err := parseDate(flags.After)
		if err != nil {
			return nil, fmt.Errorf("invalid --after date: %w", err)
		}
		q.After = &t
	}

	if flags.Before != "" {
		t, err := parseDate(flags.Before)
		if err != nil {
			return nil, fmt.Errorf("invalid --before date: %w", err)
		}
		q.Before = &t
	}

	if q.Limit <= 0 {
		q.Limit = defaultLimit
	}

	return q, nil
}

// ToSQL generates a parameterized SQL query that returns thread-grouped
// results: one row per thread, with the latest message's data.
func ToSQL(q *Query) (string, []interface{}) {
	var conditions []string
	var params []interface{}

	// FTS5 freetext search
	if len(q.Terms) > 0 {
		ftsExpr := strings.Join(q.Terms, " ")
		conditions = append(conditions, `m.rowid IN (SELECT rowid FROM messages_fts WHERE messages_fts MATCH ?)`)
		params = append(params, ftsExpr)
	}

	// From filter
	for _, from := range q.From {
		conditions = append(conditions, `(m.from_addr LIKE ? OR m.from_name LIKE ?)`)
		pattern := "%" + from + "%"
		params = append(params, pattern, pattern)
	}

	// To filter
	for _, to := range q.To {
		conditions = append(conditions, `EXISTS (
			SELECT 1 FROM message_recipients r
			WHERE r.gmail_id = m.gmail_id AND (r.addr LIKE ? OR r.name LIKE ?)
		)`)
		pattern := "%" + to + "%"
		params = append(params, pattern, pattern)
	}

	// Subject filter
	if q.Subject != "" {
		conditions = append(conditions, `m.subject LIKE ?`)
		params = append(params, "%"+q.Subject+"%")
	}

	// Label filter
	for _, label := range q.Labels {
		conditions = append(conditions, `EXISTS (
			SELECT 1 FROM message_labels l
			WHERE l.gmail_id = m.gmail_id AND l.label = ?
		)`)
		params = append(params, label)
	}

	// Date filters (internal_date is milliseconds since epoch)
	if q.After != nil {
		conditions = append(conditions, `m.internal_date >= ?`)
		params = append(params, q.After.UnixMilli())
	}
	if q.Before != nil {
		conditions = append(conditions, `m.internal_date < ?`)
		params = append(params, q.Before.UnixMilli())
	}

	// Attachment filter
	if q.HasAttachment != nil {
		if *q.HasAttachment {
			conditions = append(conditions, `m.attachment_metadata_json != '[]'`)
		} else {
			conditions = append(conditions, `m.attachment_metadata_json = '[]'`)
		}
	}

	// Thread filter
	if q.ThreadID != "" {
		conditions = append(conditions, `m.thread_id = ?`)
		params = append(params, q.ThreadID)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Thread-grouped query: group by thread_id, return the latest message per thread
	sql := fmt.Sprintf(`
		SELECT m.thread_id, m.subject, m.snippet, m.from_addr, m.from_name,
		       m.internal_date, COUNT(*) OVER (PARTITION BY m.thread_id) as msg_count
		FROM messages m
		INNER JOIN (
			SELECT thread_id, MAX(internal_date) as max_date
			FROM messages
			%s
			GROUP BY thread_id
		) latest ON m.thread_id = latest.thread_id AND m.internal_date = latest.max_date
		%s
		ORDER BY m.internal_date DESC
		LIMIT ? OFFSET ?`, where, where)

	// Duplicate params for subquery and outer query
	allParams := make([]interface{}, 0, len(params)*2+2)
	allParams = append(allParams, params...)
	allParams = append(allParams, params...)
	allParams = append(allParams, q.Limit, q.Offset)

	return sql, allParams
}

func parseDate(s string) (time.Time, error) {
	// Try common formats
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("could not parse %q (expected YYYY-MM-DD or RFC3339)", s)
}
