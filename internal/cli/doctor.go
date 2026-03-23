package cli

import (
	"context"
	"fmt"

	"github.com/jamierumbelow/letterhead/internal/diagnostics"
	"github.com/jamierumbelow/letterhead/internal/store"
	"github.com/jamierumbelow/letterhead/pkg/types"
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

			_, formatter, err := formatterFromCommand(cmd)
			if err != nil {
				return err
			}

			hasFailure := false
			checks := make([]types.DoctorCheckResult, len(results))
			for i, r := range results {
				checks[i] = types.DoctorCheckResult{
					Name:    r.Name,
					Status:  string(r.Status),
					Message: r.Message,
				}
				if r.Status == diagnostics.StatusFail {
					hasFailure = true
				}
			}

			output := types.DoctorOutput{
				OK:     !hasFailure,
				Checks: checks,
			}

			if err := formatter.WriteDoctor(cmd.OutOrStdout(), output); err != nil {
				return err
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
