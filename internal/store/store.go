package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/jamierumbelow/letterhead/pkg/types"
)

// SyncState represents the persistent sync checkpoint for an account.
type SyncState struct {
	AccountID         string
	HistoryID         uint64
	BootstrapComplete bool
	MessagesSynced    int
	LastSyncAt        *time.Time
}

// SyncRun represents one sync operation for audit purposes.
type SyncRun struct {
	ID             int64
	AccountID      string
	StartedAt      time.Time
	FinishedAt     *time.Time
	Mode           string
	MessagesSynced int
	Status         string
	ErrorMsg       string
}

// Store wraps a SQLite database and exposes typed operations for
// messages, sync state, and sync runs.
type Store struct {
	db *sql.DB
}

// New wraps an already-opened *sql.DB. The caller owns the DB lifecycle.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// DB returns the underlying database handle.
func (s *Store) DB() *sql.DB { return s.db }

// UpsertMessage inserts or replaces a message along with its labels
// and recipients. The entire operation runs in a single transaction.
func (s *Store) UpsertMessage(ctx context.Context, msg *types.Message) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint: errcheck

	now := time.Now().UTC().Unix()

	attachJSON, err := json.Marshal(msg.Attachments)
	if err != nil {
		return err
	}
	if msg.Attachments == nil {
		attachJSON = []byte("[]")
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO messages (
			gmail_id, thread_id, history_id, internal_date, received_at,
			subject, snippet, from_addr, from_name,
			plain_body, html_body, attachment_metadata_json,
			raw_size_bytes, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(gmail_id) DO UPDATE SET
			thread_id = excluded.thread_id,
			history_id = excluded.history_id,
			internal_date = excluded.internal_date,
			received_at = excluded.received_at,
			subject = excluded.subject,
			snippet = excluded.snippet,
			from_addr = excluded.from_addr,
			from_name = excluded.from_name,
			plain_body = excluded.plain_body,
			html_body = excluded.html_body,
			attachment_metadata_json = excluded.attachment_metadata_json,
			raw_size_bytes = excluded.raw_size_bytes,
			updated_at = excluded.updated_at`,
		msg.GmailID, msg.ThreadID, msg.HistoryID, msg.InternalDate, msg.ReceivedAt.Unix(),
		msg.Subject, msg.Snippet, msg.From.Email, msg.From.Name,
		msg.PlainBody, msg.HTMLBody, string(attachJSON),
		0, now, now,
	)
	if err != nil {
		return err
	}

	// Replace labels
	if _, err := tx.ExecContext(ctx, `DELETE FROM message_labels WHERE gmail_id = ?`, msg.GmailID); err != nil {
		return err
	}
	for _, label := range msg.Labels {
		if _, err := tx.ExecContext(ctx, `INSERT INTO message_labels (gmail_id, label) VALUES (?, ?)`, msg.GmailID, label); err != nil {
			return err
		}
	}

	// Replace recipients
	if _, err := tx.ExecContext(ctx, `DELETE FROM message_recipients WHERE gmail_id = ?`, msg.GmailID); err != nil {
		return err
	}
	for _, addr := range msg.To {
		if _, err := tx.ExecContext(ctx, `INSERT INTO message_recipients (gmail_id, role, addr, name) VALUES (?, 'to', ?, ?)`, msg.GmailID, addr.Email, addr.Name); err != nil {
			return err
		}
	}
	for _, addr := range msg.CC {
		if _, err := tx.ExecContext(ctx, `INSERT INTO message_recipients (gmail_id, role, addr, name) VALUES (?, 'cc', ?, ?)`, msg.GmailID, addr.Email, addr.Name); err != nil {
			return err
		}
	}
	for _, addr := range msg.BCC {
		if _, err := tx.ExecContext(ctx, `INSERT INTO message_recipients (gmail_id, role, addr, name) VALUES (?, 'bcc', ?, ?)`, msg.GmailID, addr.Email, addr.Name); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetMessage retrieves a single message by Gmail ID, including its
// labels and recipients.
func (s *Store) GetMessage(ctx context.Context, gmailID string) (*types.Message, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT gmail_id, thread_id, history_id, internal_date, received_at,
		       subject, snippet, from_addr, from_name,
		       plain_body, html_body, attachment_metadata_json
		FROM messages WHERE gmail_id = ?`, gmailID)

	var msg types.Message
	var receivedAtUnix int64
	var attachJSON string

	err := row.Scan(
		&msg.GmailID, &msg.ThreadID, &msg.HistoryID, &msg.InternalDate, &receivedAtUnix,
		&msg.Subject, &msg.Snippet, &msg.From.Email, &msg.From.Name,
		&msg.PlainBody, &msg.HTMLBody, &attachJSON,
	)
	if err != nil {
		return nil, err
	}

	msg.ReceivedAt = time.Unix(receivedAtUnix, 0).UTC()

	if err := json.Unmarshal([]byte(attachJSON), &msg.Attachments); err != nil {
		return nil, err
	}

	// Load labels
	labels, err := s.db.QueryContext(ctx, `SELECT label FROM message_labels WHERE gmail_id = ? ORDER BY label`, gmailID)
	if err != nil {
		return nil, err
	}
	defer labels.Close()
	for labels.Next() {
		var l string
		if err := labels.Scan(&l); err != nil {
			return nil, err
		}
		msg.Labels = append(msg.Labels, l)
	}
	if err := labels.Err(); err != nil {
		return nil, err
	}

	// Load recipients
	recips, err := s.db.QueryContext(ctx, `SELECT role, addr, name FROM message_recipients WHERE gmail_id = ? ORDER BY role, addr`, gmailID)
	if err != nil {
		return nil, err
	}
	defer recips.Close()
	for recips.Next() {
		var role, addr, name string
		if err := recips.Scan(&role, &addr, &name); err != nil {
			return nil, err
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
	if err := recips.Err(); err != nil {
		return nil, err
	}

	return &msg, nil
}

// MessageExists returns true if a message with the given Gmail ID exists.
func (s *Store) MessageExists(ctx context.Context, gmailID string) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM messages WHERE gmail_id = ? LIMIT 1`, gmailID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ListMessageIDsInThread returns all Gmail message IDs in the given thread,
// ordered by internal_date ascending.
func (s *Store) ListMessageIDsInThread(ctx context.Context, threadID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT gmail_id FROM messages WHERE thread_id = ? ORDER BY internal_date ASC`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// CountMessages returns the total number of stored messages.
func (s *Store) CountMessages(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages`).Scan(&count)
	return count, err
}

// CountThreads returns the number of distinct threads.
func (s *Store) CountThreads(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT thread_id) FROM messages`).Scan(&count)
	return count, err
}

// GetSyncState retrieves the sync checkpoint for the given account.
// Returns sql.ErrNoRows if no state exists yet.
func (s *Store) GetSyncState(ctx context.Context, accountID string) (*SyncState, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT account_id, history_id, bootstrap_complete, messages_synced, last_sync_at
		FROM sync_state WHERE account_id = ?`, accountID)

	var st SyncState
	var bootstrapInt int
	var lastSyncUnix sql.NullInt64

	err := row.Scan(&st.AccountID, &st.HistoryID, &bootstrapInt, &st.MessagesSynced, &lastSyncUnix)
	if err != nil {
		return nil, err
	}

	st.BootstrapComplete = bootstrapInt != 0
	if lastSyncUnix.Valid {
		t := time.Unix(lastSyncUnix.Int64, 0).UTC()
		st.LastSyncAt = &t
	}

	return &st, nil
}

// SetSyncState upserts the sync checkpoint for the given account.
func (s *Store) SetSyncState(ctx context.Context, st *SyncState) error {
	var lastSyncUnix *int64
	if st.LastSyncAt != nil {
		v := st.LastSyncAt.Unix()
		lastSyncUnix = &v
	}

	bootstrapInt := 0
	if st.BootstrapComplete {
		bootstrapInt = 1
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sync_state (account_id, history_id, bootstrap_complete, messages_synced, last_sync_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(account_id) DO UPDATE SET
			history_id = excluded.history_id,
			bootstrap_complete = excluded.bootstrap_complete,
			messages_synced = excluded.messages_synced,
			last_sync_at = excluded.last_sync_at`,
		st.AccountID, st.HistoryID, bootstrapInt, st.MessagesSynced, lastSyncUnix,
	)
	return err
}

// StartSyncRun records the beginning of a sync run and returns its ID.
func (s *Store) StartSyncRun(ctx context.Context, run *SyncRun) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO sync_runs (account_id, started_at, mode, messages_synced, status, error_msg)
		VALUES (?, ?, ?, 0, ?, '')`,
		run.AccountID, run.StartedAt.Unix(), run.Mode, run.Status,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// AllMessageIDs returns every gmail_id in the store.
func (s *Store) AllMessageIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT gmail_id FROM messages`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DeleteMessage removes a message and its associated labels and recipients.
func (s *Store) DeleteMessage(ctx context.Context, gmailID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint: errcheck

	if _, err := tx.ExecContext(ctx, `DELETE FROM message_labels WHERE gmail_id = ?`, gmailID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM message_recipients WHERE gmail_id = ?`, gmailID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM messages WHERE gmail_id = ?`, gmailID); err != nil {
		return err
	}

	return tx.Commit()
}

// FinishSyncRun records the outcome of a completed sync run.
func (s *Store) FinishSyncRun(ctx context.Context, id int64, status string, count int, errMsg string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE sync_runs SET finished_at = ?, messages_synced = ?, status = ?, error_msg = ?
		WHERE id = ?`,
		time.Now().UTC().Unix(), count, status, errMsg, id,
	)
	return err
}
