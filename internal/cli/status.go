package cli

import (
	"context"
	"database/sql"
	"errors"
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
			output, err := buildStatus()
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

func buildStatus() (types.StatusOutput, error) {
	cfg, err := config.Load()
	switch {
	case err == nil:
		return liveStatus(cfg)
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

func liveStatus(cfg config.Config) (types.StatusOutput, error) {
	out := types.StatusOutput{
		Account:        accountDisplay(cfg.AccountEmail),
		ArchivePath:    cfg.ArchiveRoot,
		SyncMode:       string(cfg.SyncMode),
		SchedulerState: "not installed",
		DBHealth:       databaseHealth(cfg.ArchiveRoot),
	}

	// Try to open the database for live counts
	dbPath := store.DatabasePath(cfg.ArchiveRoot)
	db, err := store.Open(dbPath)
	if err != nil {
		return out, nil
	}
	defer db.Close()

	ctx := context.Background()
	s := store.New(db)

	if count, err := s.CountMessages(ctx); err == nil {
		out.MessageCount = count
	}
	if count, err := s.CountThreads(ctx); err == nil {
		out.ThreadCount = count
	}

	// Load sync state
	if cfg.AccountEmail != "" {
		syncState, err := s.GetSyncState(ctx, cfg.AccountEmail)
		if err == nil {
			out.BootstrapComplete = syncState.BootstrapComplete
			out.LastSyncAt = syncState.LastSyncAt

			if syncState.MessagesSynced > 0 && !syncState.BootstrapComplete {
				// Estimate progress; we don't know total until bootstrap completes
				out.BootstrapProgress = float64(syncState.MessagesSynced)
			} else if syncState.BootstrapComplete {
				out.BootstrapProgress = 100
			}
		}
	}

	out.DBHealth = "ok"

	return out, nil
}

func accountDisplay(email string) string {
	if email != "" {
		// Check if we have a token stored
		oc, err := auth.LoadOAuthConfig(email)
		if err == nil && oc.HasToken() {
			return email
		}
		return email + " (not authenticated)"
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
