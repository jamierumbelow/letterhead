package syncer

import (
	"context"
	"fmt"
	"time"

	"github.com/jamierumbelow/letterhead/internal/gmail"
	"github.com/jamierumbelow/letterhead/internal/store"
)

// IncrementalResult summarizes the outcome of an incremental sync.
type IncrementalResult struct {
	Added   int
	Deleted int
	Labels  int
}

// Incremental syncs changes since the last saved history ID.
// Returns ErrHistoryExpired (from gmail package) if the checkpoint is too old.
func Incremental(ctx context.Context, client *gmail.Client, s *store.Store, accountEmail string) (*IncrementalResult, error) {
	// Step 1: Read last saved history ID
	syncState, err := s.GetSyncState(ctx, accountEmail)
	if err != nil {
		return nil, fmt.Errorf("get sync state: %w", err)
	}
	if syncState.HistoryID == 0 {
		return nil, fmt.Errorf("no history checkpoint; run bootstrap first")
	}

	result := &IncrementalResult{}
	currentHistoryID := syncState.HistoryID

	// Step 2-3: Fetch and process history pages
	pageToken := ""
	for {
		fetchResult, err := gmail.Retry(ctx, gmail.DefaultRetryConfig(), func() (*gmail.FetchHistoryResult, error) {
			return client.FetchHistory(ctx, currentHistoryID, pageToken)
		})
		if err != nil {
			return nil, err // includes gmail.ErrHistoryExpired
		}

		// Process each history record
		for _, rec := range fetchResult.Records {
			// Handle added messages: fetch full content and upsert
			for _, added := range rec.MessagesAdded {
				raw, err := gmail.Retry(ctx, gmail.DefaultRetryConfig(), func() (*gmail.MessageData, error) {
					msg, e := client.GetMessage(ctx, added.MessageID, "full")
					if e != nil {
						return nil, e
					}
					return &gmail.MessageData{Raw: msg}, nil
				})
				if err != nil {
					return nil, fmt.Errorf("fetch added message %s: %w", added.MessageID, err)
				}

				normalized := gmail.NormalizeMessage(raw.Raw)
				normalized.AccountID = accountEmail
				if err := s.UpsertMessage(ctx, normalized); err != nil {
					return nil, fmt.Errorf("upsert message %s: %w", added.MessageID, err)
				}
				result.Added++
			}

			// Handle deleted messages
			for _, deleted := range rec.MessagesDeleted {
				if err := s.DeleteMessage(ctx, accountEmail, deleted.MessageID); err != nil {
					return nil, fmt.Errorf("delete message %s: %w", deleted.MessageID, err)
				}
				result.Deleted++
			}

			// Handle label changes: re-fetch the message to get current labels
			for _, lc := range rec.LabelsAdded {
				if err := refetchLabels(ctx, client, s, accountEmail, lc.MessageID); err != nil {
					return nil, err
				}
				result.Labels++
			}
			for _, lc := range rec.LabelsRemoved {
				if err := refetchLabels(ctx, client, s, accountEmail, lc.MessageID); err != nil {
					return nil, err
				}
				result.Labels++
			}
		}

		// Step 5: Advance checkpoint after processing this page
		now := time.Now().UTC()
		if err := s.SetSyncState(ctx, &store.SyncState{
			AccountID:         accountEmail,
			HistoryID:         fetchResult.NewHistoryID,
			BootstrapComplete: true,
			MessagesSynced:    syncState.MessagesSynced + result.Added,
			LastSyncAt:        &now,
		}); err != nil {
			return nil, fmt.Errorf("update sync state: %w", err)
		}

		currentHistoryID = fetchResult.NewHistoryID
		pageToken = fetchResult.NextPage
		if pageToken == "" {
			break
		}
	}

	return result, nil
}

// refetchLabels re-fetches a message's metadata and updates its labels in the store.
func refetchLabels(ctx context.Context, client *gmail.Client, s *store.Store, accountEmail string, messageID string) error {
	exists, err := s.MessageExists(ctx, accountEmail, messageID)
	if err != nil {
		return fmt.Errorf("check message %s: %w", messageID, err)
	}
	if !exists {
		return nil // message not in our store, skip
	}

	raw, err := gmail.Retry(ctx, gmail.DefaultRetryConfig(), func() (*gmail.MessageData, error) {
		msg, e := client.GetMessage(ctx, messageID, "full")
		if e != nil {
			return nil, e
		}
		return &gmail.MessageData{Raw: msg}, nil
	})
	if err != nil {
		return fmt.Errorf("refetch message %s: %w", messageID, err)
	}

	normalized := gmail.NormalizeMessage(raw.Raw)
	normalized.AccountID = accountEmail
	if err := s.UpsertMessage(ctx, normalized); err != nil {
		return fmt.Errorf("upsert message %s: %w", messageID, err)
	}
	return nil
}
