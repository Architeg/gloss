package storage

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Architeg/gloss/internal/alias"
	"github.com/Architeg/gloss/internal/model"
)

func TestRepositoryRejectsInvalidManagedAliasNames(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	_, err := repo.CreateEntry(ctx, model.Entry{
		Command: "bad-name", Type: model.EntryTypeAlias, ManagedAlias: true, Target: "true",
	})
	if err == nil || !strings.Contains(err.Error(), "[A-Za-z_][A-Za-z0-9_]*") {
		t.Fatalf("invalid managed alias create error = %v", err)
	}

	id, err := repo.CreateEntry(ctx, model.Entry{
		Command: "good", Type: model.EntryTypeAlias, ManagedAlias: true, Target: "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	entry, err := repo.GetEntryByCommand(ctx, "good")
	if err != nil {
		t.Fatal(err)
	}
	entry.ID = id
	entry.Command = "bad name"
	if err := repo.UpdateEntry(ctx, entry); err == nil {
		t.Fatal("invalid managed alias update unexpectedly succeeded")
	}
	stored, err := repo.GetEntryByCommand(ctx, "good")
	if err != nil || stored.Command != "good" {
		t.Fatalf("failed update changed stored alias: %#v, %v", stored, err)
	}

	if _, err := repo.CreateEntry(ctx, model.Entry{Command: "unmanaged-name", Type: model.EntryTypeManual}); err != nil {
		t.Fatalf("unmanaged command was subjected to alias validation: %v", err)
	}
}

func TestLegacyInvalidManagedAliasCannotModifyShellFile(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	id, err := repo.CreateEntry(ctx, model.Entry{
		Command: "bad-name", Type: model.EntryTypeAlias, Target: "true", ManagedAlias: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.db.ExecContext(ctx, `UPDATE entries SET managed_alias = 1 WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	entries, err := repo.GetManagedAliases(ctx)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")
	original := []byte("export KEEP=1\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := alias.Sync(path, entries, 5); err == nil || !strings.Contains(err.Error(), "bad-name") {
		t.Fatalf("legacy invalid alias sync error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatalf("legacy invalid alias changed shell file: %q, %v", data, err)
	}
	backups, err := filepath.Glob(path + ".gloss.bak-*")
	if err != nil || len(backups) != 0 {
		t.Fatalf("legacy invalid alias created backup: %v, %v", backups, err)
	}
}
