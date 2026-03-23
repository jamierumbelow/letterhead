package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/jamierumbelow/letterhead/pkg/types"
)

func TestCompactJSONHelp(t *testing.T) {
	t.Parallel()

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var help types.HelpOutput
	if err := json.Unmarshal(stdout.Bytes(), &help); err != nil {
		t.Fatalf("JSON unmarshal error = %v, output = %q", err, stdout.String())
	}

	if len(help.Commands) == 0 {
		t.Fatal("expected non-empty commands list")
	}

	// Check that key commands are present
	found := map[string]bool{}
	for _, c := range help.Commands {
		found[c.Name] = true
	}

	for _, want := range []string{"find", "read", "status", "sync", "doctor"} {
		if !found[want] {
			t.Errorf("missing command %q in compact help", want)
		}
	}

	if len(help.Flags) == 0 {
		t.Fatal("expected non-empty flags list")
	}
}

func TestExitCodeNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code int
		want string
	}{
		{ExitOK, "ok"},
		{ExitUsage, "usage"},
		{ExitAuth, "auth"},
		{ExitNotFound, "not_found"},
		{ExitNotInitialized, "not_initialized"},
		{99, "unknown"},
	}

	for _, tt := range tests {
		got := ExitCodeName(tt.code)
		if got != tt.want {
			t.Errorf("ExitCodeName(%d) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestExitErrorWithHint(t *testing.T) {
	t.Parallel()

	err := NewExitErrorWithHint(ExitNotFound, "letterhead find <query>", "thread %q not found", "abc123")

	if err.Code != ExitNotFound {
		t.Errorf("Code = %d, want %d", err.Code, ExitNotFound)
	}
	if err.Hint != "letterhead find <query>" {
		t.Errorf("Hint = %q, want %q", err.Hint, "letterhead find <query>")
	}
	if err.Error() != `thread "abc123" not found` {
		t.Errorf("Error() = %q", err.Error())
	}
}

func TestErrorOutputJSON(t *testing.T) {
	t.Parallel()

	errOut := types.ErrorOutput{
		OK: false,
		Error: types.ErrorInfo{
			Code:     "not_found",
			ExitCode: 7,
			Message:  "thread not found",
			Hint:     "letterhead find <query>",
		},
	}

	data, err := json.Marshal(errOut)
	if err != nil {
		t.Fatalf("Marshal error = %v", err)
	}

	var decoded types.ErrorOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	if decoded.OK != false {
		t.Error("expected ok=false")
	}
	if decoded.Error.Code != "not_found" {
		t.Errorf("code = %q, want %q", decoded.Error.Code, "not_found")
	}
	if decoded.Error.ExitCode != 7 {
		t.Errorf("exit_code = %d, want 7", decoded.Error.ExitCode)
	}
	if decoded.Error.Hint != "letterhead find <query>" {
		t.Errorf("hint = %q", decoded.Error.Hint)
	}
}
