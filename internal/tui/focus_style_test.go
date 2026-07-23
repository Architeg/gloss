package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/Architeg/gloss/internal/model"
)

var (
	focusedBackgroundANSI   = regexp.MustCompile(`48;2;69;64;100`)
	quantizedBackgroundANSI = regexp.MustCompile(`48;2;68;64;100`)
	obsoleteBackgroundANSI  = regexp.MustCompile(`48;2;(?:89|90);71;142`)
	pinkForegroundANSI      = regexp.MustCompile(`38;2;232;120;200`)
	neutralForegroundANSI   = regexp.MustCompile(`38;2;236;232;226`)
)

func TestFocusedRowPaletteIsCentralized(t *testing.T) {
	styles := newStyles()
	wantBackground := lipgloss.Color("#454064")
	wantForeground := lipgloss.Color("#ECE8E2")
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
	if got := styles.FocusMarker.GetBackground(); got != wantBackground {
		t.Fatalf("focus marker background = %v, want %v", got, wantBackground)
	}
	if got, want := styles.FocusMarker.GetForeground(), lipgloss.Color("#e878c8"); got != want {
		t.Fatalf("focus marker foreground = %v, want %v", got, want)
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
	if obsoleteBackgroundANSI.MatchString(focused) {
		t.Fatalf("focused command still emits obsolete background: %q", focused)
	}
	if quantizedBackgroundANSI.MatchString(focused) {
		t.Fatalf("focused command emitted quantized background instead of RGB 69,64,100: %q", focused)
	}
	if !pinkForegroundANSI.MatchString(focused) {
		t.Fatalf("focused command arrow lost pink accent: %q", focused)
	}
	if count := len(pinkForegroundANSI.FindAllString(focused, -1)); count != 1 {
		t.Fatalf("focused command emits pink outside its arrow: count=%d row=%q", count, focused)
	}
	if !neutralForegroundANSI.MatchString(focused) {
		t.Fatalf("focused command text lacks neutral off-white foreground: %q", focused)
	}
	if strings.Contains(stripANSI(focused), "✓") {
		t.Fatalf("focused but unselected row gained a selection marker: %q", stripANSI(focused))
	}
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
	if focusedBackgroundANSI.MatchString(unfocusedSelected) || quantizedBackgroundANSI.MatchString(unfocusedSelected) {
		t.Fatalf("unfocused selected row received focused background: %q", unfocusedSelected)
	}
	if !strings.Contains(stripANSI(unfocusedSelected), "✓") {
		t.Fatalf("unfocused selected row lost its selection marker: %q", stripANSI(unfocusedSelected))
	}
	if !pinkForegroundANSI.MatchString(unfocusedSelected) {
		t.Fatalf("unfocused selection marker lost pink accent: %q", unfocusedSelected)
	}

	m.multiSelected[1] = struct{}{}
	focusedSelected := m.renderCommandEntry(76, 0)
	if !focusedBackgroundANSI.MatchString(focusedSelected) || !strings.Contains(stripANSI(focusedSelected), "✓") {
		t.Fatalf("focused selected row lost background or marker: %q", stripANSI(focusedSelected))
	}
	if count := len(pinkForegroundANSI.FindAllString(focusedSelected, -1)); count != 2 {
		t.Fatalf("focused arrow and selection marker are not both pink: %q", focusedSelected)
	}
}

func TestAliasAndScanListsUseFocusedRowPalette(t *testing.T) {
	useTrueColor(t)

	aliases := New(Options{}).(*Model)
	aliases.config = &model.Config{ShellFile: "/tmp/gloss-focus-test-shell"}
	aliases.aliasPhase = aliasPhaseView
	aliases.allEntries = []model.Entry{
		{ID: 1, Command: "focused_alias", Target: "echo focused", Type: model.EntryTypeAlias, ManagedAlias: true},
		{ID: 2, Command: "ordinary_alias", Target: "echo ordinary", Type: model.EntryTypeAlias, ManagedAlias: true},
	}
	aliasView := aliases.aliasListView(76)
	assertFocusedListLine(t, aliasView, "focused_alias", true)
	assertFocusedListLine(t, aliasView, "ordinary_alias", false)
	if line := rawLineContaining(aliasView, "focused_alias"); len(pinkForegroundANSI.FindAllString(line, -1)) != 1 || !neutralForegroundANSI.MatchString(line) {
		t.Fatalf("focused managed alias does not use pink arrow plus neutral text: %q", line)
	}
	aliases.aliasPhase = aliasPhaseMenu
	aliasMenu := aliases.aliasMenuView(76)
	menuTitle := aliasMenuHome[0].title
	assertFocusedListLine(t, aliasMenu, menuTitle, true)
	if line := rawLineContaining(aliasMenu, menuTitle); len(pinkForegroundANSI.FindAllString(line, -1)) != 1 || !neutralForegroundANSI.MatchString(line) {
		t.Fatalf("focused Aliases menu row does not use shared composition: %q", line)
	}

	scan := New(Options{}).(*Model)
	scan.scanSources = []string{"temporary source"}
	scan.scanRows = []model.ScanSuggestion{
		{Command: "focused-scan", Type: "alias", Target: "target", Selected: true},
		{Command: "selected-scan", Type: "alias", Target: "target", Selected: true},
		{Command: "ordinary-scan", Type: "alias", Target: "target"},
	}
	scanView := scan.scanView(76)
	assertFocusedListLine(t, scanView, "focused-scan", true)
	assertFocusedListLine(t, scanView, "selected-scan", false)
	assertFocusedListLine(t, scanView, "ordinary-scan", false)
	if line := rawLineContaining(scanView, "focused-scan"); len(pinkForegroundANSI.FindAllString(line, -1)) != 2 || !neutralForegroundANSI.MatchString(line) {
		t.Fatalf("focused Scan row lacks pink indicators or neutral text: %q", line)
	}
	if line := rawLineContaining(scanView, "selected-scan"); !pinkForegroundANSI.MatchString(line) || focusedBackgroundANSI.MatchString(line) {
		t.Fatalf("unfocused selected Scan row has wrong state styling: %q", line)
	}
	scan.scanCursor = 2
	focusedUnselectedScan := scan.scanView(76)
	if line := rawLineContaining(focusedUnselectedScan, "ordinary-scan"); !focusedBackgroundANSI.MatchString(line) || quantizedBackgroundANSI.MatchString(line) || len(pinkForegroundANSI.FindAllString(line, -1)) != 1 || !neutralForegroundANSI.MatchString(line) || strings.Contains(stripANSI(line), "[x]") {
		t.Fatalf("focused but unselected Scan row has wrong state styling: %q", line)
	}
	if line := rawLineContaining(focusedUnselectedScan, "focused-scan"); focusedBackgroundANSI.MatchString(line) || quantizedBackgroundANSI.MatchString(line) || !pinkForegroundANSI.MatchString(line) {
		t.Fatalf("selected but unfocused Scan row has wrong state styling after moving focus: %q", line)
	}
	if obsoleteBackgroundANSI.MatchString(aliasView) || obsoleteBackgroundANSI.MatchString(aliasMenu) || obsoleteBackgroundANSI.MatchString(scanView) {
		t.Fatal("alias or Scan list still emits obsolete focused background")
	}
	for name, rendered := range map[string]string{"aliases": aliasView, "alias menu": aliasMenu, "scan": scanView, "unselected scan focus": focusedUnselectedScan} {
		if quantizedBackgroundANSI.MatchString(rendered) {
			t.Fatalf("%s emitted quantized focused background RGB 68,64,100: %q", name, rendered)
		}
		if strings.Contains(rendered, "48;5;") {
			t.Fatalf("%s emitted an ANSI256 background under true-color profile: %q", name, rendered)
		}
	}
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

func rawLineContaining(rendered, text string) string {
	for _, line := range strings.Split(rendered, "\n") {
		if strings.Contains(stripANSI(line), text) {
			return line
		}
	}
	return ""
}
