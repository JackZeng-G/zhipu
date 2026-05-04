package store

import (
	"regexp"
	"strings"
)

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9一-龥]+`)

// Slugify converts a string to a URL-safe slug.
func Slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 100 {
		s = s[:100]
	}
	return s
}
