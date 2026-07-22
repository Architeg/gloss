package tui

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Architeg/gloss/internal/model"
	"github.com/Architeg/gloss/internal/storage"
)

func TestImportScanCmdImportsOnlySelectedRowsAtomically(t *testing.T) {
	repo := newMessageTestRepo(t)
	cmd := importScanCmd(repo, []model.ScanSuggestion{
		{Command: "first", Type: model.EntryTypeScript, Selected: true},
		{Command: "ignored", Type: model.EntryTypeScript, Selected: false},
		{Command: "second", Type: model.EntryTypeFunction, Selected: true},
	})
	msg, ok := cmd().(importScanMsg)
	if !ok {
		t.Fatalf("import command returned unexpected message type")
	}
	if msg.err != nil || msg.imported != 2 {
		t.Fatalf("import message = %#v, want imported 2 with no error", msg)
	}
	entries, err := repo.GetAllEntries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("stored entries = %#v, want two selected rows", entries)
	}
	for _, entry := range entries {
		if entry.Command == "ignored" {
			t.Fatal("unselected suggestion was imported")
		}
	}
}

func TestImportScanCmdZeroSelectionSucceeds(t *testing.T) {
	repo := newMessageTestRepo(t)
	cmd := importScanCmd(repo, []model.ScanSuggestion{{Command: "ignored", Type: model.EntryTypeScript}})
	msg := cmd().(importScanMsg)
	if msg.err != nil || msg.imported != 0 {
		t.Fatalf("zero-selection message = %#v", msg)
	}
}

func TestImportScanCmdFailureRollsBackAndReportsZero(t *testing.T) {
	repo := newMessageTestRepo(t)
	cmd := importScanCmd(repo, []model.ScanSuggestion{
		{Command: "duplicate", Type: model.EntryTypeScript, Selected: true},
		{Command: "duplicate", Type: model.EntryTypeFunction, Selected: true},
	})
	msg := cmd().(importScanMsg)
	if msg.err == nil || msg.imported != 0 {
		t.Fatalf("failed import message = %#v, want error and imported 0", msg)
	}
	entries, err := repo.GetAllEntries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed scan import partially committed: %#v", entries)
	}
}

func newMessageTestRepo(t *testing.T) *storage.EntryRepo {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "gloss.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return storage.NewEntryRepo(db)
}
