package cli

import (
	"github.com/jamierumbelow/letterhead/internal/config"
	"github.com/spf13/cobra"
)

// resolveAccount determines which account to operate on, using the
// --account persistent flag if provided, otherwise falling back to
// the config's default/sole account resolution.
func resolveAccount(cmd *cobra.Command, cfg config.Config) (*config.AccountConfig, error) {
	accountFlag, _ := cmd.Flags().GetString("account")
	return cfg.ResolveAccount(accountFlag)
}
