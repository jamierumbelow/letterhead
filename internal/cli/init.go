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
