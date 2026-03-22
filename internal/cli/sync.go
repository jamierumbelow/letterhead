package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jamierumbelow/letterhead/internal/auth"
	"github.com/jamierumbelow/letterhead/internal/config"
	"github.com/jamierumbelow/letterhead/internal/gmail"
	"github.com/jamierumbelow/letterhead/internal/imapclient"
	"github.com/jamierumbelow/letterhead/internal/mailclient"
	"github.com/jamierumbelow/letterhead/internal/store"
	"github.com/jamierumbelow/letterhead/internal/syncer"
	"github.com/spf13/cobra"
)

func newSyncCommand() *cobra.Command {
	var repair bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync messages from Gmail or IMAP",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := ensureInitialized()
			if err != nil {
				return err
			}

			acct, err := resolveAccount(cmd, cfg)
			if err != nil {
				return err
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

			// Create the appropriate mail client based on auth method
			var adapter mailclient.MailClient
			var gmailClient *gmail.Client

			switch acct.AuthMethod {
			case config.AuthMethodAppPassword:
				password, err := loadAppPassword(acct.Email)
				if err != nil {
					return err
				}

				imapClient := imapclient.New(acct.Email, password)
				if err := imapClient.Connect(ctx); err != nil {
					return err
				}
				defer imapClient.Close()

				adapter = mailclient.NewIMAPAdapter(imapClient, acct.Email)
				fmt.Fprintln(cmd.ErrOrStderr(), "Connected via IMAP.")

			default: // oauth
				result, err := auth.GetClient(ctx, acct.Email)
				if err != nil {
					return err
				}

				if result.Method == auth.AuthMethodInteractive {
					fmt.Fprintln(cmd.ErrOrStderr(), "Authenticated successfully.")
				}

				gmailClient, err = gmail.NewClient(ctx, result.Client)
				if err != nil {
					return err
				}

				adapter = mailclient.NewGmailAdapter(gmailClient)
			}

			// Check sync state to decide bootstrap vs incremental vs repair
			syncState, _ := s.GetSyncState(ctx, acct.Email)
			bootstrapComplete := syncState != nil && syncState.BootstrapComplete

			// Incremental and repair sync only work with Gmail API
			if gmailClient != nil {
				if repair {
					return runRepairSync(ctx, cmd, gmailClient, s, acct.Email)
				}

				if bootstrapComplete {
					return runIncrementalSync(ctx, cmd, gmailClient, s, acct.Email)
				}
			}

			syncMode := acct.SyncMode
			if syncMode == "" {
				syncMode = cfg.SyncMode
			}
			query := syncer.QueryForMode(string(syncMode), cfg.RecentWindowWeeks)
			if syncMode == "full" {
				fmt.Fprintln(cmd.ErrOrStderr(), "Full sync mode: this may take a long time for large mailboxes.")
			}
			return runBootstrapSync(ctx, cmd, adapter, s, acct.Email, query)
		},
	}

	cmd.Flags().BoolVar(&repair, "repair", false, "Force a repair sync (re-diff against Gmail)")

	cmd.AddCommand(newSyncInstallCommand())
	cmd.AddCommand(newSyncUninstallCommand())

	return cmd
}

func loadAppPassword(accountEmail string) (string, error) {
	path, err := config.AppPasswordPath(accountEmail)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("no app password found at %s; run letterhead init to set up", path)
	}

	password := strings.TrimSpace(string(data))
	if password == "" {
		return "", fmt.Errorf("app password file is empty at %s", path)
	}

	return password, nil
}

func runBootstrapSync(ctx context.Context, cmd *cobra.Command, client mailclient.MailClient, s *store.Store, email string, query string) error {
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
	bcfg.Query = query
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
