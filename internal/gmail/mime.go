package gmail

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/jamierumbelow/letterhead/internal/mimeutil"
	"github.com/jamierumbelow/letterhead/pkg/types"
	gm "google.golang.org/api/gmail/v1"
)

// NormalizeMessage converts a raw Gmail API message (full format) into a
// normalized types.Message.
func NormalizeMessage(raw *gm.Message) *types.Message {
	msg := &types.Message{
		GmailID:      raw.Id,
		ThreadID:     raw.ThreadId,
		HistoryID:    uint64(raw.HistoryId),
		InternalDate: raw.InternalDate,
		ReceivedAt:   time.UnixMilli(raw.InternalDate).UTC(),
		Snippet:      raw.Snippet,
	}

	// Extract headers
	headers := headerMap(raw.Payload)
	msg.Subject = mimeutil.DecodeRFC2047(headers["Subject"])
	msg.From = mimeutil.ParseAddress(headers["From"])

	msg.To = mimeutil.ParseAddressList(headers["To"])
	msg.CC = mimeutil.ParseAddressList(headers["Cc"])
	msg.BCC = mimeutil.ParseAddressList(headers["Bcc"])

	// Extract labels
	msg.Labels = raw.LabelIds

	// Walk MIME tree for body and attachments
	msg.PlainBody, msg.HTMLBody, msg.Attachments = walkParts(raw.Payload)

	// Derive plain text from HTML if no text/plain part exists
	if msg.PlainBody == "" && msg.HTMLBody != "" {
		msg.PlainBody = mimeutil.StripHTML(msg.HTMLBody)
	}

	return msg
}

func headerMap(part *gm.MessagePart) map[string]string {
	if part == nil {
		return nil
	}
	m := make(map[string]string, len(part.Headers))
	for _, h := range part.Headers {
		m[h.Name] = h.Value
	}
	return m
}

func walkParts(part *gm.MessagePart) (plainBody, htmlBody string, attachments []types.AttachmentMeta) {
	if part == nil {
		return
	}

	mimeType := strings.ToLower(part.MimeType)

	// Leaf node with body data
	if part.Body != nil && part.Body.Data != "" {
		decoded := decodeBase64URL(part.Body.Data)

		switch {
		case mimeType == "text/plain" && part.Filename == "":
			plainBody = decoded
		case mimeType == "text/html" && part.Filename == "":
			htmlBody = decoded
		}
	}

	// Attachment (has filename or is not text)
	if part.Filename != "" {
		attachments = append(attachments, types.AttachmentMeta{
			Filename:  part.Filename,
			MIMEType:  part.MimeType,
			SizeBytes: int64(part.Body.Size),
			PartID:    part.PartId,
		})
	}

	// Recurse into sub-parts (multipart/*)
	for _, child := range part.Parts {
		childPlain, childHTML, childAttach := walkParts(child)
		if plainBody == "" {
			plainBody = childPlain
		}
		if htmlBody == "" {
			htmlBody = childHTML
		}
		attachments = append(attachments, childAttach...)
	}

	return
}

func decodeBase64URL(s string) string {
	// Gmail uses URL-safe base64 without padding
	data, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		// Try without padding
		data, err = base64.RawURLEncoding.DecodeString(s)
		if err != nil {
			return s
		}
	}
	return string(data)
}

