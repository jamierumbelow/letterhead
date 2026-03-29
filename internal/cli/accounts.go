package cli

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jamierumbelow/letterhead/internal/auth"
	"github.com/jamierumbelow/letterhead/internal/config"
	"github.com/jamierumbelow/letterhead/internal/store"
	"github.com/spf13/cobra"
)

func newAccountsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accounts",
		Short: "Manage configured email accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newAccountsListCommand())
	cmd.AddCommand(newAccountsAddCommand())
	cmd.AddCommand(newAccountsRemoveCommand())
	cmd.AddCommand(newAccountsDefaultCommand())

	return cmd
}

// --- list ---

type accountInfo struct {
	Email          string  `json:"email"`
	AuthMethod     string  `json:"auth_method"`
	Authenticated  bool    `json:"authenticated"`
	MessagesSynced int     `json:"messages_synced"`
	LastSync       *string `json:"last_sync"`
	IsDefault      bool    `json:"is_default"`
}

func newAccountsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("not initialized; run `letterhead init` first")
				}
				return err
			}

			accounts := buildAccountInfos(cfg)

			mode, _, err := formatterFromCommand(cmd)
			if err != nil {
				return err
			}

			if mode != "human" {
				return writeAccountsJSON(cmd.OutOrStdout(), accounts)
			}

			return writeAccountsTable(cmd.OutOrStdout(), accounts)
		},
	}
}

func buildAccountInfos(cfg config.Config) []accountInfo {
	// Try to open DB for sync state
	var s *store.Store
	var db *sql.DB
	dbPath := store.DatabasePath(cfg.ArchiveRoot)
	db, err := store.Open(dbPath)
	if err == nil {
		s = store.New(db)
		defer db.Close()
	}

	infos := make([]accountInfo, 0, len(cfg.Accounts))
	for _, acct := range cfg.Accounts {
		info := accountInfo{
			Email:      acct.Email,
			AuthMethod: string(acct.AuthMethod),
			IsDefault:  strings.EqualFold(acct.Email, cfg.DefaultAccount),
		}

		// Check authentication
		switch acct.AuthMethod {
		case config.AuthMethodOAuth:
			info.Authenticated = auth.IsAuthenticated(acct.Email)
		case config.AuthMethodAppPassword:
			p, perr := config.AppPasswordPath(acct.Email)
			if perr == nil {
				_, serr := os.Stat(p)
				info.Authenticated = serr == nil
			}
		}

		// Check sync state
		if s != nil {
			ctx := context.Background()
			syncState, serr := s.GetSyncState(ctx, acct.Email)
			if serr == nil {
				info.MessagesSynced = syncState.MessagesSynced
				if syncState.LastSyncAt != nil {
					t := syncState.LastSyncAt.Format(time.RFC3339)
					info.LastSync = &t
				}
			}
		}

		infos = append(infos, info)
	}

	return infos
}

func writeAccountsTable(w io.Writer, accounts []accountInfo) error {
	if len(accounts) == 0 {
		_, err := fmt.Fprintln(w, "No accounts configured. Run `letterhead accounts add` to add one.")
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	fmt.Fprintln(tw, "EMAIL\tAUTH METHOD\tAUTHENTICATED\tMESSAGES SYNCED\tLAST SYNC")
	for _, a := range accounts {
		authd := "no"
		if a.Authenticated {
			authd = "yes"
		}

		lastSync := "never"
		if a.LastSync != nil {
			lastSync = *a.LastSync
		}

		email := a.Email
		if a.IsDefault {
			email += " (default)"
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n",
			email, a.AuthMethod, authd, a.MessagesSynced, lastSync)
	}

	return tw.Flush()
}

// --- add ---

func newAccountsAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add [email]",
		Short: "Add a new email account interactively",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("not initialized; run `letterhead init` first")
				}
				return err
			}

			reader := bufio.NewReader(os.Stdin)

			// Prompt for email (or use provided arg)
			var email string
			if len(args) > 0 {
				email = strings.TrimSpace(args[0])
				fmt.Printf("Email address: %s\n", email)
			} else {
				fmt.Print("Email address: ")
				email, err = reader.ReadString('\n')
				if err != nil && !errors.Is(err, io.EOF) {
					return err
				}
				email = strings.TrimSpace(email)
			}
			if email == "" {
				return fmt.Errorf("email is required")
			}

			// Check for duplicate
			if cfg.AccountByEmail(email) != nil {
				return fmt.Errorf("account %q is already configured", email)
			}

			// Prompt for auth method
			fmt.Println()
			fmt.Println("Auth method:")
			fmt.Println("  1. App password (recommended)")
			fmt.Println("  2. Google OAuth")
			fmt.Print("Choice [1]: ")
			choice, err := reader.ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				return err
			}
			choice = strings.TrimSpace(choice)

			authMethod := config.AuthMethodAppPassword
			if choice == "2" {
				authMethod = config.AuthMethodOAuth
				fmt.Println()
				fmt.Println("Google OAuth will open a browser window to sign in and grant")
				fmt.Println("Letterhead read-only access to your Gmail.")
				fmt.Println()
				fmt.Println("After adding the account, run `letterhead auth` to complete")
				fmt.Println("the sign-in flow.")
				fmt.Println()
				fmt.Println("If you need to supply your own OAuth credentials, place a")
				fmt.Println("credentials.json from Google Cloud Console in ~/.config/letterhead/")
				fmt.Println("or set LETTERHEAD_CLIENT_ID and LETTERHEAD_CLIENT_SECRET.")
			}

			// If app password, show instructions and prompt for it
			if authMethod == config.AuthMethodAppPassword {
				fmt.Println()
				fmt.Println("To create a Gmail app password:")
				fmt.Println("  1. Go to https://myaccount.google.com/security")
				fmt.Println("  2. Under 'How you sign in to Google', open 2-Step Verification")
				fmt.Println("     (you must have 2FA enabled)")
				fmt.Println("  3. Scroll to the bottom and click 'App passwords'")
				fmt.Println("  4. Enter a name (e.g. 'Letterhead') and click Create")
				fmt.Println("  5. Copy the 16-character password shown")
				fmt.Println()
				fmt.Print("App password: ")
				appPass, err := reader.ReadString('\n')
				if err != nil && !errors.Is(err, io.EOF) {
					return err
				}
				appPass = strings.TrimSpace(appPass)
				appPass = strings.ReplaceAll(appPass, " ", "")

				if appPass != "" {
					appPassPath, err := config.AppPasswordPath(email)
					if err != nil {
						return err
					}

					if err := os.MkdirAll(filepath.Dir(appPassPath), 0o700); err != nil {
						return err
					}

					if err := os.WriteFile(appPassPath, []byte(appPass), 0o600); err != nil {
						return err
					}
				}
			}

			cfg.Accounts = append(cfg.Accounts, config.AccountConfig{
				Email:      email,
				AuthMethod: authMethod,
			})

			if err := config.Save(cfg); err != nil {
				return err
			}

			fmt.Printf("\nAccount %q added successfully.\n", email)
			return nil
		},
	}
}

// --- remove ---

func newAccountsRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <email>",
		Aliases: []string{"rm"},
		Short:   "Remove a configured account",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			email := args[0]

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			acct := cfg.AccountByEmail(email)
			if acct == nil {
				return fmt.Errorf("account %q not found", email)
			}

			reader := bufio.NewReader(os.Stdin)

			// Confirm removal
			fmt.Printf("Remove account %q? [y/N] ", acct.Email)
			confirm, err := reader.ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				return err
			}
			if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
				fmt.Println("Cancelled.")
				return nil
			}

			// Optionally purge messages
			dbPath := store.DatabasePath(cfg.ArchiveRoot)
			db, dbErr := store.Open(dbPath)
			if dbErr == nil {
				defer db.Close()

				ctx := context.Background()

				// Count messages for this account
				var msgCount int
				err := db.QueryRowContext(ctx,
					`SELECT COUNT(*) FROM messages WHERE account_id = ?`, acct.Email).Scan(&msgCount)
				if err == nil && msgCount > 0 {
					fmt.Printf("Also delete %d synced messages for this account? [y/N] ", msgCount)
					purge, err := reader.ReadString('\n')
					if err != nil && !errors.Is(err, io.EOF) {
						return err
					}
					if strings.ToLower(strings.TrimSpace(purge)) == "y" {
						if _, err := db.ExecContext(ctx, `DELETE FROM message_labels WHERE account_id = ?`, acct.Email); err != nil {
							return fmt.Errorf("purge labels: %w", err)
						}
						if _, err := db.ExecContext(ctx, `DELETE FROM message_recipients WHERE account_id = ?`, acct.Email); err != nil {
							return fmt.Errorf("purge recipients: %w", err)
						}
						if _, err := db.ExecContext(ctx, `DELETE FROM messages WHERE account_id = ?`, acct.Email); err != nil {
							return fmt.Errorf("purge messages: %w", err)
						}
						if _, err := db.ExecContext(ctx, `DELETE FROM sync_state WHERE account_id = ?`, acct.Email); err != nil {
							return fmt.Errorf("purge sync state: %w", err)
						}
						fmt.Printf("Deleted %d messages.\n", msgCount)
					}
				}
			}

			// Remove from config
			newAccounts := make([]config.AccountConfig, 0, len(cfg.Accounts)-1)
			for _, a := range cfg.Accounts {
				if !strings.EqualFold(a.Email, acct.Email) {
					newAccounts = append(newAccounts, a)
				}
			}
			cfg.Accounts = newAccounts

			// Clear default if it was this account
			if strings.EqualFold(cfg.DefaultAccount, acct.Email) {
				cfg.DefaultAccount = ""
			}

			if err := config.Save(cfg); err != nil {
				return err
			}

			fmt.Printf("Account %q removed.\n", acct.Email)
			return nil
		},
	}
}

// --- default ---

func newAccountsDefaultCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "default <email>",
		Short: "Set the default account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			email := args[0]

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			acct := cfg.AccountByEmail(email)
			if acct == nil {
				return fmt.Errorf("account %q not found", email)
			}

			cfg.DefaultAccount = acct.Email

			if err := config.Save(cfg); err != nil {
				return err
			}

			fmt.Printf("Default account set to %q.\n", acct.Email)
			return nil
		},
	}
}

func writeAccountsJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
