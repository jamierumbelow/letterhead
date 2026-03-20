package imapclient

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	gomail "github.com/emersion/go-message/mail"
	"github.com/jamierumbelow/letterhead/internal/mimeutil"
	"github.com/jamierumbelow/letterhead/pkg/types"
)

// ParseRFC822Message parses a raw RFC822 message into a types.Message.
func ParseRFC822Message(raw []byte) (*types.Message, error) {
	mr, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse message: %w", err)
	}
	defer mr.Close()

	header := mr.Header

	msg := &types.Message{}

	// Message-ID
	messageID, _ := header.MessageID()
	msg.GmailID = stripAngleBrackets(messageID)

	// Thread ID from References/In-Reply-To
	references, _ := header.MsgIDList("References")
	inReplyTo, _ := header.MsgIDList("In-Reply-To")
	var inReplyToStr string
	if len(inReplyTo) > 0 {
		inReplyToStr = inReplyTo[0]
	}
	msg.ThreadID = resolveThreadID(references, inReplyToStr, msg.GmailID)

	// Subject
	msg.Subject, _ = header.Subject()

	// Date
	date, err := header.Date()
	if err == nil {
		msg.InternalDate = date.UnixMilli()
		msg.ReceivedAt = date.UTC()
	}

	// From
	fromAddrs, _ := header.AddressList("From")
	if len(fromAddrs) > 0 {
		msg.From = types.Address{Name: fromAddrs[0].Name, Email: fromAddrs[0].Address}
	}

	// To, CC, BCC
	msg.To = parseGoMailAddresses(header, "To")
	msg.CC = parseGoMailAddresses(header, "Cc")
	msg.BCC = parseGoMailAddresses(header, "Bcc")

	// Walk MIME parts
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch h := part.Header.(type) {
		case *gomail.InlineHeader:
			ct, _, _ := h.ContentType()
			body, _ := io.ReadAll(part.Body)

			switch {
			case ct == "text/plain" && msg.PlainBody == "":
				msg.PlainBody = string(body)
			case ct == "text/html" && msg.HTMLBody == "":
				msg.HTMLBody = string(body)
			}
		case *gomail.AttachmentHeader:
			filename, _ := h.Filename()
			ct, _, _ := h.ContentType()
			body, _ := io.ReadAll(part.Body)
			msg.Attachments = append(msg.Attachments, types.AttachmentMeta{
				Filename:  filename,
				MIMEType:  ct,
				SizeBytes: int64(len(body)),
			})
		}
	}

	// Derive plain text from HTML if needed
	if msg.PlainBody == "" && msg.HTMLBody != "" {
		msg.PlainBody = mimeutil.StripHTML(msg.HTMLBody)
	}

	// Generate snippet
	msg.Snippet = mimeutil.GenerateSnippet(msg.PlainBody, 200)

	return msg, nil
}

// stripAngleBrackets removes surrounding < > from a Message-ID.
func stripAngleBrackets(s string) string {
	s = strings.TrimPrefix(s, "<")
	s = strings.TrimSuffix(s, ">")
	return s
}

// resolveThreadID determines the thread root ID.
// If References exist, the first one is the thread root.
// Otherwise, In-Reply-To is used. If neither, ownID is returned.
func resolveThreadID(references []string, inReplyTo string, ownID string) string {
	if len(references) > 0 {
		return stripAngleBrackets(references[0])
	}
	if inReplyTo != "" {
		return stripAngleBrackets(inReplyTo)
	}
	return ownID
}

// parseGoMailAddresses parses an address list from a go-message mail.Header.
func parseGoMailAddresses(header gomail.Header, key string) []types.Address {
	addrs, err := header.AddressList(key)
	if err != nil || len(addrs) == 0 {
		return nil
	}
	result := make([]types.Address, len(addrs))
	for i, a := range addrs {
		result[i] = types.Address{Name: a.Name, Email: a.Address}
	}
	return result
}
