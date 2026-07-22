package tui

import (
	"reflect"
	"testing"

	"github.com/Architeg/gloss/internal/model"
)

func TestFilterEntriesCaseInsensitive(t *testing.T) {
	entries := []model.Entry{
		{ID: 1, Command: "Git Status", Description: "Show working tree", Tags: []string{"Tools"}},
		{ID: 2, Command: "deploy", Description: "Push to ÉQUIPE", Tags: []string{"Release"}},
	}
	tests := []struct {
		name, search, tag string
		wantIDs           []int64
	}{
		{name: "command substring", search: "gIT st", wantIDs: []int64{1}},
		{name: "description substring", search: "équipe", wantIDs: []int64{2}},
		{name: "tag exact different case", tag: "tOoLs", wantIDs: []int64{1}},
		{name: "tag remains exact", tag: "tool", wantIDs: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterEntries(entries, tt.search, tt.tag)
			gotIDs := make([]int64, len(got))
			for i := range got {
				gotIDs[i] = got[i].ID
			}
			if !reflect.DeepEqual(gotIDs, tt.wantIDs) {
				t.Fatalf("filterEntries() IDs = %v, want %v", gotIDs, tt.wantIDs)
			}
		})
	}
}

func TestBuildCmdRowsUsesSharedOrderingAndSyntheticUntagged(t *testing.T) {
	entries := []model.Entry{
		{ID: 5, Command: "none", Tags: nil},
		{ID: 4, Command: "literal", Tags: []string{"Untagged"}},
		{ID: 3, Command: "zeta", Tags: []string{"alpha"}},
		{ID: 2, Command: "Alpha", Tags: []string{"ALPHA"}},
		{ID: 1, Command: "beta", Tags: []string{"Beta", "alpha"}},
	}
	rows := buildCmdRows(entries)
	wantIDs := []int64{2, 3, 1, 4, 5}
	for i, wantID := range wantIDs {
		if rows[i].Entry.ID != wantID {
			t.Fatalf("row %d ID = %d, want %d", i, rows[i].Entry.ID, wantID)
		}
	}
	if !rows[0].ShowGroup || rows[1].ShowGroup {
		t.Fatalf("case variants should share one group: %#v", rows[:2])
	}
	if !rows[3].ShowGroup || !rows[4].ShowGroup || rows[3].Group != "Untagged" || rows[4].Group != "Untagged" {
		t.Fatalf("literal and synthetic Untagged groups must remain distinct: %#v", rows[3:])
	}
}

func TestEntryFormsUseSharedTagNormalization(t *testing.T) {
	form := newFormState(40)
	form.prepareAdd()
	form.cmdTI.SetValue("status")
	form.tagsTI.SetValue(" Tools,Git,TOOLS,git, ")
	m := &Model{form: form}
	entry, err := m.entryFromForm()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Tools", "Git"}; !reflect.DeepEqual(entry.Tags, want) {
		t.Fatalf("entry form tags = %q, want %q", entry.Tags, want)
	}

	form.prepareEdit(model.Entry{ID: 42, Command: "status", Tags: []string{"Old"}})
	form.tagsTI.SetValue(" Shell,shell,Git,GIT ")
	m.form = form
	edited, err := m.entryFromForm()
	if err != nil {
		t.Fatal(err)
	}
	if edited.ID != 42 {
		t.Fatalf("edited entry ID = %d, want 42", edited.ID)
	}
	if want := []string{"Shell", "Git"}; !reflect.DeepEqual(edited.Tags, want) {
		t.Fatalf("edit form tags = %q, want %q", edited.Tags, want)
	}

	aliasForm := newAliasFormState(40)
	aliasForm.prepare()
	aliasForm.nameTI.SetValue("gs")
	aliasForm.targetTI.SetValue("git status")
	aliasForm.tagsTI.SetValue(" Shell,shell,Git,GIT ")
	aliasEntry, err := aliasForm.toEntry()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Shell", "Git"}; !reflect.DeepEqual(aliasEntry.Tags, want) {
		t.Fatalf("alias form tags = %q, want %q", aliasEntry.Tags, want)
	}
}
