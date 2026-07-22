package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Architeg/gloss/internal/model"
)

func TestBrowseSelectionPreservesPersistentIdentityAcrossRebuilds(t *testing.T) {
	tests := []struct {
		name    string
		updated model.Entry
	}{
		{name: "command reorder", updated: model.Entry{ID: 2, Command: "zulu", Tags: []string{"Alpha"}}},
		{name: "primary tag reorder", updated: model.Entry{ID: 2, Command: "middle", Tags: []string{"Zulu"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newCommandTestModel([]model.Entry{
				{ID: 1, Command: "alpha", Tags: []string{"Alpha"}},
				{ID: 2, Command: "middle", Tags: []string{"Alpha"}},
				{ID: 3, Command: "omega", Tags: []string{"Beta"}},
			}, 8)
			m.selectBrowseIndex(m.rowIndexByID(2))
			m.form.prepareEdit(m.cmdRows[m.browseCursor].Entry)
			m.form.cmdTI.SetValue(tt.updated.Command)
			m.form.tagsTI.SetValue(strings.Join(tt.updated.Tags, ","))
			m.cmdPhase = commandsEdit
			m.editFromBrowse = true
			if _, _ = m.afterSave(); m.selectedID != 2 {
				t.Fatalf("afterSave selected ID = %d, want 2", m.selectedID)
			}
			m.allEntries[1] = tt.updated
			m.rebuildBrowse()
			assertBrowseSelection(t, m, 2)
		})
	}
}

func TestBrowseSelectionFilterFallbackAndRestore(t *testing.T) {
	m := newCommandTestModel([]model.Entry{
		{ID: 1, Command: "alpha", Description: "first", Tags: []string{"One", "shared"}},
		{ID: 2, Command: "bravo", Description: "second", Tags: []string{"Two", "shared"}},
		{ID: 3, Command: "charlie", Description: "third", Tags: []string{"Three"}},
	}, 8)
	m.selectBrowseIndex(m.rowIndexByID(2))

	m.searchTI.SetValue("brav")
	m.rebuildBrowse()
	assertBrowseSelection(t, m, 2)

	m.searchTI.SetValue("alpha")
	m.rebuildBrowse()
	assertBrowseSelection(t, m, 1)
	if m.restoreID != 2 {
		t.Fatalf("hidden selection restore ID = %d, want 2", m.restoreID)
	}

	m.searchTI.SetValue("")
	m.rebuildBrowse()
	assertBrowseSelection(t, m, 2)
	if m.restoreID != 0 {
		t.Fatalf("restore ID after clearing search = %d, want 0", m.restoreID)
	}

	m.tagTI.SetValue("shared")
	m.rebuildBrowse()
	assertBrowseSelection(t, m, 2)
}

func TestBrowseDeletionFallbacks(t *testing.T) {
	entries := []model.Entry{
		{ID: 1, Command: "alpha", Tags: []string{"Group"}},
		{ID: 2, Command: "bravo", Tags: []string{"Group"}},
		{ID: 3, Command: "charlie", Tags: []string{"Group"}},
	}

	t.Run("next row", func(t *testing.T) {
		m := newCommandTestModel(entries, 5)
		m.selectBrowseIndex(1)
		m.allEntries = []model.Entry{entries[0], entries[2]}
		m.rebuildBrowse()
		assertBrowseSelection(t, m, 3)
		if m.browseCursor != 1 {
			t.Fatalf("cursor = %d, want deleted row 1", m.browseCursor)
		}
	})

	t.Run("preceding final row", func(t *testing.T) {
		m := newCommandTestModel(entries, 5)
		m.selectBrowseIndex(2)
		m.allEntries = entries[:2]
		m.rebuildBrowse()
		assertBrowseSelection(t, m, 2)
		if m.browseCursor != 1 {
			t.Fatalf("cursor = %d, want final row 1", m.browseCursor)
		}
	})

	t.Run("only row", func(t *testing.T) {
		m := newCommandTestModel(entries[:1], 5)
		m.allEntries = nil
		m.rebuildBrowse()
		if len(m.cmdRows) != 0 || m.browseCursor != 0 || m.browseOffset != 0 || m.selectedID != 0 {
			t.Fatalf("empty selection state: rows=%d cursor=%d offset=%d id=%d", len(m.cmdRows), m.browseCursor, m.browseOffset, m.selectedID)
		}
	})
}

func TestBrowseReloadAndDeleteMessagesReconcileIdentity(t *testing.T) {
	entries := []model.Entry{
		{ID: 1, Command: "alpha", Tags: []string{"Group"}},
		{ID: 2, Command: "bravo", Tags: []string{"Group"}},
		{ID: 3, Command: "charlie", Tags: []string{"Group"}},
	}
	m := newCommandTestModel(entries, 4)
	m.selectBrowseIndex(1)

	_, _ = m.Update(entriesMsg{entries: []model.Entry{entries[2], entries[1], entries[0]}})
	assertBrowseSelection(t, m, 2)

	m.detailEntry = m.cmdRows[m.browseCursor].Entry
	m.deleteFromBrowse = true
	m.cmdPhase = commandsDeleteConfirm
	_, _ = m.Update(deleteMsg{})
	if m.cmdPhase != commandsBrowse || m.selectedID != 2 || m.browseCursor != 1 {
		t.Fatalf("delete completion discarded selection before reload: phase=%d id=%d cursor=%d", m.cmdPhase, m.selectedID, m.browseCursor)
	}
	_, _ = m.Update(entriesMsg{entries: []model.Entry{entries[0], entries[2]}})
	assertBrowseSelection(t, m, 3)
}

func TestBrowseZeroIDEntriesUsePositionalFallback(t *testing.T) {
	m := newCommandTestModel([]model.Entry{
		{Command: "alpha", Tags: []string{"Group"}},
		{Command: "bravo", Tags: []string{"Group"}},
		{Command: "charlie", Tags: []string{"Group"}},
	}, 5)
	m.selectBrowseIndex(1)
	m.allEntries = []model.Entry{
		{Command: "able", Tags: []string{"Group"}},
		{Command: "baker", Tags: []string{"Group"}},
		{Command: "charlie", Tags: []string{"Group"}},
	}
	m.rebuildBrowse()
	if m.browseCursor != 1 || m.cmdRows[1].Entry.Command != "baker" || m.selectedID != 0 {
		t.Fatalf("zero-ID fallback: cursor=%d command=%q ID=%d", m.browseCursor, m.cmdRows[m.browseCursor].Entry.Command, m.selectedID)
	}
}

func TestBrowseBasicNavigationAndPageMovement(t *testing.T) {
	m := newCommandTestModel(commandEntries(10), 3)
	if page := m.browsePageSize(); page < 1 {
		t.Fatalf("page size = %d, want progress", page)
	}

	m.moveBrowseBy(-1)
	assertBrowseSelection(t, m, 1)
	m.moveBrowseBy(1)
	assertBrowseSelection(t, m, 2)
	m.moveBrowseBy(-1)
	m.moveBrowseBy(-1)
	assertBrowseSelection(t, m, 1)

	m.selectBrowseIndex(len(m.cmdRows) - 1)
	m.moveBrowseBy(1)
	assertBrowseSelection(t, m, 10)
	m.selectBrowseIndex(0)
	page := m.browsePageSize()
	m.moveBrowsePage(1)
	if m.browseCursor != min(page, len(m.cmdRows)-1) {
		t.Fatalf("Page Down cursor = %d, want %d", m.browseCursor, min(page, len(m.cmdRows)-1))
	}
	assertBrowseSelectionVisible(t, m)
	m.moveBrowsePage(-1)
	if m.browseCursor != 0 {
		t.Fatalf("Page Up cursor = %d, want 0", m.browseCursor)
	}
	m.selectBrowseIndex(len(m.cmdRows) - 1)
	m.moveBrowsePage(1)
	if m.browseCursor != len(m.cmdRows)-1 {
		t.Fatalf("Page Down did not clamp: %d", m.browseCursor)
	}

	m.selectBrowseIndex(4)
	_, _ = m.updateBrowseKeys(tea.KeyMsg{Type: tea.KeyHome})
	assertBrowseSelection(t, m, 1)
	_, _ = m.updateBrowseKeys(tea.KeyMsg{Type: tea.KeyEnd})
	assertBrowseSelection(t, m, 10)
}

func TestBrowseNavigationEmptyAndZeroHeightIsSafe(t *testing.T) {
	empty := newCommandTestModel(nil, 0)
	for _, key := range []tea.KeyMsg{{Type: tea.KeyUp}, {Type: tea.KeyDown}, {Type: tea.KeyHome}, {Type: tea.KeyEnd}, {Type: tea.KeyPgUp}, {Type: tea.KeyPgDown}, {Type: tea.KeyRunes, Runes: []rune{'['}}, {Type: tea.KeyRunes, Runes: []rune{']'}}} {
		_, _ = empty.updateBrowseKeys(key)
	}
	if empty.browseCursor != 0 || empty.browseOffset != 0 {
		t.Fatalf("empty navigation changed state: cursor=%d offset=%d", empty.browseCursor, empty.browseOffset)
	}

	zeroHeight := newCommandTestModel(commandEntries(3), 0)
	before := zeroHeight.browseCursor
	zeroHeight.moveBrowsePage(1)
	if zeroHeight.browseCursor != before || zeroHeight.browseOffset != 0 {
		t.Fatalf("zero-height page navigation changed state: cursor=%d offset=%d", zeroHeight.browseCursor, zeroHeight.browseOffset)
	}
}

func TestBrowseGroupJumping(t *testing.T) {
	m := newCommandTestModel([]model.Entry{
		{ID: 1, Command: "alpha", Tags: []string{"Alpha"}},
		{ID: 2, Command: "bravo", Tags: []string{"ALPHA"}},
		{ID: 3, Command: "charlie", Tags: []string{"Beta"}},
		{ID: 4, Command: "literal", Tags: []string{"Untagged"}},
		{ID: 5, Command: "none"},
	}, 3)

	m.selectBrowseIndex(0)
	m.jumpBrowseGroup(-1)
	assertBrowseSelection(t, m, 1)
	m.jumpBrowseGroup(1)
	assertBrowseSelection(t, m, 3)
	assertBrowseSelectionVisible(t, m)
	m.jumpBrowseGroup(1)
	assertBrowseSelection(t, m, 4)
	m.jumpBrowseGroup(1)
	assertBrowseSelection(t, m, 5)
	m.jumpBrowseGroup(1)
	assertBrowseSelection(t, m, 5)

	m.selectBrowseIndex(1)
	m.jumpBrowseGroup(-1)
	assertBrowseSelection(t, m, 1)
	m.jumpBrowseGroup(1)
	m.jumpBrowseGroup(-1)
	assertBrowseSelection(t, m, 1)
}

func TestBrowsePageSizeAccountsForGroupHeadingRows(t *testing.T) {
	m := newCommandTestModel([]model.Entry{
		{ID: 1, Command: "alpha", Tags: []string{"Alpha"}},
		{ID: 2, Command: "beta", Tags: []string{"Beta"}},
		{ID: 3, Command: "charlie", Tags: []string{"Charlie"}},
	}, 20)
	width := m.commandContentWidth()
	firstBlock := m.categoryHeaderBlock(width, m.cmdRows[0].Group) + "\n\n" + m.renderCommandEntry(width, 0)
	setCommandListHeight(m, lipgloss.Height(firstBlock))
	m.ensureBrowseVisible(true)
	if page := m.browsePageSize(); page != 1 {
		t.Fatalf("page size with one heading/entry block = %d, want 1", page)
	}
	m.moveBrowsePage(1)
	assertBrowseSelection(t, m, 2)
	assertBrowseSelectionVisible(t, m)
}

func TestViewportShowsCurrentGroupWhenStartingMidGroup(t *testing.T) {
	m := newCommandTestModel([]model.Entry{
		{ID: 1, Command: "alpha", Tags: []string{"Alpha"}},
		{ID: 2, Command: "bravo", Tags: []string{"ALPHA"}},
		{ID: 3, Command: "charlie", Tags: []string{"Alpha"}},
	}, 8)
	m.selectBrowseIndex(1)
	m.browseOffset = 1
	rendered, _ := m.renderCommandViewport(m.commandContentWidth(), m.commandListHeight(m.commandContentWidth()), 1)
	if !strings.Contains(rendered, "Category:") || !model.ContainsFold(rendered, "Alpha") {
		t.Fatalf("mid-group viewport lacks current group label: %q", rendered)
	}
}

func TestBrowseGroupJumpUsesFilteredRowsOnly(t *testing.T) {
	m := newCommandTestModel([]model.Entry{
		{ID: 1, Command: "alpha", Tags: []string{"Alpha", "shared"}},
		{ID: 2, Command: "beta", Tags: []string{"Beta"}},
		{ID: 3, Command: "charlie", Tags: []string{"Charlie", "shared"}},
	}, 4)
	m.tagTI.SetValue("shared")
	m.rebuildBrowse()
	if len(m.cmdRows) != 2 {
		t.Fatalf("filtered rows = %d, want 2", len(m.cmdRows))
	}
	m.selectBrowseIndex(0)
	m.jumpBrowseGroup(1)
	assertBrowseSelection(t, m, 3)
	m.jumpBrowseGroup(1)
	assertBrowseSelection(t, m, 3)
}

func TestBrowseScrollingIsMinimalAndBounded(t *testing.T) {
	m := newCommandTestModel(commandEntries(10), 3)
	_, initialEnd := m.renderCommandViewport(m.commandContentWidth(), m.commandListHeight(m.commandContentWidth()), 0)
	if initialEnd < 2 {
		t.Fatalf("test viewport contains %d entries, want at least 2", initialEnd)
	}
	for i := 1; i < initialEnd; i++ {
		m.moveBrowseBy(1)
		if m.browseOffset != 0 {
			t.Fatalf("movement within viewport changed offset to %d", m.browseOffset)
		}
	}
	m.moveBrowseBy(1)
	if m.browseOffset != 1 {
		t.Fatalf("first movement below viewport offset = %d, want 1", m.browseOffset)
	}
	assertBrowseSelectionVisible(t, m)
	rendered, _ := m.renderCommandViewport(m.commandContentWidth(), m.commandListHeight(m.commandContentWidth()), m.browseOffset)
	if !strings.Contains(rendered, m.cmdRows[m.browseCursor].Entry.Command) || !strings.Contains(rendered, "›") {
		t.Fatalf("scrolled viewport selected the wrong logical row: %q", rendered)
	}
	m.selectBrowseIndex(m.browseOffset)
	oldOffset := m.browseOffset
	m.moveBrowseBy(-1)
	if oldOffset > 0 && m.browseOffset != oldOffset-1 {
		t.Fatalf("movement above viewport offset = %d, want %d", m.browseOffset, oldOffset-1)
	}
	m.selectBrowseIndex(0)
	if m.browseOffset != 0 {
		t.Fatalf("Home-equivalent selection offset = %d, want 0", m.browseOffset)
	}
	m.selectBrowseIndex(len(m.cmdRows) - 1)
	_, end := m.renderCommandViewport(m.commandContentWidth(), m.commandListHeight(m.commandContentWidth()), m.browseOffset)
	if end != len(m.cmdRows) || m.browseCursor < m.browseOffset {
		t.Fatalf("End not visible: cursor=%d offset=%d end=%d", m.browseCursor, m.browseOffset, end)
	}

	m.selectBrowseIndex(5)
	selected := m.selectedID
	shortOffset := m.browseOffset
	setCommandListHeight(m, 8)
	m.ensureBrowseVisible(true)
	assertBrowseSelection(t, m, selected)
	if m.browseOffset >= shortOffset {
		t.Fatalf("taller resize did not reduce offset: before=%d after=%d", shortOffset, m.browseOffset)
	}
	setCommandListHeight(m, 2)
	m.ensureBrowseVisible(true)
	assertBrowseSelection(t, m, selected)
	_, end = m.renderCommandViewport(m.commandContentWidth(), m.commandListHeight(m.commandContentWidth()), m.browseOffset)
	if m.browseCursor < m.browseOffset || m.browseCursor >= end {
		t.Fatalf("short resize hid selection: cursor=%d offset=%d end=%d", m.browseCursor, m.browseOffset, end)
	}
}

func TestBrowseFilteringClampsOffsetAndRendering(t *testing.T) {
	m := newCommandTestModel(commandEntries(12), 3)
	m.selectBrowseIndex(10)
	if m.browseOffset == 0 {
		t.Fatal("expected scrolled test state")
	}
	m.searchTI.SetValue("command-01")
	m.rebuildBrowse()
	if len(m.cmdRows) != 1 || m.browseOffset != 0 || m.browseCursor != 0 {
		t.Fatalf("filtered state: rows=%d cursor=%d offset=%d", len(m.cmdRows), m.browseCursor, m.browseOffset)
	}

	height := m.commandListHeight(m.commandContentWidth())
	rendered, end := m.renderCommandViewport(m.commandContentWidth(), height, m.browseOffset)
	if lipgloss.Height(rendered) > height || end != 1 {
		t.Fatalf("rendered height/end = %d/%d, limit  %d", lipgloss.Height(rendered), end, height)
	}
	if !strings.Contains(rendered, "command-01") || !strings.Contains(rendered, "›") {
		t.Fatalf("rendered selected row does not match selected ID: %q", rendered)
	}

	m.searchTI.SetValue("")
	m.rebuildBrowse()
	assertBrowseSelection(t, m, 11)
}

func TestCommandsBrowseViewDoesNotExceedMeasuredContentArea(t *testing.T) {
	entries := commandEntries(20)
	for i := range entries {
		entries[i].Tags = []string{"Group-" + twoDigits(i/3+1)}
		entries[i].Description = strings.Repeat("wrapped description ", 5)
	}
	m := newCommandTestModel(entries, 6)
	m.selectBrowseIndex(12)
	view := m.commandsBrowseView(m.commandContentWidth())
	if got, limit := lipgloss.Height(view), m.mainContentHeight(); got > limit {
		t.Fatalf("commands browse height = %d, measured content limit %d", got, limit)
	}
	assertBrowseSelectionVisible(t, m)
}

func TestBrowseWindowResizePreservesSelectedID(t *testing.T) {
	m := newCommandTestModel(commandEntries(10), 4)
	m.selectBrowseIndex(7)
	wantID := m.selectedID
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: m.height + 8})
	assertBrowseSelection(t, m, wantID)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	assertBrowseSelection(t, m, wantID)
}

func newCommandTestModel(entries []model.Entry, listHeight int) *Model {
	m := New(Options{}).(*Model)
	m.width = 80
	m.screen = ScreenCommands
	m.cmdPhase = commandsBrowse
	m.cmdFocus = commandsFocusList
	m.allEntries = append([]model.Entry(nil), entries...)
	setCommandListHeight(m, listHeight)
	m.rebuildBrowse()
	return m
}

func setCommandListHeight(m *Model, listHeight int) {
	m.height = 100
	_, _, width, _, footer := m.layoutMetrics()
	fixed := lipgloss.Height(m.commandsBrowseFixedBlock(width))
	m.height = lipgloss.Height(footer) + m.styles.Padding.GetVerticalFrameSize() + fixed + max(listHeight, 0)
}

func commandEntries(count int) []model.Entry {
	entries := make([]model.Entry, count)
	for i := range entries {
		entries[i] = model.Entry{
			ID: int64(i + 1), Command: "command-" + twoDigits(i+1), Tags: []string{"Group"},
		}
	}
	return entries
}

func twoDigits(value int) string {
	if value < 10 {
		return "0" + string(rune('0'+value))
	}
	return string(rune('0'+value/10)) + string(rune('0'+value%10))
}

func assertBrowseSelection(t *testing.T, m *Model, wantID int64) {
	t.Helper()
	if !m.hasBrowseSelection() {
		t.Fatalf("no browse selection: cursor=%d rows=%d", m.browseCursor, len(m.cmdRows))
	}
	if m.selectedID != wantID || m.cmdRows[m.browseCursor].Entry.ID != wantID {
		t.Fatalf("selection ID/row ID = %d/%d, want %d (cursor %d)", m.selectedID, m.cmdRows[m.browseCursor].Entry.ID, wantID, m.browseCursor)
	}
}

func assertBrowseSelectionVisible(t *testing.T, m *Model) {
	t.Helper()
	height := m.commandListHeight(m.commandContentWidth())
	if height <= 0 {
		return
	}
	_, end := m.renderCommandViewport(m.commandContentWidth(), height, m.browseOffset)
	if m.browseCursor < m.browseOffset || m.browseCursor >= end {
		t.Fatalf("selection is outside viewport: cursor=%d offset=%d end=%d", m.browseCursor, m.browseOffset, end)
	}
}
