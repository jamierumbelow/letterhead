package mimeutil

import (
	"testing"
)

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"basic tags", "<p>Hello</p>", "Hello"},
		{"nested tags", "<div><p>Hello <b>World</b></p></div>", "Hello World"},
		{"block-level spacing", "<p>First</p><p>Second</p>", "First Second"},
		{"br tag", "Line1<br>Line2", "Line1 Line2"},
		{"empty", "", ""},
		{"no tags", "plain text", "plain text"},
		{"whitespace collapse", "<p>  lots   of   spaces  </p>", "lots of spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripHTML(tt.in)
			if got != tt.want {
				t.Errorf("StripHTML(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestGenerateSnippet(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		maxLen int
		want   string
	}{
		{"short text", "Hello World", 200, "Hello World"},
		{"empty", "", 200, ""},
		{"exact length", "Hello", 5, "Hello"},
		{"truncation at word boundary", "The quick brown fox jumps over the lazy dog", 20, "The quick brown fox"},
		{"single long word", "superlongword rest", 5, "superlongword rest"[:5]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateSnippet(tt.in, tt.maxLen)
			if len(got) > tt.maxLen && tt.name != "single long word" {
				t.Errorf("GenerateSnippet result too long: %d > %d", len(got), tt.maxLen)
			}
			if got != tt.want {
				t.Errorf("GenerateSnippet(%q, %d) = %q, want %q", tt.in, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestParseAddress(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantName  string
		wantEmail string
	}{
		{"name and email", "Alice <alice@example.com>", "Alice", "alice@example.com"},
		{"bare email", "alice@example.com", "", "alice@example.com"},
		{"rfc2047 name", "=?UTF-8?B?QWxpY2U=?= <alice@example.com>", "Alice", "alice@example.com"},
		{"empty", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := ParseAddress(tt.in)
			if addr.Name != tt.wantName {
				t.Errorf("name: got %q, want %q", addr.Name, tt.wantName)
			}
			if addr.Email != tt.wantEmail {
				t.Errorf("email: got %q, want %q", addr.Email, tt.wantEmail)
			}
		})
	}
}

func TestParseAddressList(t *testing.T) {
	addrs := ParseAddressList("Alice <alice@example.com>, Bob <bob@example.com>")
	if len(addrs) != 2 {
		t.Fatalf("expected 2 addresses, got %d", len(addrs))
	}
	if addrs[0].Email != "alice@example.com" {
		t.Errorf("first email: got %q", addrs[0].Email)
	}
	if addrs[1].Email != "bob@example.com" {
		t.Errorf("second email: got %q", addrs[1].Email)
	}
}

func TestDecodeRFC2047(t *testing.T) {
	// Base64 encoded "Hello"
	got := DecodeRFC2047("=?UTF-8?B?SGVsbG8=?=")
	if got != "Hello" {
		t.Errorf("DecodeRFC2047: got %q, want %q", got, "Hello")
	}

	// Plain string (no encoding)
	got = DecodeRFC2047("Plain text")
	if got != "Plain text" {
		t.Errorf("DecodeRFC2047: got %q, want %q", got, "Plain text")
	}
}
