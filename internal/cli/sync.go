package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jamierumbelow/letterhead/internal/auth"
	"github.com/jamierumbelow/letterhead/internal/config"
	"github.com/jamierumbelow/letterhead/internal/gmail"
	"github.com/jamierumbelow/letterhead/internal/store"
	"github.com/jamierumbelow/letterhead/internal/syncer"
	"github.com/spf13/cobra"
)

func newSyncCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync messages from Gmail",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("not initialized (run letterhead init first): %w", err)
			}

			if cfg.AccountEmail == "" {
				return fmt.Errorf("account_email not set in config")
			}

			// Acquire single-writer lock
			lock, err := store.AcquireLock(cfg.ArchiveRoot)
			if err != nil {
				return err
			}
			defer lock.Release()

			// Handle SIGINT/SIGTERM
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				fmt.Fprintln(cmd.ErrOrStderr(), "\nInterrupted. Saving progress...")
				cancel()
			}()

			// Open database
			db, err := store.Open(store.DatabasePath(cfg.ArchiveRoot))
			if err != nil {
				return err
			}
			defer db.Close()

			s := store.New(db)

			// Get authenticated client
			oc, err := auth.LoadOAuthConfig(cfg.AccountEmail)
			if err != nil {
				return fmt.Errorf("auth not configured: %w", err)
			}

			httpClient, err := oc.GetAuthenticatedClient(ctx)
			if err != nil {
				return fmt.Errorf("not authenticated (run letterhead auth first): %w", err)
			}

			client, err := gmail.NewClient(ctx, httpClient)
			if err != nil {
				return err
			}

			// Record sync run
			runID, err := s.StartSyncRun(ctx, &store.SyncRun{
				AccountID: cfg.AccountEmail,
				StartedAt: time.Now().UTC(),
				Mode:      string(cfg.SyncMode),
				Status:    "running",
			})
			if err != nil {
				return err
			}

			start := time.Now()
			totalSynced := 0

			progress := func(synced int) {
				totalSynced = synced
				elapsed := time.Since(start).Truncate(time.Second)
				fmt.Fprintf(cmd.ErrOrStderr(), "\rSynced %d messages (%s)", synced, elapsed)
			}

			bcfg := syncer.DefaultBootstrapConfig(cfg.AccountEmail)
			err = syncer.Bootstrap(ctx, client, s, bcfg, progress)

			fmt.Fprintln(cmd.ErrOrStderr()) // newline after progress

			if err != nil {
				_ = s.FinishSyncRun(ctx, runID, "error", totalSynced, err.Error())
				return err
			}

			elapsed := time.Since(start).Truncate(time.Second)
			_ = s.FinishSyncRun(ctx, runID, "ok", totalSynced, "")

			fmt.Fprintf(cmd.OutOrStdout(), "Sync complete: %d messages in %s\n", totalSynced, elapsed)
			return nil
		},
	}

	return cmd
}
