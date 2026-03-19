package gmail

import (
	"context"
	"errors"

	"google.golang.org/api/googleapi"
	gm "google.golang.org/api/gmail/v1"
)

// ErrHistoryExpired is returned when the requested history ID is no longer
// available on the server (HTTP 404 with reason historyNotFound).
var ErrHistoryExpired = errors.New("gmail: history has expired")

// HistoryRecord is a parsed representation of one history entry.
type HistoryRecord struct {
	MessagesAdded   []HistoryMessage
	MessagesDeleted []HistoryMessage
	LabelsAdded     []LabelChange
	LabelsRemoved   []LabelChange
}

// HistoryMessage identifies a message affected by a history event.
type HistoryMessage struct {
	MessageID string
	ThreadID  string
}

// LabelChange records a label being added or removed from a message.
type LabelChange struct {
	MessageID string
	ThreadID  string
	LabelIDs  []string
}

// FetchHistoryResult contains the results of a history fetch call.
type FetchHistoryResult struct {
	Records      []HistoryRecord
	NewHistoryID uint64
	NextPage     string
}

// FetchHistory retrieves history records starting from the given history ID.
// Returns ErrHistoryExpired if the history ID is no longer available.
func (c *Client) FetchHistory(ctx context.Context, startHistoryID uint64, pageToken string) (*FetchHistoryResult, error) {
	call := c.svc.Users.History.List(userID).
		Context(ctx).
		StartHistoryId(startHistoryID).
		MaxResults(100)

	if pageToken != "" {
		call = call.PageToken(pageToken)
	}

	resp, err := call.Do()
	if err != nil {
		if isHistoryExpired(err) {
			return nil, ErrHistoryExpired
		}
		return nil, err
	}

	records := make([]HistoryRecord, 0, len(resp.History))
	for _, h := range resp.History {
		records = append(records, parseHistoryEntry(h))
	}

	return &FetchHistoryResult{
		Records:      records,
		NewHistoryID: uint64(resp.HistoryId),
		NextPage:     resp.NextPageToken,
	}, nil
}

func isHistoryExpired(err error) bool {
	apiErr, ok := err.(*googleapi.Error)
	if !ok || apiErr.Code != 404 {
		return false
	}
	for _, e := range apiErr.Errors {
		if e.Reason == "notFound" {
			return true
		}
	}
	return false
}

func parseHistoryEntry(h *gm.History) HistoryRecord {
	var rec HistoryRecord

	for _, ma := range h.MessagesAdded {
		if ma.Message != nil {
			rec.MessagesAdded = append(rec.MessagesAdded, HistoryMessage{
				MessageID: ma.Message.Id,
				ThreadID:  ma.Message.ThreadId,
			})
		}
	}

	for _, md := range h.MessagesDeleted {
		if md.Message != nil {
			rec.MessagesDeleted = append(rec.MessagesDeleted, HistoryMessage{
				MessageID: md.Message.Id,
				ThreadID:  md.Message.ThreadId,
			})
		}
	}

	for _, la := range h.LabelsAdded {
		if la.Message != nil {
			rec.LabelsAdded = append(rec.LabelsAdded, LabelChange{
				MessageID: la.Message.Id,
				ThreadID:  la.Message.ThreadId,
				LabelIDs:  la.LabelIds,
			})
		}
	}

	for _, lr := range h.LabelsRemoved {
		if lr.Message != nil {
			rec.LabelsRemoved = append(rec.LabelsRemoved, LabelChange{
				MessageID: lr.Message.Id,
				ThreadID:  lr.Message.ThreadId,
				LabelIDs:  lr.LabelIds,
			})
		}
	}

	return rec
}
