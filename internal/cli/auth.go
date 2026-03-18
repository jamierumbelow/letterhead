package cli

import (
	"context"
	"fmt"

	"github.com/jamierumbelow/letterhead/internal/auth"
	"github.com/jamierumbelow/letterhead/internal/config"
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

			if cfg.AccountEmail == "" {
				return fmt.Errorf("account_email not set in config; add it to %s", configPathHint())
			}

			if auth.IsAuthenticated(cfg.AccountEmail) {
				fmt.Fprintln(cmd.OutOrStdout(), "Already authenticated as "+cfg.AccountEmail)
				return nil
			}

			result, err := auth.GetClient(context.Background(), cfg.AccountEmail)
			if err != nil {
				return err
			}
			_ = result

			fmt.Fprintln(cmd.OutOrStdout(), "Authenticated as "+cfg.AccountEmail)
			return nil
		},
	}
}

func configPathHint() string {
	p, err := config.ConfigPath()
	if err != nil {
		return "~/.config/letterhead/config.toml"
	}
	return p
}
