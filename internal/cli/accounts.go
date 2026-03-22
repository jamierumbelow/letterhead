package cli

import (
	"bufio"
	"fmt"
	"os"

	"github.com/jamierumbelow/letterhead/internal/config"
	"github.com/spf13/cobra"
)

func newAccountsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accounts",
		Short: "Manage configured Gmail accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newAccountsListCommand())
	cmd.AddCommand(newAccountsAddCommand())

	return cmd
}

func newAccountsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			cfg.MigrateAccounts()

			if len(cfg.Accounts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No accounts configured. Run: letterhead init")
				return nil
			}

			for i, acct := range cfg.Accounts {
				marker := " "
				if acct.Email == cfg.DefaultAccount {
					marker = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s %d. %s (%s)\n", marker, i+1, acct.Email, acct.AuthMethod)
			}

			return nil
		},
	}
}

func newAccountsAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a new Gmail account",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			cfg.MigrateAccounts()

			reader := bufio.NewReader(os.Stdin)
			if err := promptNewAccount(reader, &cfg); err != nil {
				return err
			}

			if err := config.Save(cfg); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintf(cmd.OutOrStdout(), "Account added. You now have %d account(s) configured.\n", len(cfg.Accounts))
			return nil
		},
	}
}
