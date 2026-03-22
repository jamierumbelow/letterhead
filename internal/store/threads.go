package store

import (
	"context"
	"time"

	"github.com/jamierumbelow/letterhead/internal/query"
	"github.com/jamierumbelow/letterhead/pkg/types"
)

// SearchThreads executes a query and returns thread-grouped results.
func (s *Store) SearchThreads(ctx context.Context, q *query.Query) ([]types.ThreadSummary, error) {
	sqlStr, params := query.ToSQL(q)

	rows, err := s.db.QueryContext(ctx, sqlStr, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []types.ThreadSummary

	for rows.Next() {
		var (
			threadID     string
			subject      string
			snippet      string
			fromAddr     string
			fromName     string
			internalDate int64
			msgCount     int
		)

		if err := rows.Scan(&threadID, &subject, &snippet, &fromAddr, &fromName, &internalDate, &msgCount); err != nil {
			return nil, err
		}

		latestAt := time.Unix(0, internalDate*int64(time.Millisecond)).UTC()

		// Collect participants for this thread
		participants, err := s.threadParticipants(ctx, threadID)
		if err != nil {
			return nil, err
		}

		// Collect labels for this thread
		labels, err := s.threadLabels(ctx, threadID)
		if err != nil {
			return nil, err
		}

		// Collect message IDs
		messageIDs, err := s.ListMessageIDsInThread(ctx, "", threadID)
		if err != nil {
			return nil, err
		}

		results = append(results, types.ThreadSummary{
			ThreadID:     threadID,
			Subject:      subject,
			Participants: participants,
			LatestAt:     latestAt,
			MessageCount: msgCount,
			Snippet:      snippet,
			LabelNames:   labels,
			MessageIDs:   messageIDs,
		})
	}

	return results, rows.Err()
}

func (s *Store) threadParticipants(ctx context.Context, threadID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT
			CASE WHEN from_name != '' THEN from_name ELSE from_addr END
		FROM messages WHERE thread_id = ?
		UNION
		SELECT DISTINCT
			CASE WHEN name != '' THEN name ELSE addr END
		FROM message_recipients WHERE gmail_id IN (
			SELECT gmail_id FROM messages WHERE thread_id = ?
		)`, threadID, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participants []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		participants = append(participants, p)
	}
	return participants, rows.Err()
}

func (s *Store) threadLabels(ctx context.Context, threadID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT label
		FROM message_labels
		WHERE gmail_id IN (SELECT gmail_id FROM messages WHERE thread_id = ?)
		ORDER BY label`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var labels []string
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return nil, err
		}
		labels = append(labels, l)
	}
	return labels, rows.Err()
}
