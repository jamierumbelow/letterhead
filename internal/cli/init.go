package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jamierumbelow/letterhead/internal/config"
	"github.com/jamierumbelow/letterhead/internal/output"
	"github.com/jamierumbelow/letterhead/internal/store"
	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	var archiveRoot string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the local Letterhead archive",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, created, err := initializeArchive(cmd, archiveRoot)
			if err != nil {
				return err
			}

			mode, formatter, err := formatterFromCommand(cmd)
			if err != nil {
				return err
			}

			if mode == output.ModeHuman {
				if created {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Letterhead initialized."); err != nil {
						return err
					}
					if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
						return err
					}
				} else {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Letterhead is already initialized."); err != nil {
						return err
					}
					if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
						return err
					}
				}
			}

			if err := formatter.WriteStatus(cmd.OutOrStdout(), phaseZeroStatusOutput(cfg, "ok")); err != nil {
				return err
			}

			if mode == output.ModeHuman {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "\nNext steps:\n  letterhead status")
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&archiveRoot, "archive-root", "", "archive root for the local mail mirror")

	return cmd
}

func initializeArchive(cmd *cobra.Command, archiveRootFlag string) (config.Config, bool, error) {
	cfg, err := config.Load()
	created := false

	switch {
	case err == nil:
		// Already initialized; make the command idempotent by ensuring the archive
		// directory and schema still exist.
	case errors.Is(err, os.ErrNotExist):
		cfg, err = config.Default()
		if err != nil {
			return config.Config{}, false, err
		}

		if archiveRootFlag == "" {
			archiveRootFlag, err = promptArchiveRoot(cmd, cfg.ArchiveRoot)
			if err != nil {
				return config.Config{}, false, err
			}
		}

		if archiveRootFlag != "" {
			cfg.ArchiveRoot = archiveRootFlag
		}

		created = true
	default:
		return config.Config{}, false, err
	}

	if archiveRootFlag != "" && created {
		cfg.ArchiveRoot = archiveRootFlag
	}

	if err := os.MkdirAll(cfg.ArchiveRoot, 0o700); err != nil {
		return config.Config{}, false, err
	}

	if err := config.Save(cfg); err != nil {
		return config.Config{}, false, err
	}

	db, err := store.Open(store.DatabasePath(cfg.ArchiveRoot))
	if err != nil {
		return config.Config{}, false, err
	}
	defer db.Close()

	return cfg, created, nil
}

func formatterFromCommand(cmd *cobra.Command) (output.Mode, output.Formatter, error) {
	asJSON, err := cmd.Flags().GetBool("json")
	if err != nil {
		return "", nil, err
	}

	asJSONL, err := cmd.Flags().GetBool("jsonl")
	if err != nil {
		return "", nil, err
	}

	mode, err := output.ModeFromFlags(asJSON, asJSONL)
	if err != nil {
		return "", nil, err
	}

	formatter, err := output.NewFormatter(mode)
	if err != nil {
		return "", nil, err
	}

	return mode, formatter, nil
}

// ensureInitialized loads the config, or runs a first-time setup wizard
// if no config exists. In a TTY it prompts interactively; otherwise it
// uses defaults silently.
func ensureInitialized() (config.Config, error) {
	cfg, err := config.Load()
	firstRun := false

	if errors.Is(err, os.ErrNotExist) {
		// No config file yet
		firstRun = true
		cfg, err = config.Default()
		if err != nil {
			return config.Config{}, err
		}
	} else if err != nil {
		return config.Config{}, err
	}

	// Migrate legacy single-account config
	

	// Run wizard if first run, or if no accounts configured
	needsWizard := firstRun || (cfg.AccountEmail == "" && len(cfg.Accounts) == 0)
	if needsWizard && isTTY() {
		cfg, err = runSetupWizard(cfg)
		if err != nil {
			return config.Config{}, err
		}

		if err := os.MkdirAll(cfg.ArchiveRoot, 0o700); err != nil {
			return config.Config{}, err
		}

		if err := config.Save(cfg); err != nil {
			return config.Config{}, err
		}
	} else if firstRun {
		// Non-TTY first run: save defaults silently
		if err := os.MkdirAll(cfg.ArchiveRoot, 0o700); err != nil {
			return config.Config{}, err
		}

		if err := config.Save(cfg); err != nil {
			return config.Config{}, err
		}
	}

	// Ensure DB exists
	db, err := store.Open(store.DatabasePath(cfg.ArchiveRoot))
	if err != nil {
		return config.Config{}, err
	}
	db.Close()

	return cfg, nil
}

func runSetupWizard(cfg config.Config) (config.Config, error) {
	reader := bufio.NewReader(os.Stdin)

	// Migrate legacy config so we can show existing accounts
	

	if len(cfg.Accounts) > 0 {
		// Re-running init with existing accounts
		fmt.Printf("You have %d account(s) configured:\n", len(cfg.Accounts))
		for i, acct := range cfg.Accounts {
			fmt.Printf("  %d. %s (%s)\n", i+1, acct.Email, acct.AuthMethod)
		}
		fmt.Print("Add another account? [y/N]: ")
		answer, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return cfg, err
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(answer)), "y") {
			if err := promptNewAccount(reader, &cfg); err != nil {
				return cfg, err
			}
		}

		configPath, _ := config.ConfigPath()
		fmt.Println()
		fmt.Printf("Config saved to %s\n", configPath)
		fmt.Println()
		return cfg, nil
	}

	// First run: no accounts yet
	fmt.Println("Welcome to letterhead! Let's get you set up.")
	fmt.Println()

	// Archive root (has a sensible default)
	fmt.Printf("Archive location [%s]: ", cfg.ArchiveRoot)
	root, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return cfg, err
	}
	root = strings.TrimSpace(root)
	if root != "" {
		cfg.ArchiveRoot = root
	}

	// Prompt for first account
	if err := promptNewAccount(reader, &cfg); err != nil {
		return cfg, err
	}

	// Set legacy fields from first account for backward compat
	if len(cfg.Accounts) > 0 {
		cfg.AccountEmail = cfg.Accounts[0].Email
		cfg.AuthMethod = cfg.Accounts[0].AuthMethod
		cfg.DefaultAccount = cfg.Accounts[0].Email
	}

	configPath, _ := config.ConfigPath()
	fmt.Println()
	fmt.Printf("Config saved to %s\n", configPath)
	fmt.Println()
	fmt.Println("Next: letterhead sync")
	fmt.Println()

	return cfg, nil
}

// promptNewAccount prompts the user for email, auth method, and credentials,
// then appends a new Account to cfg.Accounts. This is shared between the
// setup wizard and the `accounts add` command.
func promptNewAccount(reader *bufio.Reader, cfg *config.Config) error {
	fmt.Print("Gmail address: ")
	email, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email is required")
	}

	// Check for duplicate
	if cfg.AccountByEmail(email) != nil {
		return fmt.Errorf("account %s is already configured", email)
	}

	// Auth method choice
	fmt.Println()
	fmt.Println("Auth method:")
	fmt.Println("  1. App password (recommended - quick setup)")
	fmt.Println("  2. Google OAuth (requires a Google Cloud project)")
	fmt.Print("Choice [1]: ")
	choice, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	choice = strings.TrimSpace(choice)

	configPath, _ := config.ConfigPath()

	acct := config.AccountConfig{Email: email}

	if choice == "2" {
		acct.AuthMethod = config.AuthMethodOAuth
		fmt.Println()
		fmt.Println("To authenticate, you need Google OAuth credentials:")
		fmt.Println("  1. Go to https://console.cloud.google.com")
		fmt.Println("  2. Create a project and enable the Gmail API")
		fmt.Println("  3. Create an OAuth client ID (Desktop app)")
		fmt.Println("  4. Download the JSON and save it as:")
		fmt.Printf("     %s/credentials.json\n", filepath.Dir(configPath))
		fmt.Println()
		fmt.Println("Then run: letterhead sync")
	} else {
		acct.AuthMethod = config.AuthMethodAppPassword
		fmt.Println()
		fmt.Println("To create an app password:")
		fmt.Println("  1. Go to https://myaccount.google.com/apppasswords")
		fmt.Println("     (requires 2-step verification to be enabled)")
		fmt.Println("  2. Enter 'letterhead' as the app name")
		fmt.Println("  3. Copy the 16-character password")
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

			fmt.Println("Credentials saved.")
		}
	}

	cfg.Accounts = append(cfg.Accounts, acct)

	// Set default account if this is the first one
	if cfg.DefaultAccount == "" {
		cfg.DefaultAccount = email
	}

	return nil
}

func isTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func promptArchiveRoot(cmd *cobra.Command, defaultArchiveRoot string) (string, error) {
	if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "Archive root [%s]: ", defaultArchiveRoot); err != nil {
		return "", err
	}

	reader := bufio.NewReader(cmd.InOrStdin())
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultArchiveRoot, nil
	}

	return trimmed, nil
}
