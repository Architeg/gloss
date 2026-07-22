package model

import (
	"sort"
	"strings"
)

// NormalizeTags trims tags, removes empty values and case-insensitive
// duplicates, and preserves the spelling and position of each first occurrence.
// It always returns a non-nil slice.
func NormalizeTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		duplicate := false
		for _, existing := range out {
			if strings.EqualFold(existing, tag) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			out = append(out, tag)
		}
	}
	return out
}

// ParseTagsCSV splits comma-separated tags and applies NormalizeTags.
func ParseTagsCSV(s string) []string {
	return NormalizeTags(strings.Split(s, ","))
}

// PrimaryTag returns the first normalized tag. The bool is false only when the
// entry has no non-empty tags.
func PrimaryTag(e Entry) (string, bool) {
	tags := NormalizeTags(e.Tags)
	if len(tags) == 0 {
		return "", false
	}
	return tags[0], true
}

// IsUntagged reports whether an entry has no non-empty tags.
func IsUntagged(e Entry) bool {
	_, ok := PrimaryTag(e)
	return !ok
}

// EqualTag compares tag names using Unicode case folding after trimming.
func EqualTag(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// EntryHasTag reports whether an entry contains tag. Matching is exact apart
// from surrounding whitespace and Unicode case folding.
func EntryHasTag(e Entry, tag string) bool {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return false
	}
	for _, candidate := range NormalizeTags(e.Tags) {
		if EqualTag(candidate, tag) {
			return true
		}
	}
	return false
}

// ContainsFold reports whether substr occurs in s using Unicode case folding.
// It preserves ordinary substring semantics rather than changing to token or
// prefix matching.
func ContainsFold(s, substr string) bool {
	if substr == "" {
		return true
	}
	haystack := []rune(s)
	needle := []rune(substr)
	if len(needle) > len(haystack) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if strings.EqualFold(string(haystack[i:i+len(needle)]), substr) {
			return true
		}
	}
	return false
}

// SortEntriesByPrimaryTag returns a sorted copy of entries. Tags in the copy
// are normalized; the caller's entries and tag slices are not mutated.
func SortEntriesByPrimaryTag(entries []Entry) []Entry {
	out := make([]Entry, len(entries))
	for i, entry := range entries {
		out[i] = entry
		out[i].Tags = NormalizeTags(entry.Tags)
	}
	sort.SliceStable(out, func(i, j int) bool {
		leftTag, leftTagged := PrimaryTag(out[i])
		rightTag, rightTagged := PrimaryTag(out[j])
		if leftTagged != rightTagged {
			return leftTagged
		}
		if leftTagged && !EqualTag(leftTag, rightTag) {
			return compareFold(leftTag, rightTag) < 0
		}
		if !strings.EqualFold(out[i].Command, out[j].Command) {
			return compareFold(out[i].Command, out[j].Command) < 0
		}
		if out[i].Command != out[j].Command {
			return out[i].Command < out[j].Command
		}
		if leftTag != rightTag {
			return leftTag < rightTag
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func compareFold(a, b string) int {
	left := strings.ToLower(a)
	right := strings.ToLower(b)
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
