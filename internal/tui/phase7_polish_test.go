package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Architeg/gloss/internal/model"
)

func TestCommandSummaryWordingAndSelectionCounts(t *testing.T) {
	m := newCommandTestModel(commandEntries(14), 8)
	m.selectBrowseIndex(8)
	if got := m.commandStatusBlock(76); !strings.Contains(got, "Item 9 of 14") {
		t.Fatalf("summary = %q, want Item 9 of 14", got)
	}
	m.multiSelected[1] = struct{}{}
	if got := m.commandStatusBlock(76); !strings.Contains(got, "1 selected") {
		t.Fatalf("singular selection summary = %q", got)
	}
	m.multiSelected[2] = struct{}{}
	if got := m.commandStatusBlock(76); !strings.Contains(got, "2 selected") {
		t.Fatalf("plural selection summary = %q", got)
	}
}

func TestCommandSummarySeparationIsOptionalAndBudgeted(t *testing.T) {
	m := newCommandTestModel(commandEntries(3), 8)
	width := m.commandContentWidth()
	height, separate := m.commandViewportLayout(width)
	if !separate {
		t.Fatal("normal command viewport omitted summary/category separation")
	}
	if want := m.mainContentHeight() - lipgloss.Height(m.commandsBrowseFixedBlock(width)) - 1; height != want {
		t.Fatalf("separated list height = %d, want %d", height, want)
	}
	view := m.commandsBrowseView(width)
	summary := strings.Index(view, "Item 1 of 3")
	category := strings.Index(view, "Category:")
	if summary < 0 || category < 0 || !strings.Contains(view[summary:category], "\n\n") {
		t.Fatalf("summary/category are not separated by a blank row: %q", view)
	}

	setCommandListHeight(m, 1)
	height, separate = m.commandViewportLayout(width)
	if separate || height != 1 {
		t.Fatalf("short layout height/separation = %d/%v, want 1/false", height, separate)
	}
	shortView := m.commandsBrowseView(width)
	if !strings.Contains(shortView, m.cmdRows[m.browseCursor].Entry.Command) {
		t.Fatalf("short viewport hid focused command: %q", shortView)
	}
	if got, limit := lipgloss.Height(shortView), m.mainContentHeight(); got > limit {
		t.Fatalf("short view height = %d, limit %d", got, limit)
	}
}

func TestVeryShortCommandScreenKeepsUsefulListContent(t *testing.T) {
	m := newCommandTestModel(commandEntries(3), 3)
	m.width = 40
	m.height = 10
	width := m.commandContentWidth()
	_, separate := m.commandViewportLayout(width)
	if separate {
		t.Fatal("very short screen retained optional summary/category separation")
	}
	view := m.commandsBrowseView(width)
	if !strings.Contains(view, m.cmdRows[m.browseCursor].Entry.Command) {
		t.Fatalf("very short screen omitted focused command: %q", view)
	}
	if got, limit := lipgloss.Height(view), m.mainContentHeight(); got > limit {
		t.Fatalf("very short view height = %d, limit %d", got, limit)
	}
}

func TestPreferredColumnGapOrderAndAliasSpacing(t *testing.T) {
	tests := []struct {
		width int
		gap   int
	}{
		{width: 76, gap: 4},
		{width: 60, gap: 3},
		{width: 40, gap: 2},
		{width: 24, gap: 1},
		{width: 0, gap: 0},
	}
	for _, tt := range tests {
		if got := preferredColumnGap(tt.width); got != tt.gap {
			t.Fatalf("preferredColumnGap(%d) = %d, want %d", tt.width, got, tt.gap)
		}
	}
	marker, command, gap, target := responsiveColumnWidths(76, 2, 22, 8, 8)
	if marker != 2 || command != 22 || gap != 4 || marker+command+gap+target != 76 {
		t.Fatalf("wide alias columns = %d/%d/%d/%d", marker, command, gap, target)
	}
}

func TestAdaptiveCommandFooterLabelsAndDiscoverability(t *testing.T) {
	m := newCommandTestModel(commandEntries(2), 3)
	m.width = 400
	wide := m.footerContent()
	for _, label := range []string{
		"↑↓ Navigate", "Space Select", "Ctrl+A All visible", "T Bulk tags",
		"C Copy", "/ Search", "F Filter", "[ ] Categories",
		"Home/End First/last", "PgUp/PgDn Page", "Enter Details",
		"Esc Back", "Q Quit", "? Help",
	} {
		if !strings.Contains(wide, label) {
			t.Fatalf("wide footer missing %q: %q", label, wide)
		}
	}
	if strings.Contains(wide, "Groups") || strings.Contains(wide, "Bounds") || strings.Contains(wide, "^A") {
		t.Fatalf("wide footer retained ambiguous terminology: %q", wide)
	}

	for _, width := range []int{7, 8, 12, 20, 40, 80} {
		m.width = width
		footer := m.footerContent()
		if !strings.Contains(footer, "?") {
			t.Fatalf("footer width %d lost help discoverability: %q", width, footer)
		}
		if got, limit := lipgloss.Width(footer), m.footerAvailableWidth(); got > limit {
			t.Fatalf("footer width %d rendered %d, limit %d: %q", width, got, limit, footer)
		}
	}
}

func TestCommandHelpOpenCloseAndStatePreservation(t *testing.T) {
	m := newCommandTestModel(commandEntries(12), 3)
	fake := &fakeClipboard{}
	m.clip = fake
	m.selectBrowseIndex(8)
	m.multiSelected[2] = struct{}{}
	m.multiSelected[9] = struct{}{}
	m.searchTI.SetValue("command")
	m.tagTI.SetValue("Group")
	m.rebuildBrowse()
	m.browseOffset = max(m.browseCursor-1, 0)
	m.commandStatus.text = "Copied"

	wantID := m.selectedID
	wantCursor := m.browseCursor
	wantOffset := m.browseOffset
	wantSearch := m.searchTI.Value()
	wantTag := m.tagTI.Value()
	wantSelected := map[int64]struct{}{2: {}, 9: {}}

	_, _ = m.updateCommands(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !m.commandHelpOpen || !strings.Contains(m.commandsMainView(m.commandContentWidth()), "Command shortcuts") {
		t.Fatal("? did not open command help")
	}
	_, _ = m.updateCommands(tea.KeyMsg{Type: tea.KeySpace})
	_, _ = m.updateCommands(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
	_, _ = m.updateCommands(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	_, _ = m.updateCommands(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	_, _ = m.updateCommands(tea.KeyMsg{Type: tea.KeyEnter})
	if m.cmdPhase != commandsBrowse || m.cmdFocus != commandsFocusList || len(m.multiSelected) != len(wantSelected) || fake.value != "" {
		t.Fatalf("list action ran while help was open: phase=%d selection=%v", m.cmdPhase, m.multiSelected)
	}

	_, _ = m.updateCommands(tea.KeyMsg{Type: tea.KeyEsc})
	if m.commandHelpOpen {
		t.Fatal("Esc did not close command help")
	}
	assertCommandHelpPreservedState(t, m, wantID, wantCursor, wantOffset, wantSearch, wantTag, wantSelected)

	_, _ = m.updateCommands(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	_, _ = m.updateCommands(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if m.commandHelpOpen {
		t.Fatal("second ? did not close command help")
	}
	assertCommandHelpPreservedState(t, m, wantID, wantCursor, wantOffset, wantSearch, wantTag, wantSelected)
}

func TestCommandHelpListsActualBindingsAndScrollsAtNarrowWidths(t *testing.T) {
	m := newCommandTestModel(commandEntries(4), 3)
	m.width = 30
	m.height = 12
	m.commandHelpOpen = true

	all := strings.Join(m.commandHelpLines(m.commandContentWidth()), "\n")
	for _, item := range commandHelpItems {
		if !strings.Contains(all, item.keys) {
			t.Fatalf("help omitted keys %q", item.keys)
		}
		for _, word := range strings.Fields(item.description) {
			if !strings.Contains(all, word) {
				t.Fatalf("help omitted %q from %q", word, item.description)
			}
		}
	}
	if !strings.Contains(all, "Enter\nOpen the focused") && !strings.Contains(all, "Open the focused") {
		t.Fatalf("Enter help does not describe details action: %q", all)
	}
	if !strings.Contains(all, "Return to the home") {
		t.Fatalf("Esc help does not describe return-to-home action: %q", all)
	}

	view := m.commandHelpView(m.commandContentWidth())
	for _, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > m.commandContentWidth() {
			t.Fatalf("narrow help line width %d exceeds %d: %q", got, m.commandContentWidth(), line)
		}
	}
	_, _ = m.updateCommandHelp(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.commandHelpOffset != 1 {
		t.Fatalf("j help offset = %d, want 1", m.commandHelpOffset)
	}
	_, _ = m.updateCommandHelp(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.commandHelpOffset != 0 {
		t.Fatalf("k help offset = %d, want 0", m.commandHelpOffset)
	}
	_, _ = m.updateCommandHelp(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.commandHelpOffset == 0 {
		t.Fatal("Page Down did not scroll command help")
	}
	_, _ = m.updateCommandHelp(tea.KeyMsg{Type: tea.KeyPgUp})
	_, _ = m.updateCommandHelp(tea.KeyMsg{Type: tea.KeyEnd})
	wantBottom := max(len(m.commandHelpLines(m.commandContentWidth()))-m.commandHelpPageHeight(), 0)
	if m.commandHelpOffset != wantBottom || m.commandHelpOffset == 0 {
		t.Fatalf("End help offset = %d, want bottom %d", m.commandHelpOffset, wantBottom)
	}
	bottom := m.commandHelpView(m.commandContentWidth())
	if !strings.Contains(bottom, "?") || !strings.Contains(bottom, "help") {
		t.Fatalf("bottom help page does not expose final ? binding: %q", bottom)
	}
	_, _ = m.updateCommandHelp(tea.KeyMsg{Type: tea.KeyHome})
	if m.commandHelpOffset != 0 {
		t.Fatalf("Home help offset = %d, want 0", m.commandHelpOffset)
	}
	_, _ = m.Update(tea.WindowSizeMsg{Width: 0, Height: 0})
	_ = m.commandsMainView(0)

	m.width = 40
	m.height = 10
	m.commandHelpOffset = 0
	veryShort := m.commandHelpView(m.commandContentWidth())
	if !strings.Contains(veryShort, "↑/↓ and j/k") {
		t.Fatalf("very short help omitted useful shortcut content: %q", veryShort)
	}
}

func TestDocumentedEnterAndEscMatchBrowseBehavior(t *testing.T) {
	m := newCommandTestModel(commandEntries(1), 3)
	_, _ = m.updateBrowseKeys(tea.KeyMsg{Type: tea.KeyEnter})
	if m.cmdPhase != commandsDetail {
		t.Fatalf("Enter phase = %d, want command details", m.cmdPhase)
	}
	m.cmdPhase = commandsBrowse
	_, _ = m.updateBrowseKeys(tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != ScreenHome {
		t.Fatalf("Esc screen = %d, want home", m.screen)
	}
}

func TestCommandHelpRestoresActiveFilterFocus(t *testing.T) {
	m := newCommandTestModel(commandEntries(2), 3)
	m.cmdFocus = commandsFocusSearch
	m.searchTI.Focus()
	m.searchTI.SetValue("kept query")
	_, _ = m.updateCommands(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !m.commandHelpOpen || m.cmdFocus != commandsFocusSearch {
		t.Fatalf("help did not preserve search focus: open=%v focus=%d", m.commandHelpOpen, m.cmdFocus)
	}
	_, _ = m.updateCommands(tea.KeyMsg{Type: tea.KeyEsc})
	if m.commandHelpOpen || m.cmdFocus != commandsFocusSearch || m.searchTI.Value() != "kept query" {
		t.Fatalf("closing help changed search state: open=%v focus=%d query=%q", m.commandHelpOpen, m.cmdFocus, m.searchTI.Value())
	}
}

func assertCommandHelpPreservedState(t *testing.T, m *Model, id int64, cursor, offset int, search, tag string, selected map[int64]struct{}) {
	t.Helper()
	if m.selectedID != id || m.browseCursor != cursor || m.browseOffset != offset || m.searchTI.Value() != search || m.tagTI.Value() != tag {
		t.Fatalf("help changed browse state: id=%d cursor=%d offset=%d search=%q tag=%q", m.selectedID, m.browseCursor, m.browseOffset, m.searchTI.Value(), m.tagTI.Value())
	}
	if m.commandStatus.text != "Copied" {
		t.Fatalf("help changed temporary status: %q", m.commandStatus.text)
	}
	if len(m.multiSelected) != len(selected) {
		t.Fatalf("help changed selected IDs: %v", m.multiSelected)
	}
	for selectedID := range selected {
		if !m.isMultiSelected(selectedID) {
			t.Fatalf("help lost selected ID %d: %v", selectedID, m.multiSelected)
		}
	}
}

func TestHelpSelectionMarkerWidthAccountingRegression(t *testing.T) {
	m := newCommandTestModel([]model.Entry{{ID: 1, Command: strings.Repeat("x", 40), Description: strings.Repeat("y", 40), Tags: []string{"Category"}}}, 6)
	m.multiSelected[1] = struct{}{}
	for _, width := range []int{0, 1, 5, 21, 40, 76} {
		marker, command, gap, description := browseColumnWidths(width)
		if marker+command+gap+description > width {
			t.Fatalf("width %d columns overflow: %d/%d/%d/%d", width, marker, command, gap, description)
		}
		for _, line := range strings.Split(m.renderCommandEntry(width, 0), "\n") {
			if lipgloss.Width(line) > width {
				t.Fatalf("selected row overflow at width %d: %q", width, line)
			}
		}
	}
}
