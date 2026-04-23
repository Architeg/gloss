package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const homeLabelW = 14

// View implements tea.Model.
func (m *Model) View() string {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}

	footerStr := m.footerContent()
	footerLine := lipgloss.NewStyle().MarginTop(1).Render(footerStr)
	footerBlock := m.styles.Padding.Render(footerLine)
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

func (m *Model) mainBlock(width int) string {
	switch m.screen {
	case ScreenHome:
		return m.homeView(width)
	case ScreenCommands:
		return m.commandsMainView(width)
	case ScreenAdd:
		return m.addFormView(width)
	case ScreenScan:
		return m.scanView(width)
	case ScreenAliases:
		return m.aliasesMainView(width)
	case ScreenSettings:
		return m.settingsView(width)
	default:
		return m.placeholderView(width)
	}
}

func (m *Model) homeView(width int) string {
	var b strings.Builder
	tw := width
	if tw <= 0 {
		tw = contentWidth(m.width)
	}
	b.WriteString(m.renderHomeBanner(tw))
	b.WriteString("\n\n\n")

	for i, item := range HomeMenu {
		desc := placeholderBlurb(item.Screen)
		gutter := lipgloss.NewStyle().Width(2).Align(lipgloss.Left).Render("")
		menuSel := m.homeSection == homeSectionMenu && i == m.homeCursor
		if menuSel {
			gutter = lipgloss.NewStyle().Width(2).Align(lipgloss.Left).Render(m.styles.SelCaret.Render("›"))
		}

		labelSt := m.styles.Item.Width(homeLabelW).Align(lipgloss.Left)
		descSt := m.styles.HomeDesc
		if menuSel {
			labelSt = m.styles.Selected.Width(homeLabelW).Align(lipgloss.Left)
			descSt = m.styles.HomeSelDesc
		}

		line := lipgloss.JoinHorizontal(
			lipgloss.Top,
			gutter,
			labelSt.Render(item.Title),
			"  ",
			descSt.Render(desc),
		)
		b.WriteString(lipgloss.NewStyle().Width(width).Render(line))
		if i < len(HomeMenu)-1 {
			b.WriteString("\n")
		}
	}

	// Extra vertical gap before the secondary support row (menu unchanged above).
	b.WriteString("\n\n\n\n")
	for j, link := range HomeSupportLinks {
		chip := fmt.Sprintf("[%s %s]", link.Icon, link.Label)
		var cell string
		if m.homeSection == homeSectionSupport && j == m.supportCursor {
			cell = m.styles.Selected.Render(chip)
		} else {
			cell = m.styles.HomeDesc.Render(chip)
		}
		b.WriteString(cell)
		if j < len(HomeSupportLinks)-1 {
			b.WriteString("  ")
		}
	}

	return b.String()
}

func (m *Model) placeholderView(width int) string {
	title := screenTitle(m.screen)
	blurb := placeholderBlurb(m.screen)
	var b strings.Builder
	b.WriteString(m.styles.Title.Width(width).Render(title))
	b.WriteString("\n\n")
	b.WriteString(m.styles.FieldLabel.Render("About"))
	b.WriteString("\n")
	b.WriteString(m.styles.FieldValue.Width(width).Render(blurb))
	return b.String()
}

func (m *Model) footerContent() string {
	switch m.screen {
	case ScreenHome:
		if m.homeSection == homeSectionSupport {
			return m.renderFooter([]footPart{
				{key: "←→", label: "Choose"},
				{key: "↑", label: "Menu"},
				{key: "Enter", label: "Open"},
				{key: "Q", label: "Quit"},
			})
		}
		return m.renderFooter([]footPart{
			{key: "↑↓", label: "Move"},
			{key: "Enter", label: "Open"},
			{key: "Q", label: "Quit"},
		})
	case ScreenAdd:
		return m.renderFooter([]footPart{
			{key: "Esc", label: "Cancel"},
			{key: "Tab", label: "Field"},
			{key: "^S", label: "Save"},
			{key: "Q", label: "Quit"},
		})
	case ScreenScan:
		return m.renderFooter([]footPart{
			{key: "↑↓", label: "Move"},
			{key: "Space", label: "Toggle"},
			{key: "Enter", label: "Import"},
			{key: "R", label: "Rescan"},
			{key: "A", label: "All"},
			{key: "C", label: "Clear"},
			{key: "Esc", label: "Back"},
			{key: "Q", label: "Quit"},
		})
	case ScreenAliases:
		switch m.aliasPhase {
		case aliasPhaseAdd:
			return m.renderFooter([]footPart{
				{key: "Esc", label: "Cancel"},
				{key: "Tab", label: "Field"},
				{key: "^S", label: "Save"},
				{key: "Q", label: "Quit"},
			})
		case aliasPhaseView:
			return m.renderFooter([]footPart{
				{key: "↑↓", label: "Move"},
				{key: "D", label: "Delete"},
				{key: "Esc", label: "Back"},
				{key: "Q", label: "Quit"},
			})
		case aliasPhaseDeleteConfirm:
			return m.renderFooter([]footPart{
				{key: "Y", label: "Confirm"},
				{key: "N", label: "Cancel"},
				{key: "Q", label: "Quit"},
			})
		case aliasPhasePreview:
			return m.renderFooter([]footPart{
				{key: "Esc", label: "Back"},
				{key: "Q", label: "Quit"},
			})
		default:
			return m.renderFooter([]footPart{
				{key: "↑↓", label: "Move"},
				{key: "Enter", label: "Open"},
				{key: "Esc", label: "Back"},
				{key: "Q", label: "Quit"},
			})
		}
	case ScreenCommands:
		switch m.cmdPhase {
		case commandsBrowse:
			if m.cmdFocus == commandsFocusSearch {
				return m.renderFooter([]footPart{
					{key: "Esc", label: "List"},
					{key: "Q", label: "Quit"},
				})
			}
			if m.cmdFocus == commandsFocusTag {
				return m.renderFooter([]footPart{
					{key: "Esc", label: "List"},
					{key: "Q", label: "Quit"},
				})
			}
			return m.renderFooter([]footPart{
				{key: "/", label: "Search"},
				{key: "F", label: "Filter"},
				{key: "E", label: "Edit"},
				{key: "D", label: "Delete"},
				{key: "A", label: "Add"},
				{key: "↑↓", label: "Move"},
				{key: "Enter", label: "Open"},
				{key: "Esc", label: "Back"},
				{key: "Q", label: "Quit"},
			})
		case commandsDetail:
			return m.renderFooter([]footPart{
				{key: "Esc", label: "Back"},
				{key: "E", label: "Edit"},
				{key: "D", label: "Delete"},
				{key: "Q", label: "Quit"},
			})
		case commandsDeleteConfirm:
			return m.renderFooter([]footPart{
				{key: "Y", label: "Confirm"},
				{key: "N", label: "Cancel"},
				{key: "Q", label: "Quit"},
			})
		case commandsEdit:
			return m.renderFooter([]footPart{
				{key: "Esc", label: "Cancel"},
				{key: "Tab", label: "Field"},
				{key: "^S", label: "Save"},
				{key: "Q", label: "Quit"},
			})
		default:
			return m.renderFooter([]footPart{{key: "Esc", label: "Back"}})
		}
	default:
		return m.renderFooter([]footPart{
			{key: "Esc", label: "Back"},
			{key: "Q", label: "Quit"},
		})
	}
}
