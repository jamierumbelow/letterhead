package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/jamierumbelow/letterhead/internal/auth"
	"github.com/jamierumbelow/letterhead/internal/config"
	"github.com/jamierumbelow/letterhead/internal/store"
	"github.com/jamierumbelow/letterhead/pkg/types"
	"github.com/spf13/cobra"
)

func newStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current Letterhead status",
		RunE: func(cmd *cobra.Command, args []string) error {
			accountFlag, _ := cmd.Flags().GetString("account")

			output, err := buildStatus(accountFlag)
			if err != nil {
				return err
			}

			_, formatter, err := formatterFromCommand(cmd)
			if err != nil {
				return err
			}

			return formatter.WriteStatus(cmd.OutOrStdout(), output)
		},
	}
}

func buildStatus(accountFlag string) (types.StatusOutput, error) {
	cfg, err := config.Load()
	switch {
	case err == nil:
		return liveStatus(cfg, accountFlag)
	case errors.Is(err, os.ErrNotExist):
		cfg, err = config.Default()
		if err != nil {
			return types.StatusOutput{}, err
		}
		return phaseZeroStatusOutput(cfg, "not initialized"), nil
	default:
		return types.StatusOutput{}, err
	}
}

func liveStatus(cfg config.Config, accountFlag string) (types.StatusOutput, error) {
	out := types.StatusOutput{
		ArchivePath:    cfg.ArchiveRoot,
		SyncMode:       string(cfg.SyncMode),
		SchedulerState: "not installed",
		DBHealth:       databaseHealth(cfg.ArchiveRoot),
	}

	// Try to open the database for live counts
	dbPath := store.DatabasePath(cfg.ArchiveRoot)
	db, err := store.Open(dbPath)
	if err != nil {
		// Fill in account info even if DB is unavailable
		out.Account = accountDisplayForConfig(cfg, accountFlag)
		return out, nil
	}
	defer db.Close()

	ctx := context.Background()
	s := store.New(db)

	// When a specific account is requested, show just that account
	if accountFlag != "" {
		acct := cfg.AccountByEmail(accountFlag)
		if acct == nil {
			return types.StatusOutput{}, config.ErrAccountNotFound
		}
		out.Account = accountDisplay(acct.Email)
		fillAccountStats(ctx, s, acct.Email, &out)
		out.DBHealth = "ok"
		return out, nil
	}

	// Multiple accounts: show all
	if len(cfg.Accounts) > 1 {
		out.Account = fmt.Sprintf("%d accounts", len(cfg.Accounts))

		var accountStatuses []types.AccountStatus
		for _, acct := range cfg.Accounts {
			as := types.AccountStatus{
				Email:      acct.Email,
				AuthMethod: string(acct.AuthMethod),
			}

			as.Authenticated = auth.IsAuthenticated(acct.Email)

			if count, err := s.CountMessagesForAccount(ctx, acct.Email); err == nil {
				as.MessageCount = count
			}

			syncState, err := s.GetSyncState(ctx, acct.Email)
			if err == nil && syncState.LastSyncAt != nil {
				as.LastSyncAt = syncState.LastSyncAt
			}

			accountStatuses = append(accountStatuses, as)
		}
		out.Accounts = accountStatuses

		// Global aggregates
		if count, err := s.CountMessages(ctx, ""); err == nil {
			out.MessageCount = count
		}
		if count, err := s.CountThreads(ctx, ""); err == nil {
			out.ThreadCount = count
		}

		// Stat the archive directory for size
		if info, err := os.Stat(dbPath); err == nil {
			out.ArchiveSize = info.Size()
		}

		out.DBHealth = "ok"
		return out, nil
	}

	// Single account or no accounts
	if len(cfg.Accounts) == 1 {
		acct := &cfg.Accounts[0]
		out.Account = accountDisplay(acct.Email)
		fillAccountStats(ctx, s, acct.Email, &out)
	} else {
		out.Account = "not configured"
	}

	// Global counts
	if count, err := s.CountMessages(ctx, ""); err == nil {
		out.MessageCount = count
	}
	if count, err := s.CountThreads(ctx, ""); err == nil {
		out.ThreadCount = count
	}

	out.DBHealth = "ok"
	return out, nil
}

func fillAccountStats(ctx context.Context, s *store.Store, email string, out *types.StatusOutput) {
	if count, err := s.CountMessagesForAccount(ctx, email); err == nil {
		out.MessageCount = count
	}
	if count, err := s.CountThreadsForAccount(ctx, email); err == nil {
		out.ThreadCount = count
	}

	syncState, err := s.GetSyncState(ctx, email)
	if err == nil {
		out.BootstrapComplete = syncState.BootstrapComplete
		out.LastSyncAt = syncState.LastSyncAt

		if syncState.MessagesSynced > 0 && !syncState.BootstrapComplete {
			out.BootstrapProgress = float64(syncState.MessagesSynced)
		} else if syncState.BootstrapComplete {
			out.BootstrapProgress = 100
		}
	}
}

func accountDisplay(email string) string {
	if email == "" {
		return "not configured"
	}

	if auth.IsAuthenticated(email) {
		return email
	}
	return email + " (not authenticated)"
}

func accountDisplayForConfig(cfg config.Config, accountFlag string) string {
	if accountFlag != "" {
		return accountDisplay(accountFlag)
	}
	if len(cfg.Accounts) > 1 {
		return fmt.Sprintf("%d accounts", len(cfg.Accounts))
	}
	if len(cfg.Accounts) == 1 {
		return accountDisplay(cfg.Accounts[0].Email)
	}
	return "not configured"
}

func phaseZeroStatusOutput(cfg config.Config, dbHealth string) types.StatusOutput {
	return types.StatusOutput{
		Account:           "not configured",
		ArchivePath:       cfg.ArchiveRoot,
		SyncMode:          string(cfg.SyncMode),
		MessageCount:      0,
		ThreadCount:       0,
		BootstrapComplete: false,
		BootstrapProgress: 0,
		LastSyncAt:        nil,
		SchedulerState:    "not installed",
		DBHealth:          dbHealth,
	}
}

func databaseHealth(archiveRoot string) string {
	dbPath := store.DatabasePath(archiveRoot)
	if _, err := os.Stat(dbPath); err == nil {
		// Try opening to verify it's not corrupted
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			return "error"
		}
		defer db.Close()
		if err := db.Ping(); err != nil {
			return "error"
		}
		return "ok"
	}
	return "not initialized"
}
