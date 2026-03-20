package cli

import (
	"context"
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
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync messages from Gmail or IMAP",
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

			// Create the appropriate mail client based on auth method
			var adapter mailclient.MailClient

			switch cfg.AuthMethod {
			case config.AuthMethodAppPassword:
				password, err := loadAppPassword(cfg.AccountEmail)
				if err != nil {
					return err
				}

				imapClient := imapclient.New(cfg.AccountEmail, password)
				if err := imapClient.Connect(ctx); err != nil {
					return err
				}
				defer imapClient.Close()

				adapter = mailclient.NewIMAPAdapter(imapClient, cfg.AccountEmail)
				fmt.Fprintln(cmd.ErrOrStderr(), "Connected via IMAP.")

			default: // oauth
				result, err := auth.GetClient(ctx, cfg.AccountEmail)
				if err != nil {
					return err
				}

				if result.Method == auth.AuthMethodInteractive {
					fmt.Fprintln(cmd.ErrOrStderr(), "Authenticated successfully.")
				}

				gmailClient, err := gmail.NewClient(ctx, result.Client)
				if err != nil {
					return err
				}

				adapter = mailclient.NewGmailAdapter(gmailClient)
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
			err = syncer.Bootstrap(ctx, adapter, s, bcfg, progress)

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
