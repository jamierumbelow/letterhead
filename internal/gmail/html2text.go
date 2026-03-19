package gmail

import (
	"strings"

	"golang.org/x/net/html"
)

// htmlToText converts an HTML string to readable plain text using proper
// HTML parsing. It is used as a search-index and display fallback when
// no text/plain MIME part is present.
func htmlToText(s string) string {
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return stripHTML(s) // fallback to simple stripper
	}

	var b strings.Builder
	walkNode(&b, doc)

	return collapseWhitespace(b.String())
}

func walkNode(b *strings.Builder, n *html.Node) {
	// Skip script and style elements entirely
	if n.Type == html.ElementNode {
		switch n.Data {
		case "script", "style", "noscript":
			return
		}
	}

	// Text nodes: emit content
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
	}

	// Add newlines before block-level elements
	if n.Type == html.ElementNode && isBlockElement(n.Data) {
		b.WriteString("\n")
	}

	// Handle <br> as newline
	if n.Type == html.ElementNode && n.Data == "br" {
		b.WriteString("\n")
	}

	// Recurse into children
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkNode(b, c)
	}

	// Add newline after block-level elements
	if n.Type == html.ElementNode && isBlockElement(n.Data) {
		b.WriteString("\n")
	}
}

func isBlockElement(tag string) bool {
	switch tag {
	case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6",
		"li", "ol", "ul", "tr", "table",
		"blockquote", "pre", "hr",
		"section", "article", "header", "footer", "nav",
		"main", "aside", "details", "summary", "figure", "figcaption":
		return true
	}
	return false
}

func collapseWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	var result []string
	blankCount := 0

	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r")
		if trimmed == "" {
			blankCount++
			if blankCount <= 2 {
				result = append(result, "")
			}
		} else {
			blankCount = 0
			result = append(result, trimmed)
		}
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}
