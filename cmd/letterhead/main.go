package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/jamierumbelow/letterhead/internal/cli"
	"github.com/jamierumbelow/letterhead/pkg/types"
)

func main() {
	cmd := cli.NewRootCommand()
	err := cmd.Execute()
	if err == nil {
		os.Exit(cli.ExitOK)
	}

	// Extract exit code from ExitError, default to 1
	var exitErr *cli.ExitError
	code := cli.ExitUsage
	hint := ""
	if errors.As(err, &exitErr) {
		code = exitErr.Code
		hint = exitErr.Hint
	}

	// If --json or --jsonl was requested, emit structured error
	asJSON, _ := cmd.PersistentFlags().GetBool("json")
	asJSONL, _ := cmd.PersistentFlags().GetBool("jsonl")

	if asJSON || asJSONL || !cli.IsStdoutTTY() {
		errOut := types.ErrorOutput{
			OK: false,
			Error: types.ErrorInfo{
				Code:     cli.ExitCodeName(code),
				ExitCode: code,
				Message:  err.Error(),
				Hint:     hint,
			},
		}
		enc := json.NewEncoder(os.Stderr)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(errOut)
	} else {
		fmt.Fprintln(os.Stderr, "Error:", err)
	}

	os.Exit(code)
}
