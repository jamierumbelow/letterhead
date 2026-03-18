package output

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jamierumbelow/letterhead/pkg/types"
)

func TestModeFromFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		asJSON  bool
		asJSONL bool
		want    Mode
		wantErr error
	}{
		{
			name: "human by default",
			want: ModeHuman,
		},
		{
			name:   "json mode",
			asJSON: true,
			want:   ModeJSON,
		},
		{
			name:    "jsonl mode",
			asJSONL: true,
			want:    ModeJSONL,
		},
		{
			name:    "conflicting flags",
			asJSON:  true,
			asJSONL: true,
			wantErr: ErrConflictingModes,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ModeFromFlags(tt.asJSON, tt.asJSONL)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ModeFromFlags() error = %v, want %v", err, tt.wantErr)
			}

			if got != tt.want {
				t.Fatalf("ModeFromFlags() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHumanFormatterStatus(t *testing.T) {
	t.Parallel()

	formatter, err := NewFormatter(ModeHuman)
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	var buf bytes.Buffer
	lastSyncAt := time.Date(2026, time.March, 18, 8, 0, 0, 0, time.UTC)

	err = formatter.WriteStatus(bufioWriter(&buf), types.StatusOutput{
		Account:           "user@example.com",
		ArchivePath:       "/tmp/archive",
		SyncMode:          "recent",
		MessageCount:      12,
		ThreadCount:       8,
		BootstrapComplete: true,
		BootstrapProgress: 100,
		LastSyncAt:        &lastSyncAt,
		SchedulerState:    "installed",
		DBHealth:          "ok",
	})
	if err != nil {
		t.Fatalf("WriteStatus() error = %v", err)
	}

	payload := buf.String()

	expectedFields := []string{
		"Account: user@example.com",
		"Archive Path: /tmp/archive",
		"Bootstrap Progress: 100.0%",
		"Last Sync At: 2026-03-18T08:00:00Z",
	}

	for _, field := range expectedFields {
		if !strings.Contains(payload, field) {
			t.Fatalf("human status output %q does not contain %q", payload, field)
		}
	}
}

func TestHumanFormatterFind(t *testing.T) {
	t.Parallel()

	formatter, err := NewFormatter(ModeHuman)
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	var buf bytes.Buffer
	err = formatter.WriteFind(bufioWriter(&buf), types.FindOutput{
		Results: []types.FindResult{
			{
				Subject:      "Quarterly update",
				Participants: []string{"A. Sender", "B. Recipient"},
				LatestAt:     time.Date(2026, time.March, 18, 8, 15, 0, 0, time.UTC),
				MessageCount: 3,
				Snippet:      "Latest numbers attached.",
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteFind() error = %v", err)
	}

	payload := buf.String()
	expectedFields := []string{
		"2026-03-18",
		"Quarterly update",
		"(3)",
		"A. Sender, B. Recipient",
		"Latest numbers attached.",
	}

	for _, field := range expectedFields {
		if !strings.Contains(payload, field) {
			t.Fatalf("human find output %q does not contain %q", payload, field)
		}
	}
}

func TestJSONFormatterFind(t *testing.T) {
	t.Parallel()

	formatter, err := NewFormatter(ModeJSON)
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	var buf bytes.Buffer
	err = formatter.WriteFind(bufioWriter(&buf), types.FindOutput{
		Results: []types.FindResult{
			{
				ResultID:   "res_1",
				ThreadID:   "thread_1",
				Subject:    "Quarterly update",
				ReadHandle: "thread_1",
			},
		},
		TotalCount: 1,
		QueryMS:    12,
	})
	if err != nil {
		t.Fatalf("WriteFind() error = %v", err)
	}

	payload := buf.String()
	expectedFields := []string{
		`"result_id":"res_1"`,
		`"total_count":1`,
		`"query_ms":12`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(payload, field) {
			t.Fatalf("json find output %q does not contain %q", payload, field)
		}
	}
}

func TestJSONLFormatterFind(t *testing.T) {
	t.Parallel()

	formatter, err := NewFormatter(ModeJSONL)
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	var buf bytes.Buffer
	err = formatter.WriteFind(bufioWriter(&buf), types.FindOutput{
		Results: []types.FindResult{
			{ResultID: "res_1", ThreadID: "thread_1"},
			{ResultID: "res_2", ThreadID: "thread_2"},
		},
	})
	if err != nil {
		t.Fatalf("WriteFind() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("jsonl line count = %d, want 2", len(lines))
	}

	if !strings.Contains(lines[0], `"result_id":"res_1"`) {
		t.Fatalf("first jsonl line = %q", lines[0])
	}

	if !strings.Contains(lines[1], `"result_id":"res_2"`) {
		t.Fatalf("second jsonl line = %q", lines[1])
	}
}

func bufioWriter(buf *bytes.Buffer) *bytes.Buffer {
	return buf
}
