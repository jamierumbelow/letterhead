package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/jamierumbelow/letterhead/internal/store"
	"github.com/spf13/cobra"
)

func newRebuildCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild the FTS search index",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := ensureInitialized()
			if err != nil {
				return err
			}

			lock, err := store.AcquireLock(cfg.ArchiveRoot)
			if err != nil {
				return err
			}
			defer lock.Release()

			db, err := store.Open(store.DatabasePath(cfg.ArchiveRoot))
			if err != nil {
				return err
			}
			defer db.Close()

			s := store.New(db)
			ctx := context.Background()

			start := time.Now()
			if err := s.RebuildFTS(ctx); err != nil {
				return fmt.Errorf("rebuild FTS: %w", err)
			}

			count, _ := s.CountMessages(ctx, "")
			elapsed := time.Since(start).Truncate(time.Millisecond)

			fmt.Fprintf(cmd.OutOrStdout(), "FTS index rebuilt: %d messages in %s\n", count, elapsed)
			return nil
		},
	}
}
