package scan

import (
	"testing"

	"github.com/Architeg/gloss/internal/model"
)

func TestSuggestionToEntryUsesNormalizedEmptyTags(t *testing.T) {
	entry := SuggestionToEntry(model.ScanSuggestion{Command: " status ", Type: model.EntryTypeScript})
	if entry.Tags == nil {
		t.Fatal("SuggestionToEntry returned nil tags")
	}
	if len(entry.Tags) != 0 {
		t.Fatalf("SuggestionToEntry tags = %q, want empty", entry.Tags)
	}
}
