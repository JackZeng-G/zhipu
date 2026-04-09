package nas

import (
	"strings"
	"testing"
)

func TestHTMLToText(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "empty string",
			html:     "",
			expected: "",
		},
		{
			name:     "plain text",
			html:     "Hello World",
			expected: "Hello World",
		},
		{
			name:     "simple paragraph",
			html:     "<p>Hello World</p>",
			expected: "Hello World",
		},
		{
			name:     "heading and paragraph",
			html:     "<h1>Title</h1><p>Content here</p>",
			expected: "Title\nContent here",
		},
		{
			name:     "multiple paragraphs",
			html:     "<p>First paragraph.</p><p>Second paragraph.</p>",
			expected: "First paragraph.\nSecond paragraph.",
		},
		{
			name:     "nested tags",
			html:     "<div><p><strong>Bold</strong> text</p></div>",
			expected: "Bold text",
		},
		{
			name:     "line break",
			html:     "Line 1<br/>Line 2",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "list items",
			html:     "<ul><li>Item 1</li><li>Item 2</li></ul>",
			expected: "Item 1\nItem 2",
		},
		{
			name:     "complex note content",
			html:     "<h2>Meeting Notes</h2><p>Discussed project timeline.</p><ul><li>Action item 1</li><li>Action item 2</li></ul>",
			expected: "Meeting Notes\nDiscussed project timeline.\nAction item 1\nAction item 2",
		},
		{
			name:     "script and style tags stripped",
			html:     "<script>alert('xss')</script><p>Safe content</p>",
			expected: "Safe content",
		},
		{
			name:     "HTML entities",
			html:     "<p>Tom &amp; Jerry</p>",
			expected: "Tom & Jerry",
		},
		{
			name:     "whitespace normalization",
			html:     "  <p>  Extra   spaces  </p>  ",
			expected: "Extra spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HTMLToText(tt.html)
			if result != tt.expected {
				t.Errorf("HTMLToText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHTMLToText_RealNoteContent(t *testing.T) {
	html := `<div class="note-content">
		<h1>Project Ideas</h1>
		<p>Here are some ideas for the next quarter:</p>
		<ul>
			<li>Build a personal knowledge base</li>
			<li>Learn Go programming</li㸾
			<li>Deploy to Synology NAS</li>
		</ul>
		<p><strong>Priority:</strong> High</p>
	</div>
	`

	result := HTMLToText(html)

	// Verify key content is present
	expectedParts := []string{
		"Project Ideas",
		"Here are some ideas for the next quarter:",
		"Build a personal knowledge base",
		"Learn Go programming",
		"Deploy to Synology NAS",
		"Priority:",
		"High",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("result missing expected part: %q\nGot: %q", part, result)
		}
	}

	// Verify no HTML tags remain
	if strings.Contains(result, "<") || strings.Contains(result, ">") {
		t.Errorf("result contains HTML tags: %q", result)
	}
}

func TestFormatNoteContent(t *testing.T) {
	note := &Note{
		ID:          "n1",
		Title:       "Test Note",
		ContentHTML: "<p>This is the body.</p>",
	}

	result := FormatNoteContent(note)
	expected := "Test Note\n\nThis is the body."
	if result != expected {
		t.Errorf("FormatNoteContent() = %q, want %q", result, expected)
	}

	// Test with empty body
	note.ContentHTML = ""
	result = FormatNoteContent(note)
	expected = "Test Note"
	if result != expected {
		t.Errorf("FormatNoteContent() with empty body = %q, want %q", result, expected)
	}
}

func TestStripTagsFallback(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "<p>Hello</p>",
			expected: "Hello",
		},
		{
			input:    "<a href='link'>Click</a> here",
			expected: "Click here",
		},
		{
			input:    "No tags here",
			expected: "No tags here",
		},
		{
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := stripTagsFallback(tt.input)
			if result != tt.expected {
				t.Errorf("stripTagsFallback() = %q, want %q", result, tt.expected)
			}
		})
	}
}
