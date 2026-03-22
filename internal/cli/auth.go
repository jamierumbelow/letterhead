package cli

import (
	"context"
	"fmt"

	"github.com/jamierumbelow/letterhead/internal/auth"
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

			if auth.IsAuthenticated(acct.Email) {
				fmt.Fprintln(cmd.OutOrStdout(), "Already authenticated as "+acct.Email)
				return nil
			}

			result, err := auth.GetClient(context.Background(), acct.Email)
			if err != nil {
				return err
			}
			_ = result

			fmt.Fprintln(cmd.OutOrStdout(), "Authenticated as "+acct.Email)
			return nil
		},
	}
}
