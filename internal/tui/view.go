package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View implements tea.Model.
func (m Model) View() string {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}

	footer := m.footerLine()
	footerBlock := m.styles.Padding.Render(m.styles.Footer.Render(footer))
	footerH := lipgloss.Height(footerBlock)
	mainH := h - footerH
	if mainH < 1 {
		mainH = 1
	}

	cw := contentWidth(w)
	main := m.styles.Padding.Render(m.mainBlock(cw))
	main = lipgloss.NewStyle().Width(w).Height(mainH).Render(
		lipgloss.Place(w, mainH, lipgloss.Left, lipgloss.Top, main),
	)

	return lipgloss.JoinVertical(lipgloss.Left, main, footerBlock)
}

func (m Model) mainBlock(width int) string {
	if m.screen == ScreenHome {
		return m.homeView(width)
	}
	return m.placeholderView(width)
}

func (m Model) homeView(width int) string {
	var b strings.Builder
	b.WriteString(m.styles.Title.Width(width).Render("Gloss"))
	b.WriteString("\n")
	b.WriteString(m.styles.Subtitle.Width(width).Render("Command glossary and alias helper"))
	b.WriteString("\n\n")

	for i, item := range HomeMenu {
		style := m.styles.Item
		if i == m.cursor {
			style = m.styles.Selected
		}
		line := style.Width(width).Render(item.Title)
		b.WriteString(line)
		if i < len(HomeMenu)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m Model) placeholderView(width int) string {
	title := screenTitle(m.screen)
	blurb := placeholderBlurb(m.screen)
	var b strings.Builder
	b.WriteString(m.styles.Title.Width(width).Render(title))
	b.WriteString("\n\n")
	b.WriteString(m.styles.Blurb.Width(width).Render(blurb))
	return b.String()
}

func (m Model) footerLine() string {
	if m.screen == ScreenHome {
		return "↑↓ Move    Enter Select    Q Quit"
	}
	return "Esc Back    ← Back    Q Quit"
}
