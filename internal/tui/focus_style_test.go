package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/Architeg/gloss/internal/model"
)

var focusedBackgroundANSI = regexp.MustCompile(`48;2;(?:89|90);71;142`)

func TestFocusedRowPaletteIsCentralized(t *testing.T) {
	styles := newStyles()
	wantBackground := lipgloss.Color("#5A478E")
	wantForeground := lipgloss.Color("#FFFFFF")
	for name, style := range map[string]lipgloss.Style{
		"row":         styles.FocusedRow,
		"command":     styles.CmdSelected,
		"description": styles.DescSelected,
	} {
		if got := style.GetBackground(); got != wantBackground {
			t.Fatalf("%s focused background = %v, want %v", name, got, wantBackground)
		}
		if got := style.GetForeground(); got != wantForeground {
			t.Fatalf("%s focused foreground = %v, want %v", name, got, wantForeground)
		}
	}
	if got := styles.CmdCol.GetBackground(); got == wantBackground {
		t.Fatal("ordinary command style unexpectedly has the focused background")
	}
	if got := styles.CategoryAccent.GetBackground(); got == wantBackground {
		t.Fatal("unfocused multiselection marker unexpectedly has the focused background")
	}
}

func TestFocusedCommandRowsPreserveStateMarkersAndWrappedWidth(t *testing.T) {
	useTrueColor(t)
	entries := []model.Entry{
		{ID: 1, Command: strings.Repeat("focused command ", 5), Description: strings.Repeat("focused description ", 6), Tags: []string{"Category"}},
		{ID: 2, Command: strings.Repeat("ordinary command ", 5), Description: strings.Repeat("ordinary description ", 6), Tags: []string{"Category"}},
	}
	m := newCommandTestModel(entries, 8)
	m.multiSelected[2] = struct{}{}

	focused := m.renderCommandEntry(76, 0)
	lines := strings.Split(focused, "\n")
	if len(lines) < 2 {
		t.Fatalf("focused row did not wrap for coverage: %q", stripANSI(focused))
	}
	for _, line := range lines {
		if !focusedBackgroundANSI.MatchString(line) {
			t.Fatalf("wrapped focused line lacks active background: %q", line)
		}
		if got := lipgloss.Width(line); got > 76 {
			t.Fatalf("ANSI-styled focused line width %d exceeds 76: %q", got, line)
		}
	}

	unfocusedSelected := m.renderCommandEntry(76, 1)
	if focusedBackgroundANSI.MatchString(unfocusedSelected) {
		t.Fatalf("unfocused selected row received focused background: %q", unfocusedSelected)
	}
	if !strings.Contains(stripANSI(unfocusedSelected), "✓") {
		t.Fatalf("unfocused selected row lost its selection marker: %q", stripANSI(unfocusedSelected))
	}

	m.multiSelected[1] = struct{}{}
	focusedSelected := m.renderCommandEntry(76, 0)
	if !focusedBackgroundANSI.MatchString(focusedSelected) || !strings.Contains(stripANSI(focusedSelected), "✓") {
		t.Fatalf("focused selected row lost background or marker: %q", stripANSI(focusedSelected))
	}
}

func TestAliasAndScanListsUseFocusedRowPalette(t *testing.T) {
	useTrueColor(t)

	aliases := New(Options{}).(*Model)
	aliases.aliasPhase = aliasPhaseView
	aliases.allEntries = []model.Entry{
		{ID: 1, Command: "focused_alias", Target: "echo focused", Type: model.EntryTypeAlias, ManagedAlias: true},
		{ID: 2, Command: "ordinary_alias", Target: "echo ordinary", Type: model.EntryTypeAlias, ManagedAlias: true},
	}
	aliasView := aliases.aliasListView(76)
	assertFocusedListLine(t, aliasView, "focused_alias", true)
	assertFocusedListLine(t, aliasView, "ordinary_alias", false)

	scan := New(Options{}).(*Model)
	scan.scanSources = []string{"temporary source"}
	scan.scanRows = []model.ScanSuggestion{
		{Command: "focused-scan", Type: "alias", Target: "target"},
		{Command: "ordinary-scan", Type: "alias", Target: "target"},
	}
	scanView := scan.scanView(76)
	assertFocusedListLine(t, scanView, "focused-scan", true)
	assertFocusedListLine(t, scanView, "ordinary-scan", false)
}

func useTrueColor(t *testing.T) {
	t.Helper()
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
}

func assertFocusedListLine(t *testing.T, rendered, text string, focused bool) {
	t.Helper()
	for _, line := range strings.Split(rendered, "\n") {
		if !strings.Contains(stripANSI(line), text) {
			continue
		}
		if got := focusedBackgroundANSI.MatchString(line); got != focused {
			t.Fatalf("line %q focused background = %v, want %v", stripANSI(line), got, focused)
		}
		if lipgloss.Width(line) != lipgloss.Width(stripANSI(line)) {
			t.Fatalf("ANSI changed row width for %q", text)
		}
		return
	}
	t.Fatalf("rendered list omitted %q: %q", text, stripANSI(rendered))
}
