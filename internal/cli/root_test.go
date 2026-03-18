package cli

import (
	"errors"
	"testing"
)

func TestRootCommandAcceptsSingleOutputMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "human output by default",
		},
		{
			name: "json output",
			args: []string{"--json"},
		},
		{
			name: "jsonl output",
			args: []string{"--jsonl"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := NewRootCommand()
			cmd.SetArgs(tt.args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
		})
	}
}

func TestRootCommandRejectsConflictingOutputModes(t *testing.T) {
	t.Parallel()

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"--json", "--jsonl"})

	err := cmd.Execute()
	if !errors.Is(err, errConflictingOutputModes) {
		t.Fatalf("Execute() error = %v, want %v", err, errConflictingOutputModes)
	}
}
