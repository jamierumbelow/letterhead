package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
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
	if err == nil {
		// Already initialized -- ensure DB exists
		db, dbErr := store.Open(store.DatabasePath(cfg.ArchiveRoot))
		if dbErr != nil {
			return config.Config{}, dbErr
		}
		db.Close()
		return cfg, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return config.Config{}, err
	}

	// First run
	cfg, err = config.Default()
	if err != nil {
		return config.Config{}, err
	}

	if isTTY() {
		cfg, err = runSetupWizard(cfg)
		if err != nil {
			return config.Config{}, err
		}
	}

	if err := os.MkdirAll(cfg.ArchiveRoot, 0o700); err != nil {
		return config.Config{}, err
	}

	if err := config.Save(cfg); err != nil {
		return config.Config{}, err
	}

	db, err := store.Open(store.DatabasePath(cfg.ArchiveRoot))
	if err != nil {
		return config.Config{}, err
	}
	db.Close()

	return cfg, nil
}

func runSetupWizard(cfg config.Config) (config.Config, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Welcome to letterhead! Let's get you set up.")
	fmt.Println()

	// Account email (required for sync)
	fmt.Print("Gmail address: ")
	email, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return cfg, err
	}
	email = strings.TrimSpace(email)
	if email != "" {
		cfg.AccountEmail = email
	}

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

	fmt.Println()
	return cfg, nil
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
