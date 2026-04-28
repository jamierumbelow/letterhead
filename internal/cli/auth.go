package cli

import (
	"context"

	"github.com/jamierumbelow/letterhead/internal/auth"
	"github.com/jamierumbelow/letterhead/pkg/types"
	"github.com/spf13/cobra"
)

func newAuthCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
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

			if !force && auth.IsAuthenticated(acct.Email) {
				output.Authenticated = true
				return formatter.WriteAuth(cmd.OutOrStdout(), output)
			}

			ctx := context.Background()

			if force {
				oc, err := auth.LoadOAuthConfig(acct.Email)
				if err != nil {
					return NewExitErrorWithHint(ExitAuth, "letterhead auth", "authentication failed: %v", err)
				}
				if _, err := oc.Authenticate(ctx); err != nil {
					return NewExitErrorWithHint(ExitAuth, "letterhead auth", "authentication failed: %v", err)
				}
			} else {
				if _, err := auth.GetClient(ctx, acct.Email); err != nil {
					return NewExitErrorWithHint(ExitAuth, "letterhead auth", "authentication failed: %v", err)
				}
			}

			output.Authenticated = true
			return formatter.WriteAuth(cmd.OutOrStdout(), output)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force interactive re-authentication, replacing any stored token")

	return cmd
}
