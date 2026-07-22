package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type commandHelpItem struct {
	keys        string
	description string
}

var commandHelpItems = []commandHelpItem{
	{keys: "↑/↓ and j/k", description: "Move between commands"},
	{keys: "Space", description: "Select or deselect the focused command"},
	{keys: "Ctrl+A", description: "Select or deselect all commands visible under the current filter"},
	{keys: "C", description: "Copy the focused command"},
	{keys: "T", description: "Add or remove tags from selected commands"},
	{keys: "/", description: "Search commands"},
	{keys: "F", description: "Filter by tag"},
	{keys: "[ / ]", description: "Move to the previous or next category"},
	{keys: "Page Up / Page Down", description: "Move one page"},
	{keys: "Home / End", description: "Move to the first or last command"},
	{keys: "Enter", description: "Open the focused command details"},
	{keys: "A", description: "Add a command"},
	{keys: "E", description: "Edit the focused command"},
	{keys: "D", description: "Delete the focused command"},
	{keys: "Esc / ← / h", description: "Return to the home screen"},
	{keys: "Q / Ctrl+C", description: "Quit Gloss"},
	{keys: "?", description: "Open or close shortcut help"},
}

func (m *Model) commandHelpView(width int) string {
	fixed := m.commandHelpFixedBlock(width)
	available := m.mainContentHeight()
	if available <= 0 {
		return ""
	}
	fixedHeight := lipgloss.Height(fixed)
	if fixedHeight >= available {
		return firstRenderedLines(fixed, available)
	}

	lines := m.commandHelpLines(width)
	pageHeight := available - fixedHeight
	maxOffset := max(len(lines)-pageHeight, 0)
	m.commandHelpOffset = clamp(m.commandHelpOffset, 0, maxOffset)
	end := min(m.commandHelpOffset+pageHeight, len(lines))
	return fixed + strings.Join(lines[m.commandHelpOffset:end], "\n")
}

func (m *Model) commandHelpFixedBlock(width int) string {
	if width <= 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.sectionTitleBlock(width, "Command shortcuts"))
	if m.height > 0 && m.height <= compactCommandScreenHeight {
		b.WriteString("\n")
		return b.String()
	}
	b.WriteString("\n\n")
	b.WriteString(m.styles.EmptyHint.Render(truncateScanTail("↑↓ scroll · ? or Esc close", width)))
	b.WriteString("\n\n")
	return b.String()
}

func (m *Model) commandHelpLines(width int) []string {
	if width <= 0 {
		return nil
	}
	lines := make([]string, 0, len(commandHelpItems)*3)
	for i, item := range commandHelpItems {
		lines = append(lines, m.styles.FooterKey.Render(truncateScanTail(item.keys, width)))
		for _, line := range wrapVisual(item.description, width) {
			lines = append(lines, m.styles.FooterLbl.Render(line))
		}
		if i < len(commandHelpItems)-1 {
			lines = append(lines, "")
		}
	}
	return lines
}

func (m *Model) commandHelpPageHeight() int {
	width := m.commandContentWidth()
	height := m.mainContentHeight() - lipgloss.Height(m.commandHelpFixedBlock(width))
	return max(height, 0)
}

func (m *Model) clampCommandHelpOffset() {
	if !m.commandHelpOpen {
		return
	}
	lines := m.commandHelpLines(m.commandContentWidth())
	maxOffset := max(len(lines)-m.commandHelpPageHeight(), 0)
	m.commandHelpOffset = clamp(m.commandHelpOffset, 0, maxOffset)
}

func (m *Model) updateCommandHelp(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch {
	case m.keys.shouldQuit(km):
		return m, tea.Quit
	case km.Type == tea.KeyEsc || km.String() == "?":
		m.commandHelpOpen = false
		return m, nil
	case key.Matches(km, m.keys.Up):
		m.commandHelpOffset--
	case key.Matches(km, m.keys.Down):
		m.commandHelpOffset++
	case key.Matches(km, m.keys.PageUp):
		m.commandHelpOffset -= max(m.commandHelpPageHeight(), 1)
	case key.Matches(km, m.keys.PageDown):
		m.commandHelpOffset += max(m.commandHelpPageHeight(), 1)
	case key.Matches(km, m.keys.Home):
		m.commandHelpOffset = 0
	case key.Matches(km, m.keys.End):
		m.commandHelpOffset = len(m.commandHelpLines(m.commandContentWidth()))
	}
	m.clampCommandHelpOffset()
	return m, nil
}
