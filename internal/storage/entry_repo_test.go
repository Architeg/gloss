package storage

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Architeg/gloss/internal/model"
)

func TestEntryRepoNormalizesTagsAtWriteBoundaries(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "gloss.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := NewEntryRepo(db)
	ctx := context.Background()

	id, err := repo.CreateEntry(ctx, model.Entry{
		Command: "status",
		Tags:    []string{" tools ", "Git", "TOOLS", "", "git"},
		Type:    model.EntryTypeManual,
	})
	if err != nil {
		t.Fatal(err)
	}
	entry, err := repo.GetEntryByCommand(ctx, "status")
	if err != nil {
		t.Fatal(err)
	}
	if entry.ID != id {
		t.Fatalf("entry ID = %d, want %d", entry.ID, id)
	}
	if want := []string{"tools", "Git"}; !reflect.DeepEqual(entry.Tags, want) {
		t.Fatalf("created tags = %q, want %q", entry.Tags, want)
	}

	entry.Tags = []string{" Shell ", "shell", "Git", "git", ""}
	if err := repo.UpdateEntry(ctx, entry); err != nil {
		t.Fatal(err)
	}
	updated, err := repo.GetEntryByCommand(ctx, "status")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Shell", "Git"}; !reflect.DeepEqual(updated.Tags, want) {
		t.Fatalf("updated tags = %q, want %q", updated.Tags, want)
	}
}

func TestEntryRepoTagFilterIsCaseInsensitiveAndExact(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "gloss.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := NewEntryRepo(db)
	ctx := context.Background()
	for _, entry := range []model.Entry{
		{Command: "zeta", Tags: []string{"Tools"}, Type: model.EntryTypeManual},
		{Command: "alpha", Tags: []string{"TOOLS"}, Type: model.EntryTypeManual},
		{Command: "other", Tags: []string{"Toolsmith"}, Type: model.EntryTypeManual},
	} {
		if _, err := repo.CreateEntry(ctx, entry); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{"tools", "TOOLS", "ToOlS"} {
		got, err := repo.GetEntriesByTag(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].Command != "alpha" || got[1].Command != "zeta" {
			t.Fatalf("filter %q returned %#v", query, got)
		}
	}
}
