package cli

import (
	"encoding/json"
	"os"

	"github.com/jamierumbelow/letterhead/internal/output"
	"github.com/jamierumbelow/letterhead/pkg/types"
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
			// In JSON mode (explicit or auto-detected), emit compact help
			autoJSON := cmd.OutOrStdout() == os.Stdout && !IsStdoutTTY()
			if opts.json || opts.jsonl || autoJSON {
				return writeCompactHelp(cmd)
			}
			return cmd.Help()
		},
	}

	flags := cmd.PersistentFlags()
	flags.BoolVar(&opts.json, "json", false, "emit structured JSON output")
	flags.BoolVar(&opts.jsonl, "jsonl", false, "emit structured JSONL output")
	flags.StringVar(&opts.account, "account", "", "operate on a specific account email")
	cmd.AddCommand(newAccountsCommand())
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

func writeCompactHelp(cmd *cobra.Command) error {
	var cmds []types.HelpCommand
	for _, sub := range cmd.Commands() {
		if sub.Hidden || !sub.IsAvailableCommand() {
			continue
		}
		cmds = append(cmds, types.HelpCommand{
			Name:  sub.Name(),
			Short: sub.Short,
			Usage: sub.UseLine(),
		})
	}

	help := types.HelpOutput{
		Commands: cmds,
		Flags:    []string{"--json", "--jsonl", "--account <email>"},
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)
	return enc.Encode(help)
}
