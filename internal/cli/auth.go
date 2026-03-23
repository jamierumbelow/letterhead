package cli

import (
	"context"

	"github.com/jamierumbelow/letterhead/internal/auth"
	"github.com/jamierumbelow/letterhead/pkg/types"
	"github.com/spf13/cobra"
)

func newAuthCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with Gmail",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := ensureInitialized()
			if err != nil {
				return err
			}

			acct, err := resolveAccount(cmd, cfg)
			if err != nil {
				return err
			}

			_, formatter, err := formatterFromCommand(cmd)
			if err != nil {
				return err
			}

			output := types.AuthOutput{
				Account: acct.Email,
				Method:  string(acct.AuthMethod),
			}

			if auth.IsAuthenticated(acct.Email) {
				output.Authenticated = true
				return formatter.WriteAuth(cmd.OutOrStdout(), output)
			}

			result, err := auth.GetClient(context.Background(), acct.Email)
			if err != nil {
				return NewExitErrorWithHint(ExitAuth, "letterhead auth", "authentication failed: %v", err)
			}
			_ = result

			output.Authenticated = true
			return formatter.WriteAuth(cmd.OutOrStdout(), output)
		},
	}
}
