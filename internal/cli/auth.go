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
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("not initialized (run letterhead init first): %w", err)
			}

			if cfg.AccountEmail == "" {
				return fmt.Errorf("account_email not set in config; add it to %s", configPathHint())
			}

			oc, err := auth.LoadOAuthConfig(cfg.AccountEmail)
			if err != nil {
				return err
			}

			if oc.HasToken() {
				fmt.Fprintln(cmd.OutOrStdout(), "Already authenticated as "+cfg.AccountEmail)
				return nil
			}

			fmt.Fprintln(cmd.ErrOrStderr(), "Opening browser to authenticate...")

			_, err = oc.Authenticate(context.Background())
			if err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}

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
