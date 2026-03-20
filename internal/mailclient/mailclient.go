package mailclient

import (
	"context"

	"github.com/jamierumbelow/letterhead/pkg/types"
)

// MailClient is the interface both Gmail and IMAP backends implement.
type MailClient interface {
	GetProfile(ctx context.Context) (*Profile, error)
	ListMessageIDs(ctx context.Context, folder string, pageToken string) (ids []string, nextPageToken string, err error)
	FetchMessage(ctx context.Context, id string) (*types.Message, error)
}

// Profile contains basic account information.
type Profile struct {
	Email     string
	TotalMsgs int64
	HistoryID uint64 // 0 for IMAP
}
