package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const glossRepoURL = "https://github.com/Architeg/gloss"

const glossTagline = "Command glossary and alias helper"

// Thin slash/underscore wordmark (original; not block glyphs).
func glossWordmarkLines() []string {
	return []string{
		`   ____ _`,
		`  / ___| | ___  ___ ___`,
		` | |  _| |/ _ \/ __/ __|`,
		` | |_| | | (_) \__ \__ \`,
		`  \____|_|\___/|___/___/`,
	}
}

func (m *Model) renderHomeBanner(termWidth int) string {
	var lines []string
	for _, L := range glossWordmarkLines() {
		lines = append(lines, m.styles.BannerMark.Render(strings.TrimRight(L, " ")))
	}
	left := strings.Join(lines, "\n")

	right := lipgloss.JoinVertical(
		lipgloss.Left,
		m.styles.RepoURL.Render(glossRepoURL),
		m.styles.BannerTagline.Render(glossTagline),
	)

	row := lipgloss.JoinHorizontal(lipgloss.Bottom, left, "  ", right)
	if termWidth > 0 && lipgloss.Width(row) > termWidth {
		row = lipgloss.JoinVertical(lipgloss.Left, left, "", right)
	}
	return row
}
