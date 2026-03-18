package types

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestStatusOutputMarshalsWithStableFields(t *testing.T) {
	t.Parallel()

	output := StatusOutput{
		Account:           "user@example.com",
		ArchivePath:       "/tmp/archive",
		SyncMode:          "recent",
		MessageCount:      120,
		ThreadCount:       80,
		BootstrapComplete: true,
		BootstrapProgress: 100,
		LastSyncAt:        nil,
		SchedulerState:    "installed",
		DBHealth:          "ok",
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	payload := string(data)

	expectedFields := []string{
		`"account":"user@example.com"`,
		`"archive_path":"/tmp/archive"`,
		`"sync_mode":"recent"`,
		`"message_count":120`,
		`"thread_count":80`,
		`"bootstrap_complete":true`,
		`"bootstrap_progress":100`,
		`"last_sync_at":null`,
		`"scheduler_state":"installed"`,
		`"db_health":"ok"`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(payload, field) {
			t.Fatalf("json payload %q does not contain %q", payload, field)
		}
	}
}

func TestFindOutputMarshalsWithStableFields(t *testing.T) {
	t.Parallel()

	output := FindOutput{
		Results: []FindResult{
			{
				ResultID:      "res_1",
				ThreadID:      "thread_1",
				Subject:       "Quarterly update",
				Participants:  []string{"A. Sender", "B. Recipient"},
				LatestAt:      time.Date(2026, time.March, 18, 7, 0, 0, 0, time.UTC),
				MessageCount:  3,
				Snippet:       "Latest numbers attached.",
				MatchedFields: []string{"subject", "plain_body"},
				ReadHandle:    "thread_1",
			},
		},
		TotalCount: 1,
		QueryMS:    14,
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	payload := string(data)

	expectedFields := []string{
		`"result_id":"res_1"`,
		`"thread_id":"thread_1"`,
		`"latest_at":"2026-03-18T07:00:00Z"`,
		`"matched_fields":["subject","plain_body"]`,
		`"read_handle":"thread_1"`,
		`"total_count":1`,
		`"query_ms":14`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(payload, field) {
			t.Fatalf("json payload %q does not contain %q", payload, field)
		}
	}
}

func TestReadOutputMarshalsWithStableFields(t *testing.T) {
	t.Parallel()

	output := ReadOutput{
		View:      ReadViewSummary,
		MessageID: "msg_1",
		ThreadID:  "thread_1",
		Subject:   "Quarterly update",
		From: Address{
			Name:  "A. Sender",
			Email: "sender@example.com",
		},
		Date:         time.Date(2026, time.March, 18, 7, 30, 0, 0, time.UTC),
		Participants: []string{"A. Sender", "B. Recipient"},
		Body:         "Hello world",
		Messages: []MessageSummary{
			{
				MessageID: "msg_1",
				ThreadID:  "thread_1",
				Subject:   "Quarterly update",
				From: Address{
					Name:  "A. Sender",
					Email: "sender@example.com",
				},
				Date:            time.Date(2026, time.March, 18, 7, 30, 0, 0, time.UTC),
				Participants:    []string{"A. Sender", "B. Recipient"},
				Snippet:         "Latest numbers attached.",
				LabelNames:      []string{"INBOX"},
				AttachmentCount: 1,
			},
		},
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	payload := string(data)

	expectedFields := []string{
		`"view":"summary"`,
		`"message_id":"msg_1"`,
		`"thread_id":"thread_1"`,
		`"date":"2026-03-18T07:30:00Z"`,
		`"participants":["A. Sender","B. Recipient"]`,
		`"body":"Hello world"`,
		`"messages":[{`,
		`"attachment_count":1`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(payload, field) {
			t.Fatalf("json payload %q does not contain %q", payload, field)
		}
	}
}
