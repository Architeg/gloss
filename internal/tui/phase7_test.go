package tui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Architeg/gloss/internal/model"
	"github.com/Architeg/gloss/internal/storage"
)

func TestBrowseColumnWidthsAreResponsiveAndBounded(t *testing.T) {
	tests := []struct {
		width                        int
		wantMarker, wantCommand, gap int
	}{
		{width: 76, wantMarker: 4, wantCommand: 18, gap: 3},
		{width: 40, wantMarker: 4, wantCommand: 12, gap: 2},
		{width: 20, wantMarker: 4, wantCommand: 8, gap: 0},
		{width: 5, wantMarker: 4, wantCommand: 1, gap: 0},
		{width: 0, wantMarker: 0, wantCommand: 0, gap: 0},
	}
	for _, tt := range tests {
		marker, command, gap, description := browseColumnWidths(tt.width)
		if marker != tt.wantMarker || command != tt.wantCommand || gap != tt.gap {
			t.Fatalf("width %d columns = %d/%d/%d/%d, want marker/command/gap %d/%d/%d", tt.width, marker, command, gap, description, tt.wantMarker, tt.wantCommand, tt.gap)
		}
		if marker < 0 || command < 0 || gap < 0 || description < 0 {
			t.Fatalf("width %d produced negative columns: %d/%d/%d/%d", tt.width, marker, command, gap, description)
		}
		if marker+command+gap+description > tt.width {
			t.Fatalf("width %d overflowed: %d/%d/%d/%d", tt.width, marker, command, gap, description)
		}
	}
}

func TestResponsiveCommandAndAliasRowsStayWithinWidth(t *testing.T) {
	for _, width := range []int{0, 1, 5, 12, 20, 40, 76} {
		m := newCommandTestModel([]model.Entry{{ID: 1, Command: "very long command", Description: strings.Repeat("long description ", 5), Tags: []string{"Group"}}}, 20)
		m.width = width + m.styles.Padding.GetHorizontalFrameSize()
		m.multiSelected[1] = struct{}{}
		rendered := m.renderCommandEntry(width, 0)
		for _, line := range strings.Split(rendered, "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("command row width %d at available %d: %q", got, width, line)
			}
		}
		if width > 0 {
			for _, line := range strings.Split(m.commandsBrowseView(width), "\n") {
				if got := lipgloss.Width(line); got > width {
					t.Fatalf("command view line width %d at available %d: %q", got, width, line)
				}
			}
		}
	}

	m := New(Options{}).(*Model)
	m.aliasPhase = aliasPhaseView
	m.allEntries = []model.Entry{{ID: 1, Command: "long_alias_name", Target: "echo a very long target", Type: model.EntryTypeAlias, ManagedAlias: true}}
	for _, width := range []int{1, 8, 20, 40} {
		for _, line := range strings.Split(m.aliasListView(width), "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("alias view line width %d at available %d: %q", got, width, line)
			}
		}
	}
}

func TestCommandMultiselectionToggleAndVisibleAll(t *testing.T) {
	m := newCommandTestModel([]model.Entry{
		{ID: 1, Command: "alpha", Tags: []string{"One"}},
		{ID: 2, Command: "bravo", Tags: []string{"Two"}},
		{ID: 3, Command: "charlie", Tags: []string{"Three"}},
	}, 5)
	m.selectBrowseIndex(m.rowIndexByID(2))
	beforeCursor, beforeID := m.browseCursor, m.selectedID
	_, _ = m.updateBrowseKeys(tea.KeyMsg{Type: tea.KeySpace})
	if !m.isMultiSelected(2) || m.browseCursor != beforeCursor || m.selectedID != beforeID {
		t.Fatalf("Space changed focus or failed selection: cursor=%d id=%d selected=%v", m.browseCursor, m.selectedID, m.multiSelected)
	}
	_, _ = m.updateBrowseKeys(tea.KeyMsg{Type: tea.KeySpace})
	if m.isMultiSelected(2) {
		t.Fatal("second Space did not deselect focused entry")
	}

	m.multiSelected[3] = struct{}{}
	m.tagTI.SetValue("One")
	m.rebuildBrowse()
	_, _ = m.updateBrowseKeys(tea.KeyMsg{Type: tea.KeyCtrlA})
	if !m.isMultiSelected(1) || !m.isMultiSelected(3) {
		t.Fatalf("Ctrl+A did not preserve hidden selection: %v", m.multiSelected)
	}
	_, _ = m.updateBrowseKeys(tea.KeyMsg{Type: tea.KeyCtrlA})
	if m.isMultiSelected(1) || !m.isMultiSelected(3) {
		t.Fatalf("second Ctrl+A altered wrong IDs: %v", m.multiSelected)
	}
	m.tagTI.SetValue("")
	m.rebuildBrowse()
	if !m.isMultiSelected(3) {
		t.Fatal("hidden selected ID did not survive clearing filter")
	}
}

func TestCommandMultiselectionRejectsTransientRowsAndPrunesReloads(t *testing.T) {
	m := newCommandTestModel([]model.Entry{{Command: "transient"}}, 3)
	_, _ = m.updateBrowseKeys(tea.KeyMsg{Type: tea.KeySpace})
	if len(m.multiSelected) != 0 || m.commandStatus.text == "" {
		t.Fatalf("transient row selection state = %v status=%q", m.multiSelected, m.commandStatus.text)
	}

	m = newCommandTestModel(commandEntries(3), 3)
	m.multiSelected[1] = struct{}{}
	m.multiSelected[2] = struct{}{}
	_, _ = m.Update(entriesMsg{entries: []model.Entry{{ID: 2, Command: "bravo"}, {ID: 3, Command: "charlie"}}})
	if m.isMultiSelected(1) || !m.isMultiSelected(2) {
		t.Fatalf("reload pruning = %v", m.multiSelected)
	}
	_, _ = m.Update(entriesMsg{entries: nil})
	if len(m.multiSelected) != 0 {
		t.Fatalf("empty reload retained selections: %v", m.multiSelected)
	}
}

func TestMultiselectionSurvivesNavigationReorderAndViewportSlicing(t *testing.T) {
	m := newCommandTestModel(commandEntries(10), 3)
	m.multiSelected[2] = struct{}{}
	m.multiSelected[8] = struct{}{}
	m.selectBrowseIndex(m.rowIndexByID(8))
	m.jumpBrowseGroup(-1)
	m.moveBrowseBy(1)
	m.ensureBrowseVisible(true)
	if !m.isMultiSelected(2) || !m.isMultiSelected(8) {
		t.Fatalf("navigation changed selected IDs: %v", m.multiSelected)
	}
	m.allEntries[7].Command = "aaa-moved"
	m.rebuildBrowse()
	if !m.isMultiSelected(8) {
		t.Fatal("reorder lost selected ID 8")
	}
	m.selectBrowseIndex(m.rowIndexByID(8))
	rendered, _ := m.renderCommandViewport(m.commandContentWidth(), m.commandListHeight(m.commandContentWidth()), m.browseOffset)
	if !strings.Contains(rendered, "aaa-moved") || !strings.Contains(rendered, "✓") || !strings.Contains(rendered, "›") {
		t.Fatalf("viewport does not show focused + selected logical row: %q", rendered)
	}
}

func TestDeleteCompletionRemovesMultiselectionID(t *testing.T) {
	m := newCommandTestModel(commandEntries(2), 3)
	m.selectBrowseIndex(0)
	m.detailEntry = m.cmdRows[0].Entry
	deletedID := m.detailEntry.ID
	m.multiSelected[deletedID] = struct{}{}
	m.deleteFromBrowse = true
	m.cmdPhase = commandsDeleteConfirm
	_, _ = m.Update(deleteMsg{})
	if m.isMultiSelected(deletedID) || len(m.multiSelected) != 0 || m.commandStatus.text != "Deleted" {
		t.Fatalf("delete completion retained ID: %v", m.multiSelected)
	}
}

type fakeClipboard struct {
	value string
	err   error
}

func (f *fakeClipboard) WriteText(value string) error {
	f.value = value
	return f.err
}

func TestCopyUsesFocusedCommandVerbatimAndReportsStatus(t *testing.T) {
	command := "printf '%s' \"$HOME\" `whoami`\n日本語"
	m := newCommandTestModel([]model.Entry{{ID: 1, Command: command, Description: "excluded", Tags: []string{"excluded"}}}, 5)
	m.multiSelected[1] = struct{}{}
	fake := &fakeClipboard{}
	m.clip = fake
	_, cmd := m.updateBrowseKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	msg := cmd()
	_, expiry := m.Update(msg)
	if fake.value != command {
		t.Fatalf("clipboard value = %q, want %q", fake.value, command)
	}
	if m.commandStatus.text != "Copied" || m.commandStatus.isError || expiry == nil {
		t.Fatalf("copy status = %#v", m.commandStatus)
	}

	fake.err = errors.New("unavailable")
	_, cmd = m.updateBrowseKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	_, _ = m.Update(cmd())
	if !m.commandStatus.isError || !strings.Contains(m.commandStatus.text, "unavailable") {
		t.Fatalf("copy error status = %#v", m.commandStatus)
	}
}

func TestCopyOnEmptyListDoesNotTouchClipboard(t *testing.T) {
	m := newCommandTestModel(nil, 3)
	fake := &fakeClipboard{}
	m.clip = fake
	_, cmd := m.updateBrowseKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	if fake.value != "" || m.commandStatus.text == "" || cmd == nil {
		t.Fatalf("empty copy value/status/cmd = %q/%q/%v", fake.value, m.commandStatus.text, cmd)
	}
}

func TestCommandStatusExpiryRejectsStaleTimers(t *testing.T) {
	m := newCommandTestModel(commandEntries(2), 3)
	selectedID := m.selectedID
	_ = m.setCommandStatus("first", false)
	firstGeneration := m.commandStatus.generation
	_ = m.setCommandStatus("second", true)
	secondGeneration := m.commandStatus.generation
	m.expireCommandStatus(commandStatusExpiredMsg{generation: firstGeneration})
	if m.commandStatus.text != "second" || !m.commandStatus.isError {
		t.Fatalf("stale expiry cleared newer status: %#v", m.commandStatus)
	}
	m.expireCommandStatus(commandStatusExpiredMsg{generation: secondGeneration})
	if m.commandStatus.text != "" || m.selectedID != selectedID {
		t.Fatalf("current expiry state = %#v selectedID=%d", m.commandStatus, m.selectedID)
	}
}

func TestCommandStatusRendersSuccessAndErrorWithoutChangingSelection(t *testing.T) {
	m := newCommandTestModel(commandEntries(2), 3)
	wantID := m.selectedID
	_ = m.setCommandStatus("Copied", false)
	if rendered := m.commandStatusBlock(76); !strings.Contains(rendered, "Copied") {
		t.Fatalf("success status not rendered: %q", rendered)
	}
	_ = m.setCommandStatus("Copy failed", true)
	if rendered := m.commandStatusBlock(76); !strings.Contains(rendered, "Copy failed") {
		t.Fatalf("error status not rendered: %q", rendered)
	}
	m.cmdPhase = commandsDetail
	m.detailEntry = m.cmdRows[m.browseCursor].Entry
	if rendered := m.commandsDetailView(76); !strings.Contains(rendered, "Copy failed") {
		t.Fatalf("detail status not rendered: %q", rendered)
	}
	if m.selectedID != wantID {
		t.Fatalf("status rendering changed selected ID to %d, want %d", m.selectedID, wantID)
	}
}

func TestBulkTagFormNormalizationCancellationAndSelectionRequirement(t *testing.T) {
	m := newCommandTestModel(commandEntries(1), 3)
	_, _ = m.updateBrowseKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
	if m.cmdPhase != commandsBrowse || !m.commandStatus.isError {
		t.Fatalf("bulk action without selection phase/status = %d/%#v", m.cmdPhase, m.commandStatus)
	}
	m.multiSelected[1] = struct{}{}
	_, _ = m.updateBrowseKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
	m.bulkTagForm.addTI.SetValue(" Shared,shared, New ")
	m.bulkTagForm.removeTI.SetValue(" OLD,shared ")
	want := []storage.BulkTagChange{
		{Operation: storage.BulkTagRemove, Tag: "OLD"},
		{Operation: storage.BulkTagRemove, Tag: "shared"},
		{Operation: storage.BulkTagAdd, Tag: "Shared"},
		{Operation: storage.BulkTagAdd, Tag: "New"},
	}
	if got := m.bulkTagForm.changes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("bulk changes = %#v, want %#v", got, want)
	}
	_, _ = m.updateBulkTagForm(tea.KeyMsg{Type: tea.KeyEsc})
	if m.cmdPhase != commandsBrowse || len(m.bulkTargetIDs) != 0 {
		t.Fatalf("bulk cancel state phase=%d targets=%v", m.cmdPhase, m.bulkTargetIDs)
	}

	m.cmdPhase = commandsBulkTags
	m.bulkTagForm.prepare()
	_, cmd := m.updateBulkTagForm(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd != nil || m.errBanner == "" {
		t.Fatalf("empty bulk form submitted: cmd=%v error=%q", cmd, m.errBanner)
	}
}

func TestLowercaseQStillQuitsEntryAndBulkTagForms(t *testing.T) {
	m := newCommandTestModel(commandEntries(1), 3)
	m.cmdPhase = commandsEdit
	m.form.prepareEdit(m.cmdRows[0].Entry)
	_, cmd := m.updateForm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}, true)
	if cmd == nil {
		t.Fatal("lowercase q did not quit the entry form")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("entry-form q command returned %T, want tea.QuitMsg", cmd())
	}

	m.cmdPhase = commandsBulkTags
	m.bulkTagForm.prepare()
	_, cmd = m.updateBulkTagForm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("lowercase q did not quit the bulk-tag form")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("bulk-form q command returned %T, want tea.QuitMsg", cmd())
	}
}

func TestBulkTagsAreAtomicAndReconcileFocusAndSelection(t *testing.T) {
	repo := newMessageTestRepo(t)
	ctx := context.Background()
	ids, err := repo.CreateEntries(ctx, []model.Entry{
		{Command: "alpha", Tags: []string{"Primary", "Old"}, Type: model.EntryTypeManual},
		{Command: "bravo", Tags: []string{"Primary"}, Type: model.EntryTypeManual},
	})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := repo.GetAllEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	m := New(Options{Repo: repo, Clipboard: &fakeClipboard{}}).(*Model)
	m.width, m.height = 80, 24
	m.screen, m.cmdPhase, m.cmdFocus = ScreenCommands, commandsBrowse, commandsFocusList
	m.allEntries = entries
	m.rebuildBrowse()
	m.selectBrowseIndex(m.rowIndexByID(ids[0]))
	m.multiSelected[ids[0]] = struct{}{}
	m.multiSelected[ids[1]] = struct{}{}
	m.tagTI.SetValue("Primary")
	m.rebuildBrowse()

	changes := []storage.BulkTagChange{
		{Operation: storage.BulkTagRemove, Tag: "primary"},
		{Operation: storage.BulkTagAdd, Tag: "New"},
	}
	msg := bulkTagsCmd(repo, m.selectedEntryIDs(), changes)().(bulkTagsMsg)
	if msg.err != nil {
		t.Fatal(msg.err)
	}
	_, _ = m.Update(msg)
	reloaded, err := repo.GetAllEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = m.Update(entriesMsg{entries: reloaded})
	if len(m.cmdRows) != 0 || m.restoreID != ids[0] {
		t.Fatalf("filtered post-update rows/restore = %d/%d", len(m.cmdRows), m.restoreID)
	}
	if !m.isMultiSelected(ids[0]) || !m.isMultiSelected(ids[1]) {
		t.Fatalf("bulk success lost selected IDs: %v", m.multiSelected)
	}
	m.tagTI.SetValue("")
	m.rebuildBrowse()
	assertBrowseSelection(t, m, ids[0])
	for _, entry := range reloaded {
		if !reflect.DeepEqual(entry.Tags, []string{"Old", "New"}) && !reflect.DeepEqual(entry.Tags, []string{"New"}) {
			t.Fatalf("unexpected normalized bulk tags: %#v", entry.Tags)
		}
	}
}

func TestBulkTagsFailureRollsBackAndReportsTemporaryError(t *testing.T) {
	repo := newMessageTestRepo(t)
	ctx := context.Background()
	id, err := repo.CreateEntry(ctx, model.Entry{Command: "alpha", Tags: []string{"Old"}, Type: model.EntryTypeManual})
	if err != nil {
		t.Fatal(err)
	}
	m := New(Options{Repo: repo}).(*Model)
	m.screen, m.cmdPhase = ScreenCommands, commandsBulkTags
	m.bulkTargetIDs = []int64{id, id + 9999}
	msg := bulkTagsCmd(repo, m.bulkTargetIDs, []storage.BulkTagChange{{Operation: storage.BulkTagAdd, Tag: "New"}})().(bulkTagsMsg)
	if msg.err == nil {
		t.Fatal("missing ID bulk update unexpectedly succeeded")
	}
	_, _ = m.Update(msg)
	if !m.commandStatus.isError || m.cmdPhase != commandsBrowse {
		t.Fatalf("bulk failure status/phase = %#v/%d", m.commandStatus, m.cmdPhase)
	}
	entry, err := repo.GetEntryByCommand(ctx, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(entry.Tags, []string{"Old"}) {
		t.Fatalf("failed bulk action partially changed storage: %v", entry.Tags)
	}
}

func TestResponsiveFooterIsSingleLineAndWidthBounded(t *testing.T) {
	m := newCommandTestModel(commandEntries(2), 3)
	for _, width := range []int{0, 8, 20, 40, 80, 240} {
		m.width = width
		footer := m.footerContent()
		available := width - m.styles.Padding.GetHorizontalFrameSize()
		if width == 0 {
			available = 80 - m.styles.Padding.GetHorizontalFrameSize()
		}
		if strings.Contains(footer, "\n") || lipgloss.Width(footer) > max(available, 0) {
			t.Fatalf("footer width/newline at %d: visual=%d value=%q", width, lipgloss.Width(footer), footer)
		}
	}
	m.width = 400
	footer := m.footerContent()
	for _, hint := range []string{"Space", "^A", "T", "C", "Pg", "Home/End", "[ ]"} {
		if !strings.Contains(footer, hint) {
			t.Fatalf("wide footer missing %q: %q", hint, footer)
		}
	}
}
