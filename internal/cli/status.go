package cli

import (
	"errors"
	"os"

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
			output, err := phaseZeroStatus()
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

func phaseZeroStatus() (types.StatusOutput, error) {
	cfg, err := config.Load()
	switch {
	case err == nil:
		return phaseZeroStatusOutput(cfg, databaseHealth(cfg.ArchiveRoot)), nil
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

func phaseZeroStatusOutput(cfg config.Config, dbHealth string) types.StatusOutput {
	return types.StatusOutput{
		Account:           "not authenticated",
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
	if _, err := os.Stat(store.DatabasePath(archiveRoot)); err == nil {
		return "ok"
	}

	return "not initialized"
}
