package cli

import (
	"context"
	"fmt"

	"github.com/jamierumbelow/letterhead/internal/diagnostics"
	"github.com/jamierumbelow/letterhead/internal/store"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	var fix bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check Letterhead health",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := ensureInitialized()
			if err != nil {
				return err
			}

			ctx := context.Background()

			var s *store.Store
			db, err := store.Open(store.DatabasePath(cfg.ArchiveRoot))
			if err == nil {
				defer db.Close()
				s = store.New(db)
			}

			results := diagnostics.RunAll(ctx, &cfg, s, fix)

			hasFailure := false
			for _, r := range results {
				var icon string
				switch r.Status {
				case diagnostics.StatusPass:
					icon = "ok"
				case diagnostics.StatusWarn:
					icon = "!!"
				case diagnostics.StatusFail:
					icon = "FAIL"
					hasFailure = true
				case diagnostics.StatusSkip:
					icon = "--"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[%4s] %-20s %s\n", icon, r.Name, r.Message)
			}

			if hasFailure {
				return fmt.Errorf("one or more checks failed")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&fix, "fix", false, "Attempt to auto-fix recoverable issues")
	return cmd
}
