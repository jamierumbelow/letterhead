package syncer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jamierumbelow/letterhead/internal/gmail"
	"github.com/jamierumbelow/letterhead/internal/store"
)

const (
	batchSize   = 50
	workerCount = 5
)

// BootstrapConfig controls the bootstrap sync process.
type BootstrapConfig struct {
	AccountEmail string
	Query        string // Gmail query to list messages (e.g. "label:INBOX", "in:anywhere")
	BatchSize    int
	Workers      int
}

// DefaultBootstrapConfig returns sensible defaults for inbox mode.
func DefaultBootstrapConfig(email string) BootstrapConfig {
	return BootstrapConfig{
		AccountEmail: email,
		Query:        "label:INBOX",
		BatchSize:    batchSize,
		Workers:      workerCount,
	}
}

// ProgressFunc is called after each batch with the total synced count.
type ProgressFunc func(synced int)

// Bootstrap performs the initial sync for the configured mode. It is resumable:
// on restart it skips messages already in the local store.
func Bootstrap(ctx context.Context, client *gmail.Client, s *store.Store, cfg BootstrapConfig, progress ProgressFunc) error {
	if progress == nil {
		progress = func(int) {}
	}

	query := cfg.Query
	if query == "" {
		query = "label:INBOX"
	}

	// Step 1: Capture current historyId from Gmail profile
	profile, err := gmail.Retry(ctx, gmail.DefaultRetryConfig(), func() (*gmail.Profile, error) {
		return client.GetProfile(ctx)
	})
	if err != nil {
		return fmt.Errorf("get profile: %w", err)
	}

	// Step 2: List message IDs for the configured query
	type listResult struct {
		IDs      []string
		NextPage string
	}

	var allIDs []string
	pageToken := ""
	for {
		result, err := gmail.Retry(ctx, gmail.DefaultRetryConfig(), func() (listResult, error) {
			ids, np, e := client.ListMessages(ctx, query, pageToken)
			if e != nil {
				return listResult{}, e
			}
			return listResult{IDs: ids, NextPage: np}, nil
		})
		if err != nil {
			return fmt.Errorf("list messages: %w", err)
		}

		allIDs = append(allIDs, result.IDs...)
		pageToken = result.NextPage

		if pageToken == "" {
			break
		}
	}

	// Step 3: Filter out IDs already in local store
	var missingIDs []string
	for _, id := range allIDs {
		exists, err := s.MessageExists(ctx, id)
		if err != nil {
			return fmt.Errorf("check message exists: %w", err)
		}
		if !exists {
			missingIDs = append(missingIDs, id)
		}
	}

	// Step 4: Fetch and store in batches using worker pool
	totalSynced := 0
	syncState, _ := s.GetSyncState(ctx, cfg.AccountEmail)
	if syncState != nil {
		totalSynced = syncState.MessagesSynced
	}

	bs := cfg.BatchSize
	if bs <= 0 {
		bs = batchSize
	}

	for i := 0; i < len(missingIDs); i += bs {
		end := i + bs
		if end > len(missingIDs) {
			end = len(missingIDs)
		}
		batch := missingIDs[i:end]

		if err := fetchAndStoreBatch(ctx, client, s, batch, cfg.Workers); err != nil {
			return fmt.Errorf("batch sync: %w", err)
		}

		totalSynced += len(batch)

		// Step 6: Update sync state after each batch
		now := time.Now().UTC()
		if err := s.SetSyncState(ctx, &store.SyncState{
			AccountID:         cfg.AccountEmail,
			HistoryID:         profile.HistoryID,
			BootstrapComplete: false,
			MessagesSynced:    totalSynced,
			LastSyncAt:        &now,
		}); err != nil {
			return fmt.Errorf("update sync state: %w", err)
		}

		progress(totalSynced)
	}

	// Step 7: Mark bootstrap complete
	now := time.Now().UTC()
	if err := s.SetSyncState(ctx, &store.SyncState{
		AccountID:         cfg.AccountEmail,
		HistoryID:         profile.HistoryID,
		BootstrapComplete: true,
		MessagesSynced:    totalSynced,
		LastSyncAt:        &now,
	}); err != nil {
		return fmt.Errorf("finalize sync state: %w", err)
	}

	return nil
}

func fetchAndStoreBatch(ctx context.Context, client *gmail.Client, s *store.Store, ids []string, workers int) error {
	if workers <= 0 {
		workers = workerCount
	}

	type result struct {
		msg *gmail.MessageData
		err error
	}

	idCh := make(chan string, len(ids))
	resultCh := make(chan result, len(ids))

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range idCh {
				raw, err := gmail.Retry(ctx, gmail.DefaultRetryConfig(), func() (*gmail.MessageData, error) {
					msg, e := client.GetMessage(ctx, id, "full")
					if e != nil {
						return nil, e
					}
					return &gmail.MessageData{Raw: msg}, nil
				})
				resultCh <- result{msg: raw, err: err}
			}
		}()
	}

	for _, id := range ids {
		idCh <- id
	}
	close(idCh)

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for res := range resultCh {
		if res.err != nil {
			return res.err
		}

		normalized := gmail.NormalizeMessage(res.msg.Raw)
		if err := s.UpsertMessage(ctx, normalized); err != nil {
			return err
		}
	}

	return nil
}

// QueryForMode returns the Gmail query string for the given sync mode.
func QueryForMode(mode string, recentWeeks int) string {
	switch mode {
	case "inbox":
		return "label:INBOX"
	case "recent":
		cutoff := time.Now().AddDate(0, 0, -recentWeeks*7)
		return fmt.Sprintf("after:%s", cutoff.Format("2006/01/02"))
	case "full":
		return ""
	default:
		return "label:INBOX"
	}
}
