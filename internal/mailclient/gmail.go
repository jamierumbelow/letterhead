package mailclient

import (
	"context"

	"github.com/jamierumbelow/letterhead/internal/gmail"
	"github.com/jamierumbelow/letterhead/pkg/types"
)

// GmailAdapter wraps a *gmail.Client to implement the MailClient interface.
type GmailAdapter struct {
	client *gmail.Client
}

// NewGmailAdapter creates a MailClient backed by the Gmail API.
func NewGmailAdapter(client *gmail.Client) MailClient {
	return &GmailAdapter{client: client}
}

// GetProfile returns the authenticated user's profile.
func (g *GmailAdapter) GetProfile(ctx context.Context) (*Profile, error) {
	p, err := gmail.Retry(ctx, gmail.DefaultRetryConfig(), func() (*gmail.Profile, error) {
		return g.client.GetProfile(ctx)
	})
	if err != nil {
		return nil, err
	}
	return &Profile{
		Email:     p.Email,
		TotalMsgs: p.TotalMsgs,
		HistoryID: p.HistoryID,
	}, nil
}

// ListMessageIDs returns a page of message IDs for the given folder.
func (g *GmailAdapter) ListMessageIDs(ctx context.Context, folder string, pageToken string) ([]string, string, error) {
	type listResult struct {
		IDs      []string
		NextPage string
	}

	result, err := gmail.Retry(ctx, gmail.DefaultRetryConfig(), func() (listResult, error) {
		ids, np, e := g.client.ListMessages(ctx, "label:"+folder, pageToken)
		if e != nil {
			return listResult{}, e
		}
		return listResult{IDs: ids, NextPage: np}, nil
	})
	if err != nil {
		return nil, "", err
	}
	return result.IDs, result.NextPage, nil
}

// FetchMessage retrieves and normalizes a single message.
func (g *GmailAdapter) FetchMessage(ctx context.Context, id string) (*types.Message, error) {
	raw, err := gmail.Retry(ctx, gmail.DefaultRetryConfig(), func() (*gmail.MessageData, error) {
		msg, e := g.client.GetMessage(ctx, id, "full")
		if e != nil {
			return nil, e
		}
		return &gmail.MessageData{Raw: msg}, nil
	})
	if err != nil {
		return nil, err
	}
	return gmail.NormalizeMessage(raw.Raw), nil
}
