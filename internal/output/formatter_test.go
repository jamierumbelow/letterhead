package output

import (
	"bytes"
	"encoding/json"
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
		"Quarterly update",
		"(3)",
		"A. Sender, B. Recipient",
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

func TestJSONFindOutputRoundtrips(t *testing.T) {
	t.Parallel()

	formatter, err := NewFormatter(ModeJSON)
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	input := types.FindOutput{
		Results: []types.FindResult{
			{
				ResultID:      "res_1",
				ThreadID:      "thread_1",
				Subject:       "Quarterly update",
				Participants:  []string{"Alice", "Bob"},
				LatestAt:      time.Date(2026, 3, 18, 8, 0, 0, 0, time.UTC),
				MessageCount:  3,
				Snippet:       "Numbers attached.",
				MatchedFields: []string{"subject"},
				ReadHandle:    "thread_1",
			},
		},
		TotalCount: 1,
		QueryMS:    14,
	}

	var buf bytes.Buffer
	if err := formatter.WriteFind(&buf, input); err != nil {
		t.Fatalf("WriteFind() error = %v", err)
	}

	var decoded types.FindOutput
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nraw: %s", err, buf.String())
	}

	if decoded.TotalCount != input.TotalCount {
		t.Errorf("TotalCount = %d, want %d", decoded.TotalCount, input.TotalCount)
	}
	if decoded.QueryMS != input.QueryMS {
		t.Errorf("QueryMS = %d, want %d", decoded.QueryMS, input.QueryMS)
	}
	if len(decoded.Results) != 1 {
		t.Fatalf("Results count = %d, want 1", len(decoded.Results))
	}
	r := decoded.Results[0]
	if r.ResultID != "res_1" {
		t.Errorf("ResultID = %q, want %q", r.ResultID, "res_1")
	}
	if r.ReadHandle != "thread_1" {
		t.Errorf("ReadHandle = %q, want %q", r.ReadHandle, "thread_1")
	}
	if r.MessageCount != 3 {
		t.Errorf("MessageCount = %d, want 3", r.MessageCount)
	}
	if len(r.Participants) != 2 {
		t.Errorf("Participants count = %d, want 2", len(r.Participants))
	}
	if len(r.MatchedFields) != 1 || r.MatchedFields[0] != "subject" {
		t.Errorf("MatchedFields = %v, want [subject]", r.MatchedFields)
	}
}

func TestJSONReadOutputRoundtrips(t *testing.T) {
	t.Parallel()

	formatter, err := NewFormatter(ModeJSON)
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	input := types.ReadOutput{
		View:      types.ReadViewText,
		MessageID: "msg_1",
		ThreadID:  "thread_1",
		Subject:   "Quarterly update",
		From:      types.Address{Name: "Alice", Email: "alice@example.com"},
		Date:      time.Date(2026, 3, 18, 7, 30, 0, 0, time.UTC),
		Participants: []string{"Alice", "Bob"},
		Body:      "Here are the numbers.",
		Messages: []types.MessageSummary{
			{
				MessageID:       "msg_1",
				ThreadID:        "thread_1",
				Subject:         "Quarterly update",
				From:            types.Address{Name: "Alice", Email: "alice@example.com"},
				Date:            time.Date(2026, 3, 18, 7, 30, 0, 0, time.UTC),
				Participants:    []string{"Alice", "Bob"},
				Snippet:         "Numbers attached.",
				LabelNames:      []string{"INBOX"},
				AttachmentCount: 2,
			},
		},
	}

	var buf bytes.Buffer
	if err := formatter.WriteRead(&buf, input); err != nil {
		t.Fatalf("WriteRead() error = %v", err)
	}

	var decoded types.ReadOutput
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nraw: %s", err, buf.String())
	}

	if decoded.View != types.ReadViewText {
		t.Errorf("View = %q, want %q", decoded.View, types.ReadViewText)
	}
	if decoded.MessageID != "msg_1" {
		t.Errorf("MessageID = %q, want %q", decoded.MessageID, "msg_1")
	}
	if decoded.From.Email != "alice@example.com" {
		t.Errorf("From.Email = %q, want %q", decoded.From.Email, "alice@example.com")
	}
	if decoded.Body != "Here are the numbers." {
		t.Errorf("Body = %q, want %q", decoded.Body, "Here are the numbers.")
	}
	if len(decoded.Messages) != 1 {
		t.Fatalf("Messages count = %d, want 1", len(decoded.Messages))
	}
	if decoded.Messages[0].AttachmentCount != 2 {
		t.Errorf("AttachmentCount = %d, want 2", decoded.Messages[0].AttachmentCount)
	}
}

func TestJSONStatusOutputRoundtrips(t *testing.T) {
	t.Parallel()

	formatter, err := NewFormatter(ModeJSON)
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	lastSync := time.Date(2026, 3, 18, 8, 0, 0, 0, time.UTC)
	input := types.StatusOutput{
		Account:           "user@example.com",
		ArchivePath:       "/home/user/.local/share/letterhead/archive",
		SyncMode:          "recent",
		MessageCount:      120,
		ThreadCount:       80,
		BootstrapComplete: true,
		BootstrapProgress: 100,
		LastSyncAt:        &lastSync,
		SchedulerState:    "installed",
		DBHealth:          "ok",
	}

	var buf bytes.Buffer
	if err := formatter.WriteStatus(&buf, input); err != nil {
		t.Fatalf("WriteStatus() error = %v", err)
	}

	var decoded types.StatusOutput
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nraw: %s", err, buf.String())
	}

	if decoded.Account != "user@example.com" {
		t.Errorf("Account = %q, want %q", decoded.Account, "user@example.com")
	}
	if decoded.MessageCount != 120 {
		t.Errorf("MessageCount = %d, want 120", decoded.MessageCount)
	}
	if decoded.ThreadCount != 80 {
		t.Errorf("ThreadCount = %d, want 80", decoded.ThreadCount)
	}
	if !decoded.BootstrapComplete {
		t.Errorf("BootstrapComplete = false, want true")
	}
	if decoded.SchedulerState != "installed" {
		t.Errorf("SchedulerState = %q, want %q", decoded.SchedulerState, "installed")
	}
	if decoded.LastSyncAt == nil || !decoded.LastSyncAt.Equal(lastSync) {
		t.Errorf("LastSyncAt = %v, want %v", decoded.LastSyncAt, lastSync)
	}
}

func TestJSONLFindEachLineIsValidJSON(t *testing.T) {
	t.Parallel()

	formatter, err := NewFormatter(ModeJSONL)
	if err != nil {
		t.Fatalf("NewFormatter() error = %v", err)
	}

	input := types.FindOutput{
		Results: []types.FindResult{
			{ResultID: "res_1", ThreadID: "thread_1", Subject: "First"},
			{ResultID: "res_2", ThreadID: "thread_2", Subject: "Second"},
			{ResultID: "res_3", ThreadID: "thread_3", Subject: "Third"},
		},
	}

	var buf bytes.Buffer
	if err := formatter.WriteFind(&buf, input); err != nil {
		t.Fatalf("WriteFind() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("line count = %d, want 3", len(lines))
	}

	for i, line := range lines {
		var decoded types.FindResult
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("line %d is not valid JSON: %v\nraw: %s", i, err, line)
		}
		if decoded.ResultID != input.Results[i].ResultID {
			t.Errorf("line %d ResultID = %q, want %q", i, decoded.ResultID, input.Results[i].ResultID)
		}
	}
}

func bufioWriter(buf *bytes.Buffer) *bytes.Buffer {
	return buf
}
