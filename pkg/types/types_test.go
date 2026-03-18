package types

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMessageMarshalsWithSnakeCaseFields(t *testing.T) {
	t.Parallel()

	receivedAt := time.Date(2026, time.March, 18, 5, 45, 0, 0, time.UTC)

	message := Message{
		GmailID:      "msg_123",
		ThreadID:     "thread_456",
		HistoryID:    789,
		InternalDate: 1742276700000,
		ReceivedAt:   receivedAt,
		Subject:      "Quarterly update",
		Snippet:      "Latest numbers attached.",
		From: Address{
			Name:  "A. Sender",
			Email: "sender@example.com",
		},
		To: []Address{
			{
				Name:  "B. Recipient",
				Email: "recipient@example.com",
			},
		},
		Labels:    []string{"INBOX", "IMPORTANT"},
		PlainBody: "Hello world",
		Attachments: []AttachmentMeta{
			{
				Filename:  "report.pdf",
				MIMEType:  "application/pdf",
				SizeBytes: 1024,
				PartID:    "2",
			},
		},
	}

	data, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	payload := string(data)

	expectedFields := []string{
		`"gmail_id":"msg_123"`,
		`"thread_id":"thread_456"`,
		`"history_id":789`,
		`"internal_date":1742276700000`,
		`"received_at":"2026-03-18T05:45:00Z"`,
		`"plain_body":"Hello world"`,
		`"mime_type":"application/pdf"`,
		`"size_bytes":1024`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(payload, field) {
			t.Fatalf("json payload %q does not contain %q", payload, field)
		}
	}

	if strings.Contains(payload, `"html_body"`) {
		t.Fatalf("json payload %q unexpectedly contains html_body", payload)
	}
}

func TestThreadSummaryMarshalsWithStableFieldNames(t *testing.T) {
	t.Parallel()

	summary := ThreadSummary{
		ThreadID:     "thread_456",
		Subject:      "Quarterly update",
		Participants: []string{"A. Sender", "B. Recipient"},
		LatestAt:     time.Date(2026, time.March, 18, 6, 0, 0, 0, time.UTC),
		MessageCount: 2,
		Snippet:      "Latest numbers attached.",
		LabelNames:   []string{"INBOX"},
		MessageIDs:   []string{"msg_123", "msg_124"},
	}

	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	payload := string(data)

	expectedFields := []string{
		`"thread_id":"thread_456"`,
		`"participants":["A. Sender","B. Recipient"]`,
		`"latest_at":"2026-03-18T06:00:00Z"`,
		`"message_count":2`,
		`"label_names":["INBOX"]`,
		`"message_ids":["msg_123","msg_124"]`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(payload, field) {
			t.Fatalf("json payload %q does not contain %q", payload, field)
		}
	}
}
