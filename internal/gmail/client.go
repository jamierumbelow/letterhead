package gmail

import (
	"context"
	"net/http"

	gm "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const userID = "me"

// Profile contains basic account information.
type Profile struct {
	Email      string
	TotalMsgs  int64
	HistoryID  uint64
}

// Client wraps the Gmail API with a focused read-only interface.
type Client struct {
	svc *gm.Service
}

// NewClient creates a Gmail API client using the provided authenticated HTTP client.
func NewClient(ctx context.Context, httpClient *http.Client) (*Client, error) {
	svc, err := gm.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}
	return &Client{svc: svc}, nil
}

// GetProfile returns the authenticated user's email, message count, and history ID.
func (c *Client) GetProfile(ctx context.Context) (*Profile, error) {
	p, err := c.svc.Users.GetProfile(userID).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return &Profile{
		Email:     p.EmailAddress,
		TotalMsgs: p.MessagesTotal,
		HistoryID: uint64(p.HistoryId),
	}, nil
}

// ListMessages returns a page of message IDs matching the given query.
// Pass an empty query to list all messages. Returns IDs and the next page token.
func (c *Client) ListMessages(ctx context.Context, query string, pageToken string) ([]string, string, error) {
	call := c.svc.Users.Messages.List(userID).Context(ctx).MaxResults(100)
	if query != "" {
		call = call.Q(query)
	}
	if pageToken != "" {
		call = call.PageToken(pageToken)
	}

	resp, err := call.Do()
	if err != nil {
		return nil, "", err
	}

	ids := make([]string, len(resp.Messages))
	for i, msg := range resp.Messages {
		ids[i] = msg.Id
	}

	return ids, resp.NextPageToken, nil
}

// GetMessage retrieves a single message. Format should be "full", "metadata", or "minimal".
func (c *Client) GetMessage(ctx context.Context, id string, format string) (*gm.Message, error) {
	return c.svc.Users.Messages.Get(userID, id).Context(ctx).Format(format).Do()
}

// HistoryResult contains the results of a history list call.
type HistoryResult struct {
	Records      []*gm.History
	NextPage     string
	NewHistoryID uint64
}

// ListHistory returns history records since the given history ID.
func (c *Client) ListHistory(ctx context.Context, startHistoryID uint64, pageToken string) (*HistoryResult, error) {
	call := c.svc.Users.History.List(userID).
		Context(ctx).
		StartHistoryId(startHistoryID).
		MaxResults(100)

	if pageToken != "" {
		call = call.PageToken(pageToken)
	}

	resp, err := call.Do()
	if err != nil {
		return nil, err
	}

	return &HistoryResult{
		Records:      resp.History,
		NextPage:     resp.NextPageToken,
		NewHistoryID: uint64(resp.HistoryId),
	}, nil
}
