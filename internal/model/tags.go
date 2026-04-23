package model

import "strings"

// ParseTagsCSV splits comma-separated tags, trims spaces, and drops empties.
func ParseTagsCSV(s string) []string {
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
