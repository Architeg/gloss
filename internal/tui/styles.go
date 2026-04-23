package tui

import "github.com/charmbracelet/lipgloss"

const maxBodyWidth = 72

// Styles holds Lip Gloss styles for a dark-terminal-first, minimal layout.
type Styles struct {
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Item     lipgloss.Style
	Selected lipgloss.Style
	Blurb    lipgloss.Style
	Footer   lipgloss.Style
	Padding  lipgloss.Style
}

func newStyles() Styles {
	accent := lipgloss.Color("#7aa2f7")

	return Styles{
		Title: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#222222", Dark: "#f0f0f0"}).
			Bold(true),
		Subtitle: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#9a9a9a"}),
		Item: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#e6e6e6"}),
		Selected: lipgloss.NewStyle().
			Foreground(accent),
		Blurb: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#777777", Dark: "#7a7a7a"}),
		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#888888", Dark: "#5a5a5a"}),
		Padding: lipgloss.NewStyle().
			Padding(0, 2),
	}
}

func contentWidth(termWidth int) int {
	w := maxBodyWidth
	if termWidth > 0 {
		inner := termWidth - 4
		if inner > 0 && inner < w {
			w = inner
		}
	}
	if w < 20 {
		w = 20
	}
	return w
}
