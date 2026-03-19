package gmail

import (
	"encoding/base64"
	"mime"
	"net/mail"
	"strings"
	"time"

	"github.com/jamierumbelow/letterhead/pkg/types"
	"golang.org/x/text/encoding/ianaindex"
	gm "google.golang.org/api/gmail/v1"
)

const maxRecursionDepth = 50

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
	msg.Subject = decodeRFC2047(headers["Subject"])
	msg.From = parseAddress(headers["From"])

	msg.To = parseAddressList(headers["To"])
	msg.CC = parseAddressList(headers["Cc"])
	msg.BCC = parseAddressList(headers["Bcc"])

	// Extract labels
	msg.Labels = raw.LabelIds

	// Handle messages with no Content-Type (treat as text/plain)
	if raw.Payload != nil && raw.Payload.MimeType == "" {
		raw.Payload.MimeType = "text/plain"
	}

	// Walk MIME tree for body and attachments
	msg.PlainBody, msg.HTMLBody, msg.Attachments = walkParts(raw.Payload, 0)

	// Derive plain text from HTML if no text/plain part exists
	if msg.PlainBody == "" && msg.HTMLBody != "" {
		msg.PlainBody = htmlToText(msg.HTMLBody)
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

func walkParts(part *gm.MessagePart, depth int) (plainBody, htmlBody string, attachments []types.AttachmentMeta) {
	if part == nil || depth > maxRecursionDepth {
		return
	}

	mimeType := strings.ToLower(part.MimeType)

	// Determine if this is an inline part or an attachment
	isAttachment := part.Filename != ""
	if !isAttachment {
		disposition := ""
		for _, h := range part.Headers {
			if strings.EqualFold(h.Name, "Content-Disposition") {
				disposition = strings.ToLower(h.Value)
				break
			}
		}
		// content-disposition: attachment means treat as attachment even without filename
		isAttachment = strings.HasPrefix(disposition, "attachment")
	}

	// Leaf node with body data
	if part.Body != nil && part.Body.Data != "" && !isAttachment {
		decoded := decodeBodyData(part)

		switch {
		case mimeType == "text/plain":
			plainBody = decoded
		case mimeType == "text/html":
			htmlBody = decoded
		case mimeType == "" || mimeType == "application/octet-stream":
			// No content-type or generic type: treat as text/plain
			plainBody = decoded
		}
	}

	// Attachment metadata (has filename)
	if part.Filename != "" && part.Body != nil {
		attachments = append(attachments, types.AttachmentMeta{
			Filename:  part.Filename,
			MIMEType:  part.MimeType,
			SizeBytes: int64(part.Body.Size),
			PartID:    part.PartId,
		})
	}

	// Recurse into sub-parts (multipart/*)
	for _, child := range part.Parts {
		childPlain, childHTML, childAttach := walkParts(child, depth+1)
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

// decodeBodyData decodes the base64url body data and handles charset conversion.
func decodeBodyData(part *gm.MessagePart) string {
	raw := decodeBase64URL(part.Body.Data)

	// Check for non-UTF-8 charset in Content-Type header
	charset := extractCharset(part)
	if charset != "" && !isUTF8(charset) {
		if converted, err := decodeCharset(raw, charset); err == nil {
			return converted
		}
	}

	return raw
}

func extractCharset(part *gm.MessagePart) string {
	for _, h := range part.Headers {
		if strings.EqualFold(h.Name, "Content-Type") {
			_, params, err := mime.ParseMediaType(h.Value)
			if err == nil {
				return strings.ToLower(params["charset"])
			}
		}
	}
	return ""
}

func isUTF8(charset string) bool {
	charset = strings.ToLower(strings.ReplaceAll(charset, "-", ""))
	return charset == "" || charset == "utf8" || charset == "usascii" || charset == "ascii"
}

func decodeCharset(s string, charset string) (string, error) {
	enc, err := ianaindex.IANA.Encoding(charset)
	if err != nil || enc == nil {
		return s, err
	}
	decoded, err := enc.NewDecoder().String(s)
	if err != nil {
		return s, err
	}
	return decoded, nil
}

func decodeBase64URL(s string) string {
	// Gmail uses URL-safe base64 without padding
	data, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		// Try without padding (handles malformed base64 with missing padding)
		data, err = base64.RawURLEncoding.DecodeString(s)
		if err != nil {
			return s
		}
	}
	return string(data)
}

func decodeRFC2047(s string) string {
	dec := new(mime.WordDecoder)
	decoded, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return decoded
}

func parseAddress(s string) types.Address {
	if s == "" {
		return types.Address{}
	}

	s = decodeRFC2047(s)

	addr, err := mail.ParseAddress(s)
	if err != nil {
		// Fall back to using raw string as email
		return types.Address{Email: s}
	}
	return types.Address{Name: addr.Name, Email: addr.Address}
}

func parseAddressList(s string) []types.Address {
	if s == "" {
		return nil
	}

	s = decodeRFC2047(s)

	addrs, err := mail.ParseAddressList(s)
	if err != nil {
		return nil
	}

	result := make([]types.Address, len(addrs))
	for i, a := range addrs {
		result[i] = types.Address{Name: a.Name, Email: a.Address}
	}
	return result
}

// stripHTML is a simple fallback HTML stripper for when proper parsing fails.
func stripHTML(html string) string {
	var b strings.Builder
	inTag := false
	var tagName strings.Builder

	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
			tagName.Reset()
		case r == '>' && inTag:
			inTag = false
			tag := strings.ToLower(tagName.String())
			if isBlockTag(tag) || strings.HasPrefix(tag, "br") {
				b.WriteRune(' ')
			}
		case inTag:
			tagName.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}

	result := b.String()
	result = strings.Join(strings.Fields(result), " ")
	return strings.TrimSpace(result)
}

func isBlockTag(tag string) bool {
	tag = strings.TrimPrefix(tag, "/")
	switch tag {
	case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6",
		"li", "tr", "td", "th", "blockquote", "pre", "hr",
		"section", "article", "header", "footer", "nav":
		return true
	}
	return false
}
