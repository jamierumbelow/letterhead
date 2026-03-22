package cli

import (
	"github.com/jamierumbelow/letterhead/internal/output"
	"github.com/spf13/cobra"
)

type rootOptions struct {
	json    bool
	jsonl   bool
	account string
}

func NewRootCommand() *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:           "letterhead",
		Short:         "Local-first Gmail mirror for humans and agents",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			_, err := output.ModeFromFlags(opts.json, opts.jsonl)
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	flags := cmd.PersistentFlags()
	flags.BoolVar(&opts.json, "json", false, "emit structured JSON output")
	flags.BoolVar(&opts.jsonl, "jsonl", false, "emit structured JSONL output")
	flags.StringVar(&opts.account, "account", "", "operate on a specific account email")
	cmd.AddCommand(newInitCommand())
	cmd.AddCommand(newAuthCommand())
	cmd.AddCommand(newStatusCommand())
	cmd.AddCommand(newReadCommand())
	cmd.AddCommand(newFindCommand())
	cmd.AddCommand(newSyncCommand())
	cmd.AddCommand(newDoctorCommand())
	cmd.AddCommand(newRebuildCommand())

	return cmd
}

func Execute() {
	cobra.CheckErr(NewRootCommand().Execute())
}
