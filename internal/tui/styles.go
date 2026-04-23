package tui

import "github.com/charmbracelet/lipgloss"

const maxBodyWidth = 76

// Styles: MOLE-inspired palette — warm white, muted gray, cyan, green, magenta accents.
type Styles struct {
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Item     lipgloss.Style
	Selected lipgloss.Style
	Err      lipgloss.Style
	Padding  lipgloss.Style

	BannerMark    lipgloss.Style
	RepoURL       lipgloss.Style
	BannerTagline lipgloss.Style
	BannerMeta    lipgloss.Style

	HomeDesc    lipgloss.Style
	HomeSelDesc lipgloss.Style
	SelCaret    lipgloss.Style

	CategoryAccent lipgloss.Style
	CategoryPrefix lipgloss.Style
	CategoryName   lipgloss.Style
	Divider        lipgloss.Style
	FilterLabel    lipgloss.Style
	FilterWrap     lipgloss.Style
	FilterSep      lipgloss.Style
	CmdCol         lipgloss.Style
	DescCol        lipgloss.Style
	CmdSelected    lipgloss.Style
	DescSelected   lipgloss.Style
	EmptyHint      lipgloss.Style

	FieldLabel lipgloss.Style
	FieldValue lipgloss.Style

	InputPrompt      lipgloss.Style
	InputText        lipgloss.Style
	InputPlaceholder lipgloss.Style

	FooterKey lipgloss.Style
	FooterLbl lipgloss.Style
	FooterBar lipgloss.Style
}

func newStyles() Styles {
	cyan := lipgloss.Color("#5ecae0")
	cyanURL := lipgloss.Color("#7dd8f0")
	green := lipgloss.Color("#90d4a8")
	magenta := lipgloss.Color("#e878c8")
	magentaSoft := lipgloss.Color("#f0a8d8")

	primary := lipgloss.AdaptiveColor{Light: "#1c1c1c", Dark: "#ece8e2"}
	titleFG := lipgloss.AdaptiveColor{Light: "#0a0a0a", Dark: "#faf6f0"}
	secondary := lipgloss.AdaptiveColor{Light: "#5a5a5a", Dark: "#a4a2ab"}
	tertiary := lipgloss.AdaptiveColor{Light: "#777777", Dark: "#7f7d86"}
	dim := lipgloss.AdaptiveColor{Light: "#999999", Dark: "#61646b"}
	labelMuted := lipgloss.AdaptiveColor{Light: "#888888", Dark: "#5f636b"}
	valueBright := lipgloss.AdaptiveColor{Light: "#111111", Dark: "#f4efe9"}

	return Styles{
		Title: lipgloss.NewStyle().
			Foreground(titleFG).
			Bold(true),

		Subtitle: lipgloss.NewStyle().
			Foreground(secondary),

		Item: lipgloss.NewStyle().
			Foreground(primary),

		Selected: lipgloss.NewStyle().
			Foreground(magenta).
			Bold(true),

		Err: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#b00020", Dark: "#e0787d"}),

		Padding: lipgloss.NewStyle().
			Padding(1, 3),

		BannerMark: lipgloss.NewStyle().
			Foreground(cyan),

		RepoURL: lipgloss.NewStyle().
			Foreground(cyanURL).
			Bold(true),

		BannerTagline: lipgloss.NewStyle().
			Foreground(green),

		BannerMeta: lipgloss.NewStyle().
			Foreground(dim),

		HomeDesc: lipgloss.NewStyle().
			Foreground(tertiary),

		HomeSelDesc: lipgloss.NewStyle().
			Foreground(magentaSoft),

		SelCaret: lipgloss.NewStyle().
			Foreground(magenta).
			Bold(true),

		CategoryAccent: lipgloss.NewStyle().
			Foreground(magenta),

		CategoryPrefix: lipgloss.NewStyle().
			Foreground(dim),

		CategoryName: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#ebe7e1"}).
			Bold(true),

		Divider: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#cccccc", Dark: "#4b5058"}),

		FilterLabel: lipgloss.NewStyle().
			Foreground(magentaSoft).
			Bold(true).
			Width(9),

		FilterWrap: lipgloss.NewStyle().
			MarginBottom(1).
			Padding(0, 0, 1, 1),

		FilterSep: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#d0d0d0", Dark: "#41464e"}),

		CmdCol: lipgloss.NewStyle().
			Foreground(primary).
			Bold(true),

		DescCol: lipgloss.NewStyle().
			Foreground(tertiary),

		CmdSelected: lipgloss.NewStyle().
			Foreground(magentaSoft).
			Bold(true),

		DescSelected: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#c8c0d0"}),

		EmptyHint: lipgloss.NewStyle().
			Foreground(tertiary),

		FieldLabel: lipgloss.NewStyle().
			Foreground(labelMuted),

		FieldValue: lipgloss.NewStyle().
			Foreground(valueBright),

		InputPrompt: lipgloss.NewStyle().
			Foreground(magenta).
			Bold(true),

		InputText: lipgloss.NewStyle().
			Foreground(valueBright),

		InputPlaceholder: lipgloss.NewStyle().
			Foreground(tertiary),

		FooterKey: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#303030", Dark: "#f2f0ed"}).
			Bold(true),

		FooterLbl: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#888888", Dark: "#6b6a71"}),

		FooterBar: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#c8c8c8", Dark: "#50555d"}),
	}
}

func contentWidth(termWidth int) int {
	w := maxBodyWidth
	if termWidth > 0 {
		inner := termWidth - 6
		if inner > 0 && inner < w {
			w = inner
		}
	}
	if w < 24 {
		w = 24
	}
	return w
}
