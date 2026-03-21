package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jamierumbelow/letterhead/internal/scheduler"
	"github.com/spf13/cobra"
)

func newSyncInstallCommand() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install periodic sync scheduler",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := ensureInitialized()
			if err != nil {
				return err
			}

			bin, err := os.Executable()
			if err != nil {
				bin = "letterhead"
			}

			interval := parseCadence(cfg.SchedulerCadence)
			logDir := filepath.Join(cfg.ArchiveRoot, "logs")

			sched := scheduler.New()

			if sched.IsInstalled() {
				fmt.Fprintln(cmd.OutOrStdout(), "Scheduler is already installed.")
				return nil
			}

			scfg := scheduler.Config{
				LetterheadBin: bin,
				IntervalSecs:  interval,
				LogDir:        logDir,
			}

			if !yes {
				fmt.Fprintf(cmd.OutOrStdout(), "Will install periodic sync (every %s).\n", time.Duration(interval)*time.Second)
				fmt.Fprintf(cmd.OutOrStdout(), "Binary: %s\nLogs: %s\n", bin, logDir)
				fmt.Fprint(cmd.OutOrStdout(), "Proceed? [y/N] ")

				var answer string
				fmt.Fscanln(cmd.InOrStdin(), &answer)
				if !strings.HasPrefix(strings.ToLower(answer), "y") {
					fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
					return nil
				}
			}

			if err := sched.Install(scfg); err != nil {
				return fmt.Errorf("install scheduler: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Scheduler installed successfully.")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func newSyncUninstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove periodic sync scheduler",
		RunE: func(cmd *cobra.Command, args []string) error {
			sched := scheduler.New()

			if !sched.IsInstalled() {
				fmt.Fprintln(cmd.OutOrStdout(), "Scheduler is not installed.")
				return nil
			}

			if err := sched.Uninstall(); err != nil {
				return fmt.Errorf("uninstall scheduler: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Scheduler uninstalled successfully.")
			return nil
		},
	}
}

func parseCadence(cadence string) int {
	cadence = strings.TrimSpace(cadence)
	if cadence == "" {
		return 3600
	}

	// Try parsing as a duration like "1h", "30m", "3600s"
	if d, err := time.ParseDuration(cadence); err == nil {
		return int(d.Seconds())
	}

	// Try parsing as plain integer (seconds)
	if secs, err := strconv.Atoi(cadence); err == nil {
		return secs
	}

	return 3600
}
