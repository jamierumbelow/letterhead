package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

var errConflictingOutputModes = errors.New("--json and --jsonl cannot be used together")

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
			if opts.json && opts.jsonl {
				return errConflictingOutputModes
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	flags := cmd.PersistentFlags()
	flags.BoolVar(&opts.json, "json", false, "emit structured JSON output")
	flags.BoolVar(&opts.jsonl, "jsonl", false, "emit structured JSONL output")

	return cmd
}

func Execute() {
	cobra.CheckErr(NewRootCommand().Execute())
}
