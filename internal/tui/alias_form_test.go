package tui

import (
	"strings"
	"testing"

	"github.com/Architeg/gloss/internal/model"
)

func TestAliasFormUsesSharedNameValidation(t *testing.T) {
	form := newAliasFormState(40)
	form.prepare()
	form.nameTI.SetValue("bad-name")
	form.targetTI.SetValue("git status")
	if _, err := form.toEntry(); err == nil || !strings.Contains(err.Error(), "[A-Za-z_][A-Za-z0-9_]*") {
		t.Fatalf("invalid alias error = %v", err)
	}

	form.nameTI.SetValue("_valid2")
	entry, err := form.toEntry()
	if err != nil {
		t.Fatal(err)
	}
	if entry.Command != "_valid2" || !entry.ManagedAlias || entry.Type != model.EntryTypeAlias {
		t.Fatalf("valid alias entry = %#v", entry)
	}
}

func TestManagedAliasGenericEditUsesSharedNameValidation(t *testing.T) {
	form := newFormState(40)
	form.prepareEdit(model.Entry{
		ID: 1, Command: "valid", Type: model.EntryTypeAlias, ManagedAlias: true,
	})
	form.cmdTI.SetValue("invalid name")
	m := &Model{form: form}
	if _, err := m.entryFromForm(); err == nil || !strings.Contains(err.Error(), "[A-Za-z_][A-Za-z0-9_]*") {
		t.Fatalf("invalid managed-alias edit error = %v", err)
	}
}
