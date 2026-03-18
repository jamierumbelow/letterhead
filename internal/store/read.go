package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jamierumbelow/letterhead/pkg/types"
)

// GetMessageByID retrieves a single message by Gmail ID, including
// labels and recipients. This is an alias for GetMessage for read-command clarity.
func (s *Store) GetMessageByID(ctx context.Context, gmailID string) (*types.Message, error) {
	return s.GetMessage(ctx, gmailID)
}

// GetMessagesInThread returns all messages in the given thread,
// ordered by internal_date ASC, with labels and recipients populated.
func (s *Store) GetMessagesInThread(ctx context.Context, threadID string) ([]types.Message, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT gmail_id, thread_id, history_id, internal_date, received_at,
		       subject, snippet, from_addr, from_name,
		       plain_body, html_body, attachment_metadata_json
		FROM messages
		WHERE thread_id = ?
		ORDER BY internal_date ASC`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []types.Message
	for rows.Next() {
		var msg types.Message
		var receivedAtUnix int64
		var attachJSON string

		if err := rows.Scan(
			&msg.GmailID, &msg.ThreadID, &msg.HistoryID, &msg.InternalDate, &receivedAtUnix,
			&msg.Subject, &msg.Snippet, &msg.From.Email, &msg.From.Name,
			&msg.PlainBody, &msg.HTMLBody, &attachJSON,
		); err != nil {
			return nil, err
		}

		msg.ReceivedAt = time.Unix(receivedAtUnix, 0).UTC()

		if err := json.Unmarshal([]byte(attachJSON), &msg.Attachments); err != nil {
			return nil, err
		}

		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Populate labels and recipients for each message
	for i := range messages {
		if err := s.populateLabels(ctx, &messages[i]); err != nil {
			return nil, err
		}
		if err := s.populateRecipients(ctx, &messages[i]); err != nil {
			return nil, err
		}
	}

	return messages, nil
}

// GetThreadSummary returns a ThreadSummary for the given thread ID.
// The read_handle used by find is the thread_id directly.
func (s *Store) GetThreadSummary(ctx context.Context, threadID string) (*types.ThreadSummary, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT thread_id, COUNT(*) as msg_count,
		       MAX(internal_date) as latest_date
		FROM messages
		WHERE thread_id = ?
		GROUP BY thread_id`, threadID)

	var summary types.ThreadSummary
	var latestDate int64

	if err := row.Scan(&summary.ThreadID, &summary.MessageCount, &latestDate); err != nil {
		return nil, err
	}

	summary.LatestAt = time.Unix(0, latestDate*int64(time.Millisecond)).UTC()

	// Get subject and snippet from the latest message
	if err := s.db.QueryRowContext(ctx, `
		SELECT subject, snippet
		FROM messages
		WHERE thread_id = ?
		ORDER BY internal_date DESC
		LIMIT 1`, threadID).Scan(&summary.Subject, &summary.Snippet); err != nil {
		return nil, err
	}

	// Collect participants
	participantRows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT
			CASE WHEN from_name != '' THEN from_name ELSE from_addr END as participant
		FROM messages WHERE thread_id = ?
		UNION
		SELECT DISTINCT
			CASE WHEN name != '' THEN name ELSE addr END as participant
		FROM message_recipients WHERE gmail_id IN (
			SELECT gmail_id FROM messages WHERE thread_id = ?
		)`, threadID, threadID)
	if err != nil {
		return nil, err
	}
	defer participantRows.Close()

	for participantRows.Next() {
		var p string
		if err := participantRows.Scan(&p); err != nil {
			return nil, err
		}
		summary.Participants = append(summary.Participants, p)
	}
	if err := participantRows.Err(); err != nil {
		return nil, err
	}

	// Collect labels
	labelRows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT label
		FROM message_labels
		WHERE gmail_id IN (SELECT gmail_id FROM messages WHERE thread_id = ?)
		ORDER BY label`, threadID)
	if err != nil {
		return nil, err
	}
	defer labelRows.Close()

	for labelRows.Next() {
		var l string
		if err := labelRows.Scan(&l); err != nil {
			return nil, err
		}
		summary.LabelNames = append(summary.LabelNames, l)
	}
	if err := labelRows.Err(); err != nil {
		return nil, err
	}

	// Collect message IDs
	ids, err := s.ListMessageIDsInThread(ctx, threadID)
	if err != nil {
		return nil, err
	}
	summary.MessageIDs = ids

	return &summary, nil
}

func (s *Store) populateLabels(ctx context.Context, msg *types.Message) error {
	rows, err := s.db.QueryContext(ctx, `SELECT label FROM message_labels WHERE gmail_id = ? ORDER BY label`, msg.GmailID)
	if err != nil {
		return err
	}
	defer rows.Close()

	msg.Labels = nil
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return err
		}
		msg.Labels = append(msg.Labels, l)
	}
	return rows.Err()
}

func (s *Store) populateRecipients(ctx context.Context, msg *types.Message) error {
	rows, err := s.db.QueryContext(ctx, `SELECT role, addr, name FROM message_recipients WHERE gmail_id = ? ORDER BY role, addr`, msg.GmailID)
	if err != nil {
		return err
	}
	defer rows.Close()

	msg.To = nil
	msg.CC = nil
	msg.BCC = nil

	for rows.Next() {
		var role, addr, name string
		if err := rows.Scan(&role, &addr, &name); err != nil {
			return err
		}
		a := types.Address{Email: addr, Name: name}
		switch role {
		case "to":
			msg.To = append(msg.To, a)
		case "cc":
			msg.CC = append(msg.CC, a)
		case "bcc":
			msg.BCC = append(msg.BCC, a)
		}
	}
	return rows.Err()
}
