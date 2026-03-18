package gmail

import (
	"encoding/base64"
	"testing"

	gm "google.golang.org/api/gmail/v1"
)

func b64url(s string) string {
	return base64.URLEncoding.EncodeToString([]byte(s))
}

func TestNormalizeSimpleMessage(t *testing.T) {
	t.Parallel()

	raw := &gm.Message{
		Id:           "msg_123",
		ThreadId:     "thread_456",
		HistoryId:    789,
		InternalDate: 1742280000000,
		Snippet:      "Hello there",
		LabelIds:     []string{"INBOX", "IMPORTANT"},
		Payload: &gm.MessagePart{
			MimeType: "text/plain",
			Headers: []*gm.MessagePartHeader{
				{Name: "Subject", Value: "Test Subject"},
				{Name: "From", Value: "Alice <alice@example.com>"},
				{Name: "To", Value: "Bob <bob@example.com>"},
			},
			Body: &gm.MessagePartBody{
				Data: b64url("Hello, World!"),
			},
		},
	}

	msg := NormalizeMessage(raw)

	if msg.GmailID != "msg_123" {
		t.Errorf("GmailID = %q, want msg_123", msg.GmailID)
	}
	if msg.ThreadID != "thread_456" {
		t.Errorf("ThreadID = %q, want thread_456", msg.ThreadID)
	}
	if msg.Subject != "Test Subject" {
		t.Errorf("Subject = %q, want %q", msg.Subject, "Test Subject")
	}
	if msg.From.Email != "alice@example.com" {
		t.Errorf("From.Email = %q, want alice@example.com", msg.From.Email)
	}
	if msg.From.Name != "Alice" {
		t.Errorf("From.Name = %q, want Alice", msg.From.Name)
	}
	if len(msg.To) != 1 || msg.To[0].Email != "bob@example.com" {
		t.Errorf("To = %v", msg.To)
	}
	if msg.PlainBody != "Hello, World!" {
		t.Errorf("PlainBody = %q, want %q", msg.PlainBody, "Hello, World!")
	}
	if len(msg.Labels) != 2 {
		t.Errorf("Labels = %v, want [INBOX IMPORTANT]", msg.Labels)
	}
}

func TestNormalizeMultipartMessage(t *testing.T) {
	t.Parallel()

	raw := &gm.Message{
		Id:           "msg_multi",
		ThreadId:     "thread_1",
		InternalDate: 1742280000000,
		Payload: &gm.MessagePart{
			MimeType: "multipart/alternative",
			Headers: []*gm.MessagePartHeader{
				{Name: "Subject", Value: "Multipart"},
				{Name: "From", Value: "sender@example.com"},
			},
			Parts: []*gm.MessagePart{
				{
					MimeType: "text/plain",
					Body:     &gm.MessagePartBody{Data: b64url("Plain version")},
				},
				{
					MimeType: "text/html",
					Body:     &gm.MessagePartBody{Data: b64url("<p>HTML version</p>")},
				},
			},
		},
	}

	msg := NormalizeMessage(raw)

	if msg.PlainBody != "Plain version" {
		t.Errorf("PlainBody = %q, want %q", msg.PlainBody, "Plain version")
	}
	if msg.HTMLBody != "<p>HTML version</p>" {
		t.Errorf("HTMLBody = %q", msg.HTMLBody)
	}
}

func TestNormalizeHTMLOnlyDerivesPlainText(t *testing.T) {
	t.Parallel()

	raw := &gm.Message{
		Id:           "msg_html",
		ThreadId:     "thread_1",
		InternalDate: 1742280000000,
		Payload: &gm.MessagePart{
			MimeType: "text/html",
			Headers: []*gm.MessagePartHeader{
				{Name: "Subject", Value: "HTML only"},
				{Name: "From", Value: "sender@example.com"},
			},
			Body: &gm.MessagePartBody{
				Data: b64url("<html><body><p>Hello</p><br><p>World</p></body></html>"),
			},
		},
	}

	msg := NormalizeMessage(raw)

	if msg.PlainBody == "" {
		t.Fatal("PlainBody is empty, should be derived from HTML")
	}
	if msg.PlainBody != "Hello World" {
		t.Errorf("PlainBody = %q, want %q", msg.PlainBody, "Hello World")
	}
}

func TestNormalizeWithAttachments(t *testing.T) {
	t.Parallel()

	raw := &gm.Message{
		Id:           "msg_attach",
		ThreadId:     "thread_1",
		InternalDate: 1742280000000,
		Payload: &gm.MessagePart{
			MimeType: "multipart/mixed",
			Headers: []*gm.MessagePartHeader{
				{Name: "Subject", Value: "With attachment"},
				{Name: "From", Value: "sender@example.com"},
			},
			Parts: []*gm.MessagePart{
				{
					MimeType: "text/plain",
					Body:     &gm.MessagePartBody{Data: b64url("See attached")},
				},
				{
					MimeType: "application/pdf",
					Filename: "report.pdf",
					PartId:   "1",
					Body:     &gm.MessagePartBody{Size: 2048, AttachmentId: "att123"},
				},
			},
		},
	}

	msg := NormalizeMessage(raw)

	if len(msg.Attachments) != 1 {
		t.Fatalf("Attachments count = %d, want 1", len(msg.Attachments))
	}

	att := msg.Attachments[0]
	if att.Filename != "report.pdf" {
		t.Errorf("Filename = %q", att.Filename)
	}
	if att.MIMEType != "application/pdf" {
		t.Errorf("MIMEType = %q", att.MIMEType)
	}
	if att.SizeBytes != 2048 {
		t.Errorf("SizeBytes = %d", att.SizeBytes)
	}
	if att.PartID != "1" {
		t.Errorf("PartID = %q", att.PartID)
	}
}

func TestNormalizeMultipleRecipients(t *testing.T) {
	t.Parallel()

	raw := &gm.Message{
		Id:           "msg_recips",
		ThreadId:     "thread_1",
		InternalDate: 1742280000000,
		Payload: &gm.MessagePart{
			MimeType: "text/plain",
			Headers: []*gm.MessagePartHeader{
				{Name: "Subject", Value: "Multi recips"},
				{Name: "From", Value: "Alice <alice@example.com>"},
				{Name: "To", Value: "Bob <bob@example.com>, Carol <carol@example.com>"},
				{Name: "Cc", Value: "Dave <dave@example.com>"},
				{Name: "Bcc", Value: "Eve <eve@example.com>"},
			},
			Body: &gm.MessagePartBody{Data: b64url("body")},
		},
	}

	msg := NormalizeMessage(raw)

	if len(msg.To) != 2 {
		t.Fatalf("To count = %d, want 2", len(msg.To))
	}
	if msg.To[0].Email != "bob@example.com" || msg.To[1].Email != "carol@example.com" {
		t.Errorf("To = %v", msg.To)
	}
	if len(msg.CC) != 1 || msg.CC[0].Email != "dave@example.com" {
		t.Errorf("CC = %v", msg.CC)
	}
	if len(msg.BCC) != 1 || msg.BCC[0].Email != "eve@example.com" {
		t.Errorf("BCC = %v", msg.BCC)
	}
}

func TestStripHTML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"<p>Hello</p>", "Hello"},
		{"<html><body><h1>Title</h1><p>Body text</p></body></html>", "Title Body text"},
		{"No tags here", "No tags here"},
		{"<br>Line1<br>Line2", "Line1 Line2"},
		{"", ""},
	}

	for _, tt := range tests {
		got := stripHTML(tt.input)
		if got != tt.want {
			t.Errorf("stripHTML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDecodeRFC2047(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"Plain text", "Plain text"},
		{"=?UTF-8?B?SGVsbG8gV29ybGQ=?=", "Hello World"},
		{"=?UTF-8?Q?Hello_World?=", "Hello World"},
	}

	for _, tt := range tests {
		got := decodeRFC2047(tt.input)
		if got != tt.want {
			t.Errorf("decodeRFC2047(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input     string
		wantName  string
		wantEmail string
	}{
		{"Alice <alice@example.com>", "Alice", "alice@example.com"},
		{"alice@example.com", "", "alice@example.com"},
		{"\"Bob Smith\" <bob@example.com>", "Bob Smith", "bob@example.com"},
		{"", "", ""},
	}

	for _, tt := range tests {
		got := parseAddress(tt.input)
		if got.Name != tt.wantName {
			t.Errorf("parseAddress(%q).Name = %q, want %q", tt.input, got.Name, tt.wantName)
		}
		if got.Email != tt.wantEmail {
			t.Errorf("parseAddress(%q).Email = %q, want %q", tt.input, got.Email, tt.wantEmail)
		}
	}
}

func TestNormalizeNestedMultipart(t *testing.T) {
	t.Parallel()

	// multipart/mixed -> multipart/alternative -> text/plain + text/html
	raw := &gm.Message{
		Id:           "msg_nested",
		ThreadId:     "thread_1",
		InternalDate: 1742280000000,
		Payload: &gm.MessagePart{
			MimeType: "multipart/mixed",
			Headers: []*gm.MessagePartHeader{
				{Name: "Subject", Value: "Nested"},
				{Name: "From", Value: "sender@example.com"},
			},
			Parts: []*gm.MessagePart{
				{
					MimeType: "multipart/alternative",
					Parts: []*gm.MessagePart{
						{
							MimeType: "text/plain",
							Body:     &gm.MessagePartBody{Data: b64url("Plain text")},
						},
						{
							MimeType: "text/html",
							Body:     &gm.MessagePartBody{Data: b64url("<p>HTML</p>")},
						},
					},
				},
				{
					MimeType: "image/png",
					Filename: "photo.png",
					PartId:   "2",
					Body:     &gm.MessagePartBody{Size: 4096},
				},
			},
		},
	}

	msg := NormalizeMessage(raw)

	if msg.PlainBody != "Plain text" {
		t.Errorf("PlainBody = %q", msg.PlainBody)
	}
	if msg.HTMLBody != "<p>HTML</p>" {
		t.Errorf("HTMLBody = %q", msg.HTMLBody)
	}
	if len(msg.Attachments) != 1 || msg.Attachments[0].Filename != "photo.png" {
		t.Errorf("Attachments = %v", msg.Attachments)
	}
}
