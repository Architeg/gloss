package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"

	"github.com/valeriybagrintsev/gloss/internal/model"
)

func (f *formState) applyTextInputTheme(s Styles) {
	patch := func(ti *textinput.Model) {
		ti.Prompt = "> "
		ti.PromptStyle = s.InputPrompt
		ti.TextStyle = s.InputText
		ti.PlaceholderStyle = s.InputPlaceholder
	}
	patch(&f.cmdTI)
	patch(&f.descTI)
	patch(&f.tagsTI)
}

type formMode int

const (
	formModeAdd formMode = iota
	formModeEdit
)

type formFocus int

const (
	formFocusCommand formFocus = iota
	formFocusDesc
	formFocusTags
)

type formState struct {
	mode formMode
	base model.Entry

	cmdTI  textinput.Model
	descTI textinput.Model
	tagsTI textinput.Model
	focus  formFocus
}

func newFormState(width int) formState {
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
	return formState{
		cmdTI:  mk("command"),
		descTI: mk("description"),
		tagsTI: mk("tags (comma-separated)"),
	}
}

func (f *formState) blurAll() {
	f.cmdTI.Blur()
	f.descTI.Blur()
	f.tagsTI.Blur()
}

func (f *formState) focusField(ff formFocus) {
	f.blurAll()
	f.focus = ff
	switch ff {
	case formFocusCommand:
		f.cmdTI.Focus()
	case formFocusDesc:
		f.descTI.Focus()
	case formFocusTags:
		f.tagsTI.Focus()
	}
}

func (f *formState) prepareAdd() {
	f.mode = formModeAdd
	f.base = model.Entry{}
	f.cmdTI.SetValue("")
	f.descTI.SetValue("")
	f.tagsTI.SetValue("")
	f.focusField(formFocusCommand)
}

func (f *formState) prepareEdit(e model.Entry) {
	f.mode = formModeEdit
	f.base = e
	f.cmdTI.SetValue(e.Command)
	f.descTI.SetValue(e.Description)
	f.tagsTI.SetValue(strings.Join(e.Tags, ", "))
	f.focusField(formFocusCommand)
}

func (f *formState) resizeInputs(width int) {
	if width < 20 {
		width = 40
	}
	f.cmdTI.Width = width
	f.descTI.Width = width
	f.tagsTI.Width = width
}
