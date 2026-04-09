package nas

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// blockElements is the set of HTML elements that produce line breaks.
var blockElements = map[string]bool{
	"p": true, "div": true, "br": true, "h1": true, "h2": true, "h3": true,
	"h4": true, "h5": true, "h6": true, "li": true, "tr": true, "hr": true,
	"blockquote": true, "pre": true, "table": true, "ul": true, "ol": true,
}

// skipElements is the set of elements whose text content should be ignored.
var skipElements = map[string]bool{
	"script": true, "style": true, "noscript": true,
}

// HTMLToText strips HTML tags and returns plain text content.
// It parses the HTML properly and extracts text nodes, preserving
// readability by adding newlines for block-level elements.
func HTMLToText(htmlContent string) string {
	if htmlContent == "" {
		return ""
	}

	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return stripTagsFallback(htmlContent)
	}

	var b strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		// Skip content of script/style elements entirely
		if n.Type == html.ElementNode && skipElements[n.Data] {
			return
		}

		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				if b.Len() > 0 {
					last := b.String()[b.Len()-1]
					if last == '\n' {
						// Already at newline, just append text
					} else {
						b.WriteByte(' ')
					}
				}
				b.WriteString(text)
			}
		}

		// Add newline before block-level elements
		if n.Type == html.ElementNode && blockElements[n.Data] {
			if b.Len() > 0 && b.String()[b.Len()-1] != '\n' {
				b.WriteByte('\n')
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}

		// Add newline after closing block-level elements
		if n.Type == html.ElementNode && blockElements[n.Data] {
			if b.Len() > 0 && b.String()[b.Len()-1] != '\n' {
				b.WriteByte('\n')
			}
		}
	}
	f(doc)

	// Post-process: normalize multiple spaces within each line
	result := b.String()
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		var sb strings.Builder
		prevSpace := false
		for _, r := range line {
			if r == ' ' || r == '\t' {
				if !prevSpace {
					sb.WriteByte(' ')
				}
				prevSpace = true
			} else {
				sb.WriteRune(r)
				prevSpace = false
			}
		}
		lines[i] = strings.TrimSpace(sb.String())
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// stripTagsFallback is a simple fallback that removes text between < and >.
func stripTagsFallback(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// FormatNoteContent formats a note for display by converting its HTML body to plain text.
func FormatNoteContent(note *Note) string {
	title := note.Title
	text := HTMLToText(note.ContentHTML)
	if text == "" {
		return title
	}
	return fmt.Sprintf("%s\n\n%s", title, text)
}
