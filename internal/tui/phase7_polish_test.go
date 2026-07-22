package tui

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

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

func TestRenderedCommandRowsUseLiteralResponsiveColumnGaps(t *testing.T) {
	tests := []struct {
		width int
		gap   int
	}{
		{width: 76, gap: 4},
		{width: 64, gap: 4},
		{width: 63, gap: 3},
		{width: 44, gap: 3},
		{width: 43, gap: 2},
		{width: 28, gap: 2},
		{width: 27, gap: 1},
	}
	for _, tt := range tests {
		m := newCommandTestModel([]model.Entry{{
			ID:          1,
			Command:     strings.Repeat("c", comfortableCommandWidth),
			Description: "DESC",
			Tags:        []string{"Category"},
		}}, 2)
		markerW, commandW, gap, _ := m.contentAwareBrowseColumnWidths(tt.width)
		m.cmdRows[0].Entry.Command = strings.Repeat("c", commandW)
		m.multiSelected[1] = struct{}{}
		rendered := m.renderCommandEntry(tt.width, 0)
		plain := stripANSI(rendered)
		assertRenderedGap(t, plain, tt.width, markerW, commandW, gap, "DESC")
		if gap != tt.gap {
			t.Fatalf("width %d rendered gap %d, want %d", tt.width, gap, tt.gap)
		}
		if !strings.Contains(firstLine(plain), "›") || !strings.Contains(firstLine(plain), "✓") {
			t.Fatalf("width %d omitted focus/selection markers: %q", tt.width, firstLine(plain))
		}
	}

	m := newCommandTestModel([]model.Entry{{
		ID:          1,
		Command:     strings.Repeat("x", comfortableCommandWidth-1),
		Description: "DESC",
		Tags:        []string{"Category"},
	}}, 2)
	markerW, commandW, gap, _ := m.contentAwareBrowseColumnWidths(76)
	m.multiSelected[1] = struct{}{}
	assertRenderedGap(t, stripANSI(m.renderCommandEntry(76, 0)), 76, markerW, commandW, gap, "DESC")
}

func TestWideCommandColumnExpandsForVisibleContent(t *testing.T) {
	const width = 76
	const command = "dssh add 'name' 'root@ip' 'pass'"
	m := newCommandTestModel([]model.Entry{{
		ID:          1,
		Command:     command,
		Description: "DESC",
		Tags:        []string{"Category"},
	}}, 2)
	m.multiSelected[1] = struct{}{}
	markerW, commandW, gap, descW := m.contentAwareBrowseColumnWidths(width)
	if measured := runewidth.StringWidth(command); measured != 32 {
		t.Fatalf("representative command width = %d, want 32", measured)
	}
	if commandW != 32 || gap != 4 || descW != 36 {
		t.Fatalf("wide content-aware columns = %d/%d/%d/%d, want 4/32/4/36", markerW, commandW, gap, descW)
	}
	plain := stripANSI(m.renderCommandEntry(width, 0))
	if strings.Count(plain, "\n") != 0 || !strings.Contains(plain, command) {
		t.Fatalf("ordinary wide command wrapped unexpectedly: %q", plain)
	}
	assertRenderedGap(t, plain, width, markerW, commandW, gap, "DESC")
}

func TestWrappedCommandRowsStayBoundedAndKeepAllocatedGap(t *testing.T) {
	const width = 76
	m := newCommandTestModel([]model.Entry{{
		ID:          1,
		Command:     strings.Repeat("long command ", 7),
		Description: strings.Repeat("long description ", 9),
		Tags:        []string{"Category"},
	}}, 2)
	markerW, commandW, gap, descW := m.contentAwareBrowseColumnWidths(width)
	if commandW != 32 || descW != 36 {
		t.Fatalf("capped wide columns = %d/%d, want 32/36", commandW, descW)
	}
	m.multiSelected[1] = struct{}{}
	plain := stripANSI(m.renderCommandEntry(width, 0))
	lines := strings.Split(plain, "\n")
	if len(lines) < 2 {
		t.Fatalf("long command row did not wrap: %q", plain)
	}
	for _, line := range lines {
		if got := runewidth.StringWidth(line); got > width {
			t.Fatalf("wrapped command line width %d exceeds %d: %q", got, width, line)
		}
		if got := visualCellSlice(line, markerW+commandW, markerW+commandW+gap); got != strings.Repeat(" ", gap) {
			t.Fatalf("wrapped command gap = %q, want %d literal spaces in %q", got, gap, line)
		}
	}
}

func TestRenderedAliasRowsUseLiteralResponsiveColumnGaps(t *testing.T) {
	for _, width := range []int{76, 64, 63, 44, 43, 28, 27} {
		markerW, commandW, gap, _ := responsiveColumnWidths(width, 2, 22, 8, 8)
		m := New(Options{}).(*Model)
		m.aliasPhase = aliasPhaseView
		m.allEntries = []model.Entry{{
			ID:           1,
			Command:      strings.Repeat("a", commandW),
			Target:       "TARGET",
			Type:         model.EntryTypeAlias,
			ManagedAlias: true,
		}}
		plain := stripANSI(m.aliasListView(width))
		line := lineContaining(plain, "TARGET")
		if line == "" {
			t.Fatalf("width %d alias view omitted target: %q", width, plain)
		}
		assertRenderedGap(t, line, width, markerW, commandW, gap, "TARGET")
	}

	m := New(Options{}).(*Model)
	m.aliasPhase = aliasPhaseView
	m.allEntries = []model.Entry{{
		ID:           1,
		Command:      strings.Repeat("a", 32),
		Target:       strings.Repeat("target ", 12),
		Type:         model.EntryTypeAlias,
		ManagedAlias: true,
	}}
	for _, line := range strings.Split(stripANSI(m.aliasListView(76)), "\n") {
		if got := runewidth.StringWidth(line); got > 76 {
			t.Fatalf("content-aware alias line width %d exceeds 76: %q", got, line)
		}
	}
}

func TestPriorityCommandFooterLabelsOrderAndDiscoverability(t *testing.T) {
	m := newCommandTestModel(commandEntries(2), 3)
	m.width = 600
	wide := stripANSI(m.footerContent())
	labels := []string{
		"A Add", "E Edit", "D Delete", "Esc Back", "Q Quit",
		"↑↓ Navigate", "Space Select/deselect", "Enter Details", "/ Search",
		"F Filter by tag", "? Help", "Ctrl+A Select visible",
		"T Edit selected tags", "C Copy command", "[ ] Change category",
		"PgUp/PgDn Move page", "Home/End First/last",
	}
	previous := -1
	for _, label := range labels {
		if !strings.Contains(wide, label) {
			t.Fatalf("wide footer missing %q: %q", label, wide)
		}
		position := strings.Index(wide, label)
		if position <= previous {
			t.Fatalf("footer priority order is wrong at %q: %q", label, wide)
		}
		previous = position
	}
	assertNoUnclearFooterTerms(t, wide)

	priorityKeys := []string{"A", "E", "D", "Esc", "Q", "↑↓", "Space", "Enter", "/", "F", "Ctrl+A", "T", "C", "[ ]", "PgUp/PgDn", "Home/End"}
	completeHints := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		completeHints[label] = struct{}{}
	}
	for width := 7; width <= 220; width++ {
		m.width = width
		footer := stripANSI(m.footerContent())
		if !strings.Contains(footer, "?") {
			t.Fatalf("footer width %d lost reserved help hint: %q", width, footer)
		}
		if got, limit := runewidth.StringWidth(footer), m.footerAvailableWidth(); got > limit {
			t.Fatalf("footer width %d rendered %d, limit %d: %q", width, got, limit, footer)
		}
		if footer != "?" {
			for _, hint := range strings.Split(footer, " │ ") {
				if _, ok := completeHints[hint]; !ok {
					t.Fatalf("footer width %d contains non-atomic hint %q: %q", width, hint, footer)
				}
			}
		}
		missingHigherPriority := false
		for _, key := range priorityKeys {
			present := footerHasKey(footer, key)
			if missingHigherPriority && present {
				t.Fatalf("footer width %d let %q displace a higher-priority action: %q", width, key, footer)
			}
			if !present {
				missingHigherPriority = true
			}
		}
		assertNoUnclearFooterTerms(t, footer)
	}

	m.width = 40
	narrow := stripANSI(m.footerContent())
	if want := "A Add │ E Edit │ D Delete │ ? Help"; narrow != want {
		t.Fatalf("narrow footer = %q, want %q", narrow, want)
	}
	m.width = 60
	if got, want := stripANSI(m.footerContent()), "A Add │ E Edit │ D Delete │ Esc Back │ Q Quit │ ? Help"; got != want {
		t.Fatalf("medium footer = %q, want %q", got, want)
	}
	m.width = 80
	if got, want := stripANSI(m.footerContent()), "A Add │ E Edit │ D Delete │ Esc Back │ Q Quit │ ↑↓ Navigate │ ? Help"; got != want {
		t.Fatalf("screenshot-width footer = %q, want %q", got, want)
	}
	m.width = 7
	if got := stripANSI(m.footerContent()); got != "?" {
		t.Fatalf("emergency footer = %q, want ?", got)
	}
}

func TestCommandHelpUsesBeginnerReadableBulkDescriptions(t *testing.T) {
	all := strings.Join((&Model{styles: newStyles()}).commandHelpLines(120), "\n")
	for _, want := range []string{
		"Select or deselect all commands visible under the current search and tag filters.",
		"Add or remove tags from the selected commands.",
	} {
		if !strings.Contains(stripANSI(all), want) {
			t.Fatalf("shortcut help missing %q: %q", want, stripANSI(all))
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
		marker, command, gap, description := m.contentAwareBrowseColumnWidths(width)
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

var ansiSequence = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiSequence.ReplaceAllString(s, "")
}

func firstLine(s string) string {
	if before, _, ok := strings.Cut(s, "\n"); ok {
		return before
	}
	return s
}

func lineContaining(s, needle string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func visualCellSlice(s string, start, end int) string {
	var b strings.Builder
	position := 0
	for _, r := range s {
		width := runewidth.RuneWidth(r)
		if position >= start && position+width <= end {
			b.WriteRune(r)
		}
		position += width
		if position >= end {
			break
		}
	}
	return b.String()
}

func assertRenderedGap(t *testing.T, rendered string, available, markerW, leadingW, gap int, tail string) {
	t.Helper()
	line := lineContaining(rendered, tail)
	if line == "" {
		t.Fatalf("rendered row omitted %q: %q", tail, rendered)
	}
	byteIndex := strings.Index(line, tail)
	tailStart := runewidth.StringWidth(line[:byteIndex])
	wantStart := markerW + leadingW + gap
	if tailStart != wantStart {
		t.Fatalf("%q starts at cell %d, want %d (marker=%d leading=%d gap=%d): %q", tail, tailStart, wantStart, markerW, leadingW, gap, line)
	}
	literalGap := visualCellSlice(line, markerW+leadingW, tailStart)
	if literalGap != strings.Repeat(" ", gap) {
		t.Fatalf("allocated column gap = %q, want %d literal spaces: %q", literalGap, gap, line)
	}
	for _, renderedLine := range strings.Split(rendered, "\n") {
		if got := runewidth.StringWidth(renderedLine); got > available {
			t.Fatalf("rendered line width %d exceeds %d: %q", got, available, renderedLine)
		}
	}
}

func footerHasKey(footer, key string) bool {
	for _, part := range strings.Split(footer, " │ ") {
		if part == key || strings.HasPrefix(part, key+" ") {
			return true
		}
	}
	return false
}

func assertNoUnclearFooterTerms(t *testing.T, footer string) {
	t.Helper()
	for _, forbidden := range []string{"^A", "All visible", "Bulk tags", "Bounds", "Groups"} {
		if strings.Contains(footer, forbidden) {
			t.Fatalf("footer contains unclear term %q: %q", forbidden, footer)
		}
	}
}
