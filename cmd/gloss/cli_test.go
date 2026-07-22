package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Architeg/gloss/internal/model"
	"github.com/Architeg/gloss/internal/storage"
)

func TestRunListCLITagFilterIsCaseInsensitive(t *testing.T) {
	repo := newCLIRepo(t)
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

	var baseline string
	for i, query := range []string{"tools", "TOOLS", "ToOlS"} {
		output := captureListOutput(t, repo, query)
		if strings.Contains(output, "other") || !strings.Contains(output, "alpha") || !strings.Contains(output, "zeta") {
			t.Fatalf("filter %q output = %q", query, output)
		}
		if i == 0 {
			baseline = output
		} else if output != baseline {
			t.Fatalf("filter %q output differs by casing:\n%s\nwant:\n%s", query, output, baseline)
		}
	}
}

func TestRunListCLIUsesPrimaryTagOrdering(t *testing.T) {
	repo := newCLIRepo(t)
	ctx := context.Background()
	entries := []model.Entry{
		{Command: "none", Tags: nil, Type: model.EntryTypeManual},
		{Command: "literal", Tags: []string{"Untagged"}, Type: model.EntryTypeManual},
		{Command: "zeta", Tags: []string{"alpha"}, Type: model.EntryTypeManual},
		{Command: "alpha", Tags: []string{"ALPHA"}, Type: model.EntryTypeManual},
		{Command: "beta", Tags: []string{"Beta", "alpha"}, Type: model.EntryTypeManual},
	}
	for _, entry := range entries {
		if _, err := repo.CreateEntry(ctx, entry); err != nil {
			t.Fatal(err)
		}
	}
	output := captureListOutput(t, repo, "")
	previous := -1
	for _, command := range []string{"alpha", "zeta", "beta", "literal", "none"} {
		position := strings.Index(output, command)
		if position < 0 {
			t.Fatalf("command %q missing from output %q", command, output)
		}
		if position <= previous {
			t.Fatalf("command %q is out of order in output %q", command, output)
		}
		previous = position
	}
}

func newCLIRepo(t *testing.T) *storage.EntryRepo {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "gloss.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return storage.NewEntryRepo(db)
}

func captureListOutput(t *testing.T, repo *storage.EntryRepo, tag string) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	err = runListCLI(repo, tag)
	_ = writer.Close()
	os.Stdout = original
	if err != nil {
		_ = reader.Close()
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
