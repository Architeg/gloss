package model

import (
	"reflect"
	"testing"
)

func TestNormalizeTags(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{name: "trim blanks and duplicates", input: []string{" tools ", "Git", "TOOLS", "", "git", "Shell"}, want: []string{"tools", "Git", "Shell"}},
		{name: "preserve order not alphabetical", input: []string{"zeta", "Alpha", "beta"}, want: []string{"zeta", "Alpha", "beta"}},
		{name: "unicode case pair", input: []string{" Équipe ", "éQUIPE", "Autre"}, want: []string{"Équipe", "Autre"}},
		{name: "nil", input: nil, want: []string{}},
		{name: "empty", input: []string{}, want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeTags(tt.input)
			if got == nil {
				t.Fatal("NormalizeTags returned nil")
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("NormalizeTags(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTagsCSVUsesNormalization(t *testing.T) {
	want := []string{"Tools", "Git"}
	if got := ParseTagsCSV(" Tools, Git,TOOLS, ,git "); !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseTagsCSV() = %q, want %q", got, want)
	}
}

func TestPrimaryTag(t *testing.T) {
	tests := []struct {
		name   string
		entry  Entry
		want   string
		tagged bool
	}{
		{name: "first normalized tag", entry: Entry{Tags: []string{" ", " Tools ", "Git"}}, want: "Tools", tagged: true},
		{name: "none", entry: Entry{Tags: []string{"", " "}}, tagged: false},
		{name: "literal untagged", entry: Entry{Tags: []string{"Untagged"}}, want: "Untagged", tagged: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, tagged := PrimaryTag(tt.entry)
			if got != tt.want || tagged != tt.tagged {
				t.Fatalf("PrimaryTag() = (%q, %v), want (%q, %v)", got, tagged, tt.want, tt.tagged)
			}
			if IsUntagged(tt.entry) == tt.tagged {
				t.Fatalf("IsUntagged() inconsistent with PrimaryTag tagged=%v", tagged)
			}
		})
	}
}

func TestEntryHasTagIsCaseInsensitiveAndExact(t *testing.T) {
	entry := Entry{Tags: []string{" Tools ", "Équipe"}}
	for _, query := range []string{"tools", "TOOLS", "éQUIPE"} {
		if !EntryHasTag(entry, query) {
			t.Fatalf("EntryHasTag() did not match %q", query)
		}
	}
	for _, query := range []string{"tool", "quipe", ""} {
		if EntryHasTag(entry, query) {
			t.Fatalf("EntryHasTag() unexpectedly matched %q", query)
		}
	}
}

func TestSortEntriesByPrimaryTag(t *testing.T) {
	input := []Entry{
		{ID: 6, Command: "untagged", Tags: nil},
		{ID: 5, Command: "literal", Tags: []string{"Untagged"}},
		{ID: 4, Command: "beta", Tags: []string{"Beta", "alpha"}},
		{ID: 1, Command: "zeta", Tags: []string{" alpha ", "extra"}},
		{ID: 3, Command: "alpha", Tags: []string{"ALPHA"}},
		{ID: 2, Command: "Alpha", Tags: []string{"alpha", "ALPHA", ""}},
	}
	original := make([]Entry, len(input))
	copy(original, input)
	for i := range input {
		original[i].Tags = append([]string(nil), input[i].Tags...)
	}

	got := SortEntriesByPrimaryTag(input)
	wantIDs := []int64{2, 3, 1, 4, 5, 6}
	for i, wantID := range wantIDs {
		if got[i].ID != wantID {
			t.Fatalf("sorted ID at %d = %d, want %d", i, got[i].ID, wantID)
		}
	}
	if !reflect.DeepEqual(input, original) {
		t.Fatalf("SortEntriesByPrimaryTag mutated input: got %#v, want %#v", input, original)
	}
	if want := []string{"alpha"}; !reflect.DeepEqual(got[0].Tags, want) {
		t.Fatalf("sorted copy tags = %q, want normalized %q", got[0].Tags, want)
	}
}

func TestSortEntriesDeterministicCaseTies(t *testing.T) {
	input := []Entry{
		{ID: 3, Command: "same", Tags: []string{"tag"}},
		{ID: 1, Command: "same", Tags: []string{"tag"}},
		{ID: 2, Command: "Same", Tags: []string{"Tag"}},
	}
	first := SortEntriesByPrimaryTag(input)
	second := SortEntriesByPrimaryTag(input)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("case-tie ordering is not deterministic: %#v vs %#v", first, second)
	}
	wantIDs := []int64{2, 1, 3}
	for i, wantID := range wantIDs {
		if first[i].ID != wantID {
			t.Fatalf("case-tie ID at %d = %d, want %d", i, first[i].ID, wantID)
		}
	}
}

func TestContainsFold(t *testing.T) {
	tests := []struct {
		name, text, query string
		want              bool
	}{
		{name: "ASCII substring", text: "Show Git Status", query: "gIT st", want: true},
		{name: "Unicode substring", text: "Équipe outils", query: "éQUIPE", want: true},
		{name: "not prefix only", text: "prefix-middle-suffix", query: "MIDDLE", want: true},
		{name: "no match", text: "git status", query: "push", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsFold(tt.text, tt.query); got != tt.want {
				t.Fatalf("ContainsFold(%q, %q) = %v, want %v", tt.text, tt.query, got, tt.want)
			}
		})
	}
}
