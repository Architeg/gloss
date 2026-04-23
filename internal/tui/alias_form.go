package tui

import (
	"errors"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"

	"github.com/valeriybagrintsev/gloss/internal/model"
)

type aliasField int

const (
	aliasFieldName aliasField = iota
	aliasFieldTarget
	aliasFieldDesc
	aliasFieldTags
)

type aliasFormState struct {
	nameTI   textinput.Model
	targetTI textinput.Model
	descTI   textinput.Model
	tagsTI   textinput.Model
	focus    aliasField
}

func newAliasFormState(width int) aliasFormState {
	w := width
	if w < 20 {
		w = 40
	}
	mk := func(ph string) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = ph
		ti.CharLimit = 4096
		ti.Width = w
		return ti
	}
	return aliasFormState{
		nameTI:   mk("alias name"),
		targetTI: mk("expansion"),
		descTI:   mk("optional"),
		tagsTI:   mk("comma-separated"),
	}
}

func (f *aliasFormState) applyTheme(s Styles) {
	patch := func(ti *textinput.Model) {
		ti.Prompt = "> "
		ti.PromptStyle = s.InputPrompt
		ti.TextStyle = s.InputText
		ti.PlaceholderStyle = s.InputPlaceholder
	}
	patch(&f.nameTI)
	patch(&f.targetTI)
	patch(&f.descTI)
	patch(&f.tagsTI)
}

func (f *aliasFormState) blurAll() {
	f.nameTI.Blur()
	f.targetTI.Blur()
	f.descTI.Blur()
	f.tagsTI.Blur()
}

func (f *aliasFormState) focusField(ff aliasField) {
	f.blurAll()
	f.focus = ff
	switch ff {
	case aliasFieldName:
		f.nameTI.Focus()
	case aliasFieldTarget:
		f.targetTI.Focus()
	case aliasFieldDesc:
		f.descTI.Focus()
	case aliasFieldTags:
		f.tagsTI.Focus()
	}
}

func (f *aliasFormState) prepare() {
	f.nameTI.SetValue("")
	f.targetTI.SetValue("")
	f.descTI.SetValue("")
	f.tagsTI.SetValue("")
	f.focusField(aliasFieldName)
}

func (f *aliasFormState) cycleFocus(delta int) {
	next := int(f.focus) + delta
	for next < 0 {
		next += 4
	}
	next %= 4
	f.focusField(aliasField(next))
}

func (f *aliasFormState) resize(width int) {
	if width < 20 {
		width = 40
	}
	f.nameTI.Width = width
	f.targetTI.Width = width
	f.descTI.Width = width
	f.tagsTI.Width = width
}

func (f *aliasFormState) toEntry() (model.Entry, error) {
	name := model.NormalizeCommand(strings.TrimSpace(f.nameTI.Value()))
	if name == "" {
		return model.Entry{}, errors.New("alias name is required")
	}
	target := strings.TrimSpace(f.targetTI.Value())
	if target == "" {
		return model.Entry{}, errors.New("expansion is required")
	}
	desc := strings.TrimSpace(f.descTI.Value())
	if desc == "" {
		desc = target
	}
	return model.Entry{
		Command:      name,
		Description:  desc,
		Tags:         model.ParseTagsCSV(f.tagsTI.Value()),
		Type:         model.EntryTypeAlias,
		Source:       "managed",
		Target:       target,
		ManagedAlias: true,
	}, nil
}
