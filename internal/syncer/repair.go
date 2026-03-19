package syncer

import (
	"context"
	"fmt"
	"time"

	"github.com/jamierumbelow/letterhead/internal/gmail"
	"github.com/jamierumbelow/letterhead/internal/store"
)

// RepairResult summarizes the outcome of a repair sync.
type RepairResult struct {
	Added   int
	Deleted int
}

// RepairSync re-diffs the local store against Gmail for the given query scope.
// This is used when history has expired or when explicitly requested.
func RepairSync(ctx context.Context, client *gmail.Client, s *store.Store, accountEmail string, query string, progress ProgressFunc) (*RepairResult, error) {
	if progress == nil {
		progress = func(int) {}
	}

	// Step 1: Fetch current history ID from Gmail profile
	profile, err := gmail.Retry(ctx, gmail.DefaultRetryConfig(), func() (*gmail.Profile, error) {
		return client.GetProfile(ctx)
	})
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}

	// Step 2: List all message IDs from Gmail for this scope
	remoteIDs := make(map[string]bool)
	pageToken := ""
	for {
		type listResult struct {
			IDs      []string
			NextPage string
		}
		result, err := gmail.Retry(ctx, gmail.DefaultRetryConfig(), func() (listResult, error) {
			ids, np, e := client.ListMessages(ctx, query, pageToken)
			if e != nil {
				return listResult{}, e
			}
			return listResult{IDs: ids, NextPage: np}, nil
		})
		if err != nil {
			return nil, fmt.Errorf("list messages: %w", err)
		}

		for _, id := range result.IDs {
			remoteIDs[id] = true
		}
		pageToken = result.NextPage
		if pageToken == "" {
			break
		}
	}

	// Step 3: Get local message IDs
	localIDs, err := s.AllMessageIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list local messages: %w", err)
	}

	localIDSet := make(map[string]bool, len(localIDs))
	for _, id := range localIDs {
		localIDSet[id] = true
	}

	result := &RepairResult{}

	// Fetch messages in Gmail but not locally
	var toFetch []string
	for id := range remoteIDs {
		if !localIDSet[id] {
			toFetch = append(toFetch, id)
		}
	}

	// Batch fetch missing messages
	for i := 0; i < len(toFetch); i += batchSize {
		end := i + batchSize
		if end > len(toFetch) {
			end = len(toFetch)
		}
		batch := toFetch[i:end]

		if err := fetchAndStoreBatch(ctx, client, s, batch, workerCount); err != nil {
			return nil, fmt.Errorf("fetch missing batch: %w", err)
		}
		result.Added += len(batch)
		progress(result.Added)
	}

	// Delete messages locally that are not in Gmail
	for _, id := range localIDs {
		if !remoteIDs[id] {
			if err := s.DeleteMessage(ctx, id); err != nil {
				return nil, fmt.Errorf("delete message %s: %w", id, err)
			}
			result.Deleted++
		}
	}

	// Step 4: Update sync state with new history ID
	now := time.Now().UTC()
	msgCount, _ := s.CountMessages(ctx)
	if err := s.SetSyncState(ctx, &store.SyncState{
		AccountID:         accountEmail,
		HistoryID:         profile.HistoryID,
		BootstrapComplete: true,
		MessagesSynced:    msgCount,
		LastSyncAt:        &now,
	}); err != nil {
		return nil, fmt.Errorf("update sync state: %w", err)
	}

	return result, nil
}
