package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Architeg/gloss/internal/model"
)

func TestUpdateEntryUsesFreshUTCTimestamp(t *testing.T) {
	for _, tt := range []struct {
		name            string
		callerUpdatedAt time.Time
	}{
		{name: "stale caller timestamp", callerUpdatedAt: time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)},
		{name: "future caller timestamp", callerUpdatedAt: time.Now().UTC().Add(24 * time.Hour)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := newTestRepo(t)
			ctx := context.Background()
			old := time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)
			id, err := repo.CreateEntry(ctx, model.Entry{
				Command:   "status",
				Type:      model.EntryTypeManual,
				CreatedAt: old,
				UpdatedAt: old,
			})
			if err != nil {
				t.Fatal(err)
			}
			entry, err := repo.GetEntryByCommand(ctx, "status")
			if err != nil {
				t.Fatal(err)
			}
			entry.Description = "updated"
			entry.UpdatedAt = tt.callerUpdatedAt
			before := time.Now().UTC()
			if err := repo.UpdateEntry(ctx, entry); err != nil {
				t.Fatal(err)
			}
			after := time.Now().UTC()
			updated, err := repo.GetEntryByCommand(ctx, "status")
			if err != nil {
				t.Fatal(err)
			}
			if updated.ID != id || !updated.CreatedAt.Equal(old) {
				t.Fatalf("identity or created_at changed: %#v", updated)
			}
			if updated.UpdatedAt.Before(before) || updated.UpdatedAt.After(after) {
				t.Fatalf("updated_at %s is outside [%s, %s]", updated.UpdatedAt, before, after)
			}
			if updated.UpdatedAt.Location() != time.UTC {
				t.Fatalf("updated_at location = %v, want UTC", updated.UpdatedAt.Location())
			}
		})
	}
}

func TestCreateEntriesSuccessAndEmptyInput(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	empty, err := repo.CreateEntries(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty CreateEntries IDs = %#v, want non-nil empty slice", empty)
	}

	ids, err := repo.CreateEntries(ctx, []model.Entry{
		{Command: "second", Tags: []string{" Tools ", "TOOLS"}, Type: model.EntryTypeManual},
		{Command: "first", Tags: []string{"Git"}, Type: model.EntryTypeManual},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] >= ids[1] {
		t.Fatalf("CreateEntries IDs = %v, want input-order IDs", ids)
	}
	entry, err := repo.GetEntryByCommand(ctx, "second")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Tools"}; !reflect.DeepEqual(entry.Tags, want) {
		t.Fatalf("batch-normalized tags = %q, want %q", entry.Tags, want)
	}
}

func TestCreateEntriesRollsBackAfterLaterFailure(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	ids, err := repo.CreateEntries(ctx, []model.Entry{
		{Command: "duplicate", Type: model.EntryTypeManual},
		{Command: "duplicate", Type: model.EntryTypeManual},
	})
	if err == nil {
		t.Fatal("CreateEntries unexpectedly succeeded")
	}
	if ids != nil {
		t.Fatalf("failed CreateEntries IDs = %v, want nil", ids)
	}
	if !strings.Contains(err.Error(), "item 2") {
		t.Fatalf("batch error lacks failing item context: %v", err)
	}
	entries, loadErr := repo.GetAllEntries(ctx)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(entries) != 0 {
		t.Fatalf("partial batch rows remained: %#v", entries)
	}
}

func TestBulkUpdateTagOperations(t *testing.T) {
	tests := []struct {
		name      string
		initial   []string
		operation BulkTagOperation
		tag       string
		want      []string
	}{
		{name: "set existing primary preserves spelling", initial: []string{"Alpha", "Tools", "Shell"}, operation: BulkTagSetPrimary, tag: "tools", want: []string{"Tools", "Alpha", "Shell"}},
		{name: "set new primary", initial: []string{"Alpha", "Shell"}, operation: BulkTagSetPrimary, tag: " New ", want: []string{"New", "Alpha", "Shell"}},
		{name: "add existing does not move", initial: []string{"Alpha", "Tools"}, operation: BulkTagAdd, tag: "TOOLS", want: []string{"Alpha", "Tools"}},
		{name: "add new appends", initial: []string{"Alpha", "Tools"}, operation: BulkTagAdd, tag: "Shell", want: []string{"Alpha", "Tools", "Shell"}},
		{name: "remove different case", initial: []string{"Alpha", "Tools", "Shell"}, operation: BulkTagRemove, tag: "tOoLs", want: []string{"Alpha", "Shell"}},
		{name: "remove primary promotes next", initial: []string{"Alpha", "Tools", "Shell"}, operation: BulkTagRemove, tag: "ALPHA", want: []string{"Tools", "Shell"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newTestRepo(t)
			ctx := context.Background()
			old := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
			id, err := repo.CreateEntry(ctx, model.Entry{Command: "entry", Tags: tt.initial, Type: model.EntryTypeManual, UpdatedAt: old})
			if err != nil {
				t.Fatal(err)
			}
			before := time.Now().UTC()
			if err := repo.BulkUpdateTag(ctx, []int64{id, id}, tt.operation, tt.tag); err != nil {
				t.Fatal(err)
			}
			after := time.Now().UTC()
			entry, err := repo.GetEntryByCommand(ctx, "entry")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(entry.Tags, tt.want) {
				t.Fatalf("tags = %q, want %q", entry.Tags, tt.want)
			}
			changed := !reflect.DeepEqual(model.NormalizeTags(tt.initial), tt.want)
			if changed && (entry.UpdatedAt.Before(before) || entry.UpdatedAt.After(after)) {
				t.Fatalf("changed updated_at %s is outside [%s, %s]", entry.UpdatedAt, before, after)
			}
			if !changed && !entry.UpdatedAt.Equal(old) {
				t.Fatalf("no-op updated_at = %s, want %s", entry.UpdatedAt, old)
			}
		})
	}
}

func TestBulkUpdateTagValidationAndMissingIDRollback(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	firstID, err := repo.CreateEntry(ctx, model.Entry{Command: "first", Tags: []string{"Old"}, Type: model.EntryTypeManual})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.BulkUpdateTag(ctx, nil, BulkTagAdd, ""); err != nil {
		t.Fatalf("empty ID list was not a no-op: %v", err)
	}
	if err := repo.BulkUpdateTag(ctx, []int64{firstID}, BulkTagAdd, " "); err == nil {
		t.Fatal("blank tag was accepted")
	}
	for _, ids := range [][]int64{{0}, {-1}} {
		if err := repo.BulkUpdateTag(ctx, ids, BulkTagAdd, "New"); err == nil {
			t.Fatalf("invalid IDs %v were accepted", ids)
		}
	}

	err = repo.BulkUpdateTag(ctx, []int64{firstID, firstID + 9999}, BulkTagAdd, "New")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing ID error = %v, want sql.ErrNoRows", err)
	}
	entry, err := repo.GetEntryByCommand(ctx, "first")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Old"}; !reflect.DeepEqual(entry.Tags, want) {
		t.Fatalf("missing-ID operation partially committed: tags = %q, want %q", entry.Tags, want)
	}
}

func newTestRepo(t *testing.T) *EntryRepo {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "gloss.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewEntryRepo(db)
}
