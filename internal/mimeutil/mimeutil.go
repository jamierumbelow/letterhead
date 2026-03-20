package mimeutil

import (
	"mime"
	"net/mail"
	"strings"

	"github.com/jamierumbelow/letterhead/pkg/types"
)

// StripHTML removes HTML tags and collapses whitespace to produce plain text.
func StripHTML(html string) string {
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
			// Insert space after block-level closing tags and <br>
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

	// Collapse whitespace
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

// ParseAddress parses a single RFC 5322 address string into a types.Address.
func ParseAddress(s string) types.Address {
	if s == "" {
		return types.Address{}
	}

	s = DecodeRFC2047(s)

	addr, err := mail.ParseAddress(s)
	if err != nil {
		return types.Address{Email: s}
	}
	return types.Address{Name: addr.Name, Email: addr.Address}
}

// ParseAddressList parses a comma-separated list of RFC 5322 addresses.
func ParseAddressList(s string) []types.Address {
	if s == "" {
		return nil
	}

	s = DecodeRFC2047(s)

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

// DecodeRFC2047 decodes RFC 2047 encoded-word strings.
func DecodeRFC2047(s string) string {
	dec := new(mime.WordDecoder)
	decoded, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return decoded
}

// GenerateSnippet returns the first maxLen characters of plainBody,
// trimmed to the last word boundary.
func GenerateSnippet(plainBody string, maxLen int) string {
	if len(plainBody) <= maxLen {
		return plainBody
	}

	truncated := plainBody[:maxLen]
	// Trim to last space to avoid cutting mid-word
	if idx := strings.LastIndex(truncated, " "); idx > 0 {
		truncated = truncated[:idx]
	}
	return truncated
}
