package mailclient

import (
	"context"
	"fmt"

	"github.com/emersion/go-imap/v2"
	"github.com/jamierumbelow/letterhead/internal/imapclient"
	"github.com/jamierumbelow/letterhead/pkg/types"
)

// IMAPState exposes IMAP-specific sync state for the syncer to persist.
type IMAPState interface {
	UIDValidity() uint32
	LastUID() uint32
}

// IMAPAdapter wraps an *imapclient.Client to implement the MailClient interface.
type IMAPAdapter struct {
	client      *imapclient.Client
	email       string
	uidValidity uint32
	lastUID     uint32
}

// NewIMAPAdapter creates a MailClient backed by IMAP.
func NewIMAPAdapter(client *imapclient.Client, email string) MailClient {
	return &IMAPAdapter{client: client, email: email}
}

// GetProfile returns the account email and inbox message count.
func (a *IMAPAdapter) GetProfile(ctx context.Context) (*Profile, error) {
	selectData, err := a.client.Conn().Select("INBOX", nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("select INBOX: %w", err)
	}

	a.uidValidity = selectData.UIDValidity

	return &Profile{
		Email:     a.email,
		TotalMsgs: int64(selectData.NumMessages),
		HistoryID: 0, // not applicable for IMAP
	}, nil
}

// ListMessageIDs returns UIDs from INBOX as string IDs.
// The pageToken is unused for IMAP (all UIDs are returned in one call).
func (a *IMAPAdapter) ListMessageIDs(ctx context.Context, folder string, pageToken string) ([]string, string, error) {
	uids, uidValidity, err := imapclient.ListUIDs(ctx, a.client.Conn(), folder, 0)
	if err != nil {
		return nil, "", err
	}

	a.uidValidity = uidValidity
	if len(uids) > 0 {
		a.lastUID = uint32(uids[len(uids)-1])
	}

	ids := make([]string, len(uids))
	for i, uid := range uids {
		ids[i] = fmt.Sprintf("%d", uid)
	}

	// IMAP returns all UIDs in one call, no pagination
	return ids, "", nil
}

// FetchMessage retrieves and parses a single message by UID.
func (a *IMAPAdapter) FetchMessage(ctx context.Context, id string) (*types.Message, error) {
	var uid uint32
	if _, err := fmt.Sscanf(id, "%d", &uid); err != nil {
		return nil, fmt.Errorf("invalid IMAP UID %q: %w", id, err)
	}

	msgs, err := imapclient.FetchMessages(ctx, a.client.Conn(), "INBOX", []imap.UID{imap.UID(uid)})
	if err != nil {
		return nil, err
	}

	if len(msgs) == 0 {
		return nil, fmt.Errorf("message UID %d not found", uid)
	}

	msg := msgs[0]
	// Add INBOX label for IMAP messages
	msg.Labels = []string{"INBOX"}

	return msg, nil
}

// UIDValidity returns the UIDVALIDITY of the last accessed mailbox.
func (a *IMAPAdapter) UIDValidity() uint32 {
	return a.uidValidity
}

// LastUID returns the highest UID seen during the last ListMessageIDs call.
func (a *IMAPAdapter) LastUID() uint32 {
	return a.lastUID
}
