package imapclient

import (
	"strings"
	"testing"
	"time"

	"github.com/jamierumbelow/letterhead/pkg/types"
)

func TestParseRFC822Message_SimplePlainText(t *testing.T) {
	raw := []byte("From: Alice <alice@example.com>\r\n" +
		"To: Bob <bob@example.com>\r\n" +
		"Subject: Hello World\r\n" +
		"Message-Id: <msg001@example.com>\r\n" +
		"Date: Thu, 20 Mar 2025 10:00:00 +0000\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"This is the body.\r\n")

	msg, err := ParseRFC822Message(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, "subject", "Hello World", msg.Subject)
	assertEqual(t, "from.name", "Alice", msg.From.Name)
	assertEqual(t, "from.email", "alice@example.com", msg.From.Email)
	assertEqual(t, "to[0].email", "bob@example.com", msg.To[0].Email)
	assertEqual(t, "gmail_id", "msg001@example.com", msg.GmailID)
	assertContains(t, "plain_body", msg.PlainBody, "This is the body.")

	expectedDate := time.Date(2025, 3, 20, 10, 0, 0, 0, time.UTC)
	if !msg.ReceivedAt.Equal(expectedDate) {
		t.Errorf("received_at: got %v, want %v", msg.ReceivedAt, expectedDate)
	}
}

func TestParseRFC822Message_MultipartAlternative(t *testing.T) {
	raw := []byte("From: Alice <alice@example.com>\r\n" +
		"To: Bob <bob@example.com>\r\n" +
		"Subject: Multipart\r\n" +
		"Message-Id: <msg002@example.com>\r\n" +
		"Date: Thu, 20 Mar 2025 10:00:00 +0000\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/alternative; boundary=boundary42\r\n" +
		"\r\n" +
		"--boundary42\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Plain text version\r\n" +
		"--boundary42\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<p>HTML version</p>\r\n" +
		"--boundary42--\r\n")

	msg, err := ParseRFC822Message(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContains(t, "plain_body", msg.PlainBody, "Plain text version")
	assertContains(t, "html_body", msg.HTMLBody, "<p>HTML version</p>")
}

func TestParseRFC822Message_WithAttachment(t *testing.T) {
	raw := []byte("From: Alice <alice@example.com>\r\n" +
		"To: Bob <bob@example.com>\r\n" +
		"Subject: Attachment\r\n" +
		"Message-Id: <msg003@example.com>\r\n" +
		"Date: Thu, 20 Mar 2025 10:00:00 +0000\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=boundary99\r\n" +
		"\r\n" +
		"--boundary99\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Body text\r\n" +
		"--boundary99\r\n" +
		"Content-Type: application/pdf; name=\"report.pdf\"\r\n" +
		"Content-Disposition: attachment; filename=\"report.pdf\"\r\n" +
		"\r\n" +
		"fakepdfcontent\r\n" +
		"--boundary99--\r\n")

	msg, err := ParseRFC822Message(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msg.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(msg.Attachments))
	}
	assertEqual(t, "attachment.filename", "report.pdf", msg.Attachments[0].Filename)
	assertEqual(t, "attachment.mime_type", "application/pdf", msg.Attachments[0].MIMEType)
	if msg.Attachments[0].SizeBytes <= 0 {
		t.Errorf("expected positive size, got %d", msg.Attachments[0].SizeBytes)
	}
}

func TestParseRFC822Message_RFC2047Subject(t *testing.T) {
	raw := []byte("From: Alice <alice@example.com>\r\n" +
		"To: Bob <bob@example.com>\r\n" +
		"Subject: =?UTF-8?B?SGVsbG8gV29ybGQ=?=\r\n" +
		"Message-Id: <msg004@example.com>\r\n" +
		"Date: Thu, 20 Mar 2025 10:00:00 +0000\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Body\r\n")

	msg, err := ParseRFC822Message(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, "subject", "Hello World", msg.Subject)
}

func TestParseRFC822Message_ThreadFromReferences(t *testing.T) {
	raw := []byte("From: Alice <alice@example.com>\r\n" +
		"To: Bob <bob@example.com>\r\n" +
		"Subject: Re: Thread\r\n" +
		"Message-Id: <msg005@example.com>\r\n" +
		"References: <root@example.com> <mid@example.com>\r\n" +
		"Date: Thu, 20 Mar 2025 10:00:00 +0000\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Reply body\r\n")

	msg, err := ParseRFC822Message(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, "thread_id", "root@example.com", msg.ThreadID)
}

func TestParseRFC822Message_ThreadFromInReplyTo(t *testing.T) {
	raw := []byte("From: Alice <alice@example.com>\r\n" +
		"To: Bob <bob@example.com>\r\n" +
		"Subject: Re: Thread\r\n" +
		"Message-Id: <msg006@example.com>\r\n" +
		"In-Reply-To: <parent@example.com>\r\n" +
		"Date: Thu, 20 Mar 2025 10:00:00 +0000\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Reply body\r\n")

	msg, err := ParseRFC822Message(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, "thread_id", "parent@example.com", msg.ThreadID)
}

func TestParseRFC822Message_StandaloneThreadID(t *testing.T) {
	raw := []byte("From: Alice <alice@example.com>\r\n" +
		"To: Bob <bob@example.com>\r\n" +
		"Subject: Standalone\r\n" +
		"Message-Id: <standalone@example.com>\r\n" +
		"Date: Thu, 20 Mar 2025 10:00:00 +0000\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Body\r\n")

	msg, err := ParseRFC822Message(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, "thread_id", "standalone@example.com", msg.ThreadID)
	assertEqual(t, "thread_id == gmail_id", msg.GmailID, msg.ThreadID)
}

func TestParseRFC822Message_SnippetGeneration(t *testing.T) {
	longBody := strings.Repeat("word ", 100) // 500 chars
	raw := []byte("From: Alice <alice@example.com>\r\n" +
		"To: Bob <bob@example.com>\r\n" +
		"Subject: Long\r\n" +
		"Message-Id: <msg008@example.com>\r\n" +
		"Date: Thu, 20 Mar 2025 10:00:00 +0000\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		longBody + "\r\n")

	msg, err := ParseRFC822Message(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msg.Snippet) > 200 {
		t.Errorf("snippet too long: %d chars", len(msg.Snippet))
	}
	// Should end at a word boundary (no trailing partial word)
	if msg.Snippet != "" && msg.Snippet[len(msg.Snippet)-1] == ' ' {
		t.Error("snippet should not end with a space")
	}
}

func TestParseRFC822Message_HTMLOnly(t *testing.T) {
	raw := []byte("From: Alice <alice@example.com>\r\n" +
		"To: Bob <bob@example.com>\r\n" +
		"Subject: HTML Only\r\n" +
		"Message-Id: <msg009@example.com>\r\n" +
		"Date: Thu, 20 Mar 2025 10:00:00 +0000\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"\r\n" +
		"<p>Hello <b>World</b></p>\r\n")

	msg, err := ParseRFC822Message(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertContains(t, "html_body", msg.HTMLBody, "<p>Hello <b>World</b></p>")
	// PlainBody should be derived from HTML
	assertContains(t, "plain_body", msg.PlainBody, "Hello World")
	if strings.Contains(msg.PlainBody, "<") {
		t.Error("plain_body should not contain HTML tags")
	}
}

// helpers

func assertEqual(t *testing.T, field, want, got string) {
	t.Helper()
	if want != got {
		t.Errorf("%s: got %q, want %q", field, got, want)
	}
}

func assertContains(t *testing.T, field, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("%s: expected to contain %q, got %q", field, needle, haystack)
	}
}

// Verify that parsed message satisfies types.Message (compile check).
var _ *types.Message
