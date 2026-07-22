package main

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunAliasAddCLIUsesSharedValidation(t *testing.T) {
	repo := newCLIRepo(t)
	withCLIInput(t, "bad-name\ngit status\n\n\n", func() {
		err := runAliasAddCLI(repo)
		if err == nil || !strings.Contains(err.Error(), "[A-Za-z_][A-Za-z0-9_]*") {
			t.Fatalf("invalid alias error = %v", err)
		}
	})
	entries, err := repo.GetAllEntries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("invalid alias was persisted: %#v", entries)
	}

	withCLIInput(t, "gs\ngit status\n\nTools\n", func() {
		if err := runAliasAddCLI(repo); err != nil {
			t.Fatal(err)
		}
	})
	entry, err := repo.GetEntryByCommand(context.Background(), "gs")
	if err != nil {
		t.Fatal(err)
	}
	if !entry.ManagedAlias || entry.Target != "git status" {
		t.Fatalf("valid alias entry = %#v", entry)
	}
}

func withCLIInput(t *testing.T, input string, fn func()) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(f, input); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	original := os.Stdin
	os.Stdin = f
	t.Cleanup(func() {
		os.Stdin = original
		_ = f.Close()
	})
	fn()
	os.Stdin = original
}
