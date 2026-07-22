package tui

import (
	"sort"

	"github.com/charmbracelet/bubbles/textinput"

	"github.com/Architeg/gloss/internal/model"
	"github.com/Architeg/gloss/internal/storage"
)

type bulkTagFocus int

const (
	bulkTagFocusAdd bulkTagFocus = iota
	bulkTagFocusRemove
)

type bulkTagFormState struct {
	addTI    textinput.Model
	removeTI textinput.Model
	focus    bulkTagFocus
}

func newBulkTagFormState(width int) bulkTagFormState {
	mk := func(placeholder string) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = placeholder
		ti.CharLimit = 4096
		ti.Width = inputWidth(width)
		return ti
	}
	return bulkTagFormState{
		addTI:    mk("tags to add (comma-separated)"),
		removeTI: mk("tags to remove (comma-separated)"),
	}
}

func inputWidth(width int) int {
	if width < 1 {
		return 1
	}
	return width
}

func (f *bulkTagFormState) applyTheme(styles Styles) {
	for _, input := range []*textinput.Model{&f.addTI, &f.removeTI} {
		input.Prompt = "> "
		input.PromptStyle = styles.InputPrompt
		input.TextStyle = styles.InputText
		input.PlaceholderStyle = styles.InputPlaceholder
	}
}

func (f *bulkTagFormState) prepare() {
	f.addTI.SetValue("")
	f.removeTI.SetValue("")
	f.focusField(bulkTagFocusAdd)
}

func (f *bulkTagFormState) blurAll() {
	f.addTI.Blur()
	f.removeTI.Blur()
}

func (f *bulkTagFormState) focusField(focus bulkTagFocus) {
	f.blurAll()
	f.focus = focus
	if focus == bulkTagFocusRemove {
		f.removeTI.Focus()
		return
	}
	f.addTI.Focus()
}

func (f *bulkTagFormState) resize(width int) {
	width = inputWidth(width)
	f.addTI.Width = width
	f.removeTI.Width = width
}

func (f *bulkTagFormState) changes() []storage.BulkTagChange {
	remove := model.ParseTagsCSV(f.removeTI.Value())
	add := model.ParseTagsCSV(f.addTI.Value())
	changes := make([]storage.BulkTagChange, 0, len(remove)+len(add))
	// Removes run first so an overlapping add deterministically wins.
	for _, tag := range remove {
		changes = append(changes, storage.BulkTagChange{Operation: storage.BulkTagRemove, Tag: tag})
	}
	for _, tag := range add {
		changes = append(changes, storage.BulkTagChange{Operation: storage.BulkTagAdd, Tag: tag})
	}
	return changes
}

func (m *Model) selectedEntryIDs() []int64 {
	ids := make([]int64, 0, len(m.multiSelected))
	for id := range m.multiSelected {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func (m *Model) pruneMultiSelection() {
	if len(m.multiSelected) == 0 {
		return
	}
	existing := make(map[int64]struct{}, len(m.allEntries))
	for _, entry := range m.allEntries {
		if entry.ID > 0 {
			existing[entry.ID] = struct{}{}
		}
	}
	for id := range m.multiSelected {
		if _, ok := existing[id]; !ok {
			delete(m.multiSelected, id)
		}
	}
}

func (m *Model) toggleFocusedSelection() bool {
	if !m.hasBrowseSelection() {
		return false
	}
	id := m.cmdRows[m.browseCursor].Entry.ID
	if id <= 0 {
		return false
	}
	if _, selected := m.multiSelected[id]; selected {
		delete(m.multiSelected, id)
	} else {
		m.multiSelected[id] = struct{}{}
	}
	return true
}

func (m *Model) toggleVisibleSelection() bool {
	visible := make([]int64, 0, len(m.cmdRows))
	allSelected := true
	for _, row := range m.cmdRows {
		if row.Entry.ID <= 0 {
			continue
		}
		visible = append(visible, row.Entry.ID)
		if _, ok := m.multiSelected[row.Entry.ID]; !ok {
			allSelected = false
		}
	}
	if len(visible) == 0 {
		return false
	}
	for _, id := range visible {
		if allSelected {
			delete(m.multiSelected, id)
		} else {
			m.multiSelected[id] = struct{}{}
		}
	}
	return true
}

func (m *Model) isMultiSelected(id int64) bool {
	if id <= 0 {
		return false
	}
	_, ok := m.multiSelected[id]
	return ok
}
