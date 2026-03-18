package cli

import (
	"github.com/jamierumbelow/letterhead/internal/output"
	"github.com/spf13/cobra"
)

type rootOptions struct {
	json  bool
	jsonl bool
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
			return nil
		},
	}

	flags := cmd.PersistentFlags()
	flags.BoolVar(&opts.json, "json", false, "emit structured JSON output")
	flags.BoolVar(&opts.jsonl, "jsonl", false, "emit structured JSONL output")
	cmd.AddCommand(newInitCommand())
	cmd.AddCommand(newStatusCommand())

	return cmd
}

func Execute() {
	cobra.CheckErr(NewRootCommand().Execute())
}
