package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jamierumbelow/letterhead/internal/auth"
	"github.com/jamierumbelow/letterhead/internal/gmail"
	"github.com/jamierumbelow/letterhead/internal/store"
	"github.com/jamierumbelow/letterhead/internal/syncer"
	"github.com/spf13/cobra"
)

func newSyncCommand() *cobra.Command {
	var repair bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync messages from Gmail",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := ensureInitialized()
			if err != nil {
				return err
			}

			if cfg.AccountEmail == "" {
				return fmt.Errorf("account_email not set in config; add it to %s", configPathHint())
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

			// Get authenticated client (uses stored token or triggers interactive OAuth)
			result, err := auth.GetClient(ctx, cfg.AccountEmail)
			if err != nil {
				return err
			}

			if result.Method == auth.AuthMethodInteractive {
				fmt.Fprintln(cmd.ErrOrStderr(), "Authenticated successfully.")
			}

			client, err := gmail.NewClient(ctx, result.Client)
			if err != nil {
				return err
			}

			// Check sync state to decide bootstrap vs incremental vs repair
			syncState, _ := s.GetSyncState(ctx, cfg.AccountEmail)
			bootstrapComplete := syncState != nil && syncState.BootstrapComplete

			if repair {
				return runRepairSync(ctx, cmd, client, s, cfg.AccountEmail)
			}

			if bootstrapComplete {
				return runIncrementalSync(ctx, cmd, client, s, cfg.AccountEmail)
			}

			return runBootstrapSync(ctx, cmd, client, s, cfg.AccountEmail)
		},
	}

	cmd.Flags().BoolVar(&repair, "repair", false, "Force a repair sync (re-diff against Gmail)")

	cmd.AddCommand(newSyncInstallCommand())
	cmd.AddCommand(newSyncUninstallCommand())

	return cmd
}

func runBootstrapSync(ctx context.Context, cmd *cobra.Command, client *gmail.Client, s *store.Store, email string) error {
	runID, err := s.StartSyncRun(ctx, &store.SyncRun{
		AccountID: email,
		StartedAt: time.Now().UTC(),
		Mode:      "bootstrap",
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

	bcfg := syncer.DefaultBootstrapConfig(email)
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
}

func runIncrementalSync(ctx context.Context, cmd *cobra.Command, client *gmail.Client, s *store.Store, email string) error {
	runID, err := s.StartSyncRun(ctx, &store.SyncRun{
		AccountID: email,
		StartedAt: time.Now().UTC(),
		Mode:      "incremental",
		Status:    "running",
	})
	if err != nil {
		return err
	}

	start := time.Now()

	result, err := syncer.Incremental(ctx, client, s, email)
	if err != nil {
		if errors.Is(err, gmail.ErrHistoryExpired) {
			fmt.Fprintln(cmd.ErrOrStderr(), "History expired. Running repair sync...")
			_ = s.FinishSyncRun(ctx, runID, "expired", 0, "history expired, falling back to repair")
			return runRepairSync(ctx, cmd, client, s, email)
		}
		_ = s.FinishSyncRun(ctx, runID, "error", 0, err.Error())
		return err
	}

	elapsed := time.Since(start).Truncate(time.Second)
	_ = s.FinishSyncRun(ctx, runID, "ok", result.Added, "")

	fmt.Fprintf(cmd.OutOrStdout(), "Sync complete: %d added, %d deleted, %d label changes in %s\n",
		result.Added, result.Deleted, result.Labels, elapsed)
	return nil
}

func runRepairSync(ctx context.Context, cmd *cobra.Command, client *gmail.Client, s *store.Store, email string) error {
	runID, err := s.StartSyncRun(ctx, &store.SyncRun{
		AccountID: email,
		StartedAt: time.Now().UTC(),
		Mode:      "repair",
		Status:    "running",
	})
	if err != nil {
		return err
	}

	start := time.Now()

	progress := func(synced int) {
		elapsed := time.Since(start).Truncate(time.Second)
		fmt.Fprintf(cmd.ErrOrStderr(), "\rRepair: fetched %d messages (%s)", synced, elapsed)
	}

	result, err := syncer.RepairSync(ctx, client, s, email, "label:INBOX", progress)

	fmt.Fprintln(cmd.ErrOrStderr()) // newline after progress

	if err != nil {
		_ = s.FinishSyncRun(ctx, runID, "error", 0, err.Error())
		return err
	}

	elapsed := time.Since(start).Truncate(time.Second)
	_ = s.FinishSyncRun(ctx, runID, "ok", result.Added, "")

	fmt.Fprintf(cmd.OutOrStdout(), "Repair complete: %d added, %d deleted in %s\n",
		result.Added, result.Deleted, elapsed)
	return nil
}
