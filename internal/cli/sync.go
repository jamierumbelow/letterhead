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
	"github.com/jamierumbelow/letterhead/internal/output"
	"github.com/jamierumbelow/letterhead/internal/store"
	"github.com/jamierumbelow/letterhead/internal/syncer"
	"github.com/jamierumbelow/letterhead/pkg/types"
	"github.com/spf13/cobra"
)

func newSyncCommand() *cobra.Command {
	var (
		repair bool
		all    bool
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync messages from Gmail or IMAP",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := ensureInitialized()
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

			_, formatter, err := formatterFromCommand(cmd)
			if err != nil {
				return err
			}

			if all {
				return syncAllAccounts(ctx, cmd, cfg, s, repair, formatter)
			}

			acct, err := resolveAccount(cmd, cfg)
			if err != nil {
				return err
			}

			return syncOneAccount(ctx, cmd, cfg, s, acct, repair, false, formatter)
		},
	}

	cmd.Flags().BoolVar(&repair, "repair", false, "Force a repair sync (re-diff against Gmail)")
	cmd.Flags().BoolVar(&all, "all", false, "Sync all configured accounts")

	cmd.AddCommand(newSyncInstallCommand())
	cmd.AddCommand(newSyncUninstallCommand())

	return cmd
}

func syncAllAccounts(ctx context.Context, cmd *cobra.Command, cfg config.Config, s *store.Store, repair bool, formatter output.Formatter) error {
	if len(cfg.Accounts) == 0 {
		return fmt.Errorf("no accounts configured")
	}

	multi := len(cfg.Accounts) > 1
	var syncErrors []string

	for i := range cfg.Accounts {
		acct := &cfg.Accounts[i]
		if multi {
			fmt.Fprintf(cmd.ErrOrStderr(), "\n[%s] Starting sync...\n", acct.Email)
		}

		if err := syncOneAccount(ctx, cmd, cfg, s, acct, repair, multi, formatter); err != nil {
			errMsg := fmt.Sprintf("%s: %v", acct.Email, err)
			syncErrors = append(syncErrors, errMsg)
			if multi {
				fmt.Fprintf(cmd.ErrOrStderr(), "[%s] Error: %v\n", acct.Email, err)
			}
			continue
		}
	}

	if multi && len(syncErrors) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "\nSync completed with errors:\n")
		for _, e := range syncErrors {
			fmt.Fprintf(cmd.ErrOrStderr(), "  - %s\n", e)
		}
		return fmt.Errorf("%d of %d accounts failed to sync", len(syncErrors), len(cfg.Accounts))
	}

	return nil
}

func syncOneAccount(ctx context.Context, cmd *cobra.Command, cfg config.Config, s *store.Store, acct *config.AccountConfig, repair bool, prefixOutput bool, formatter output.Formatter) error {
	prefix := ""
	if prefixOutput {
		prefix = fmt.Sprintf("[%s] ", acct.Email)
	}

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
		fmt.Fprintf(cmd.ErrOrStderr(), "%sConnected via IMAP.\n", prefix)

	default: // oauth
		result, err := auth.GetClient(ctx, acct.Email)
		if err != nil {
			return err
		}

		if result.Method == auth.AuthMethodInteractive {
			fmt.Fprintf(cmd.ErrOrStderr(), "%sAuthenticated successfully.\n", prefix)
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
			return runRepairSync(ctx, cmd, gmailClient, s, acct.Email, prefix, formatter)
		}

		if bootstrapComplete {
			return runIncrementalSync(ctx, cmd, gmailClient, s, acct.Email, prefix, formatter)
		}
	}

	syncMode := acct.SyncMode
	if syncMode == "" {
		syncMode = cfg.SyncMode
	}
	query := syncer.QueryForMode(string(syncMode), cfg.RecentWindowWeeks)
	if syncMode == "full" {
		fmt.Fprintf(cmd.ErrOrStderr(), "%sFull sync mode: this may take a long time for large mailboxes.\n", prefix)
	}
	return runBootstrapSync(ctx, cmd, adapter, s, acct.Email, query, prefix, formatter)
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

func runBootstrapSync(ctx context.Context, cmd *cobra.Command, client mailclient.MailClient, s *store.Store, email string, query string, prefix string, formatter output.Formatter) error {
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
		fmt.Fprintf(cmd.ErrOrStderr(), "\r%sSynced %d messages (%s)", prefix, synced, elapsed)
	}

	bcfg := syncer.DefaultBootstrapConfig(email)
	bcfg.Query = query
	err = syncer.Bootstrap(ctx, client, s, bcfg, progress)

	fmt.Fprintln(cmd.ErrOrStderr()) // newline after progress

	if err != nil {
		_ = s.FinishSyncRun(ctx, runID, "error", totalSynced, err.Error())
		return err
	}

	elapsed := time.Since(start)
	_ = s.FinishSyncRun(ctx, runID, "ok", totalSynced, "")

	return formatter.WriteSync(cmd.OutOrStdout(), types.SyncOutput{
		Account:   email,
		Mode:      "bootstrap",
		Added:     totalSynced,
		ElapsedMS: elapsed.Milliseconds(),
	})
}

func runIncrementalSync(ctx context.Context, cmd *cobra.Command, client *gmail.Client, s *store.Store, email string, prefix string, formatter output.Formatter) error {
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
			fmt.Fprintf(cmd.ErrOrStderr(), "%sHistory expired. Running repair sync...\n", prefix)
			_ = s.FinishSyncRun(ctx, runID, "expired", 0, "history expired, falling back to repair")
			return runRepairSync(ctx, cmd, client, s, email, prefix, formatter)
		}
		_ = s.FinishSyncRun(ctx, runID, "error", 0, err.Error())
		return err
	}

	elapsed := time.Since(start)
	_ = s.FinishSyncRun(ctx, runID, "ok", result.Added, "")

	return formatter.WriteSync(cmd.OutOrStdout(), types.SyncOutput{
		Account:   email,
		Mode:      "incremental",
		Added:     result.Added,
		Deleted:   result.Deleted,
		Labels:    result.Labels,
		ElapsedMS: elapsed.Milliseconds(),
	})
}

func runRepairSync(ctx context.Context, cmd *cobra.Command, client *gmail.Client, s *store.Store, email string, prefix string, formatter output.Formatter) error {
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
		fmt.Fprintf(cmd.ErrOrStderr(), "\r%sRepair: fetched %d messages (%s)", prefix, synced, elapsed)
	}

	result, err := syncer.RepairSync(ctx, client, s, email, "label:INBOX", progress)

	fmt.Fprintln(cmd.ErrOrStderr()) // newline after progress

	if err != nil {
		_ = s.FinishSyncRun(ctx, runID, "error", 0, err.Error())
		return err
	}

	elapsed := time.Since(start)
	_ = s.FinishSyncRun(ctx, runID, "ok", result.Added, "")

	return formatter.WriteSync(cmd.OutOrStdout(), types.SyncOutput{
		Account:   email,
		Mode:      "repair",
		Added:     result.Added,
		Deleted:   result.Deleted,
		ElapsedMS: elapsed.Milliseconds(),
	})
}
