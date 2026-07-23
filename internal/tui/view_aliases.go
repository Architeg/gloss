package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Architeg/gloss/internal/alias"
)

func (m *Model) aliasesMainView(width int) string {
	switch m.aliasPhase {
	case aliasPhaseAdd:
		return m.aliasAddView(width)
	case aliasPhaseView:
		return m.aliasListView(width)
	case aliasPhaseDeleteConfirm:
		return m.aliasDeleteConfirmView(width)
	case aliasPhasePreview:
		return m.aliasPreviewView(width)
	default:
		return m.aliasMenuView(width)
	}
}

func (m *Model) aliasMenuView(width int) string {
	var b strings.Builder
	b.WriteString(m.banner(width))
	b.WriteString(m.sectionTitleBlock(width, "Aliases"))
	b.WriteString("\n\n")

	shellPath := m.resolvedShellPath()
	b.WriteString(m.styles.FieldLabel.Render("Shell file"))
	b.WriteString("\n")
	b.WriteString(m.styles.FieldValue.Width(width).Render(shellPath))
	b.WriteString("\n\n")

	for i, item := range aliasMenuHome {
		focused := i == m.aliasMenuCursor
		gutter := lipgloss.NewStyle().Width(2).Align(lipgloss.Left).Render("")
		if focused {
			gutter = m.styles.FocusMarker.Width(2).Align(lipgloss.Left).Render("›")
		}
		labelSt := m.styles.Item.Width(22).Align(lipgloss.Left)
		descSt := m.styles.HomeDesc
		gap := "  "
		if focused {
			labelSt = m.styles.CmdSelected.Width(22).Align(lipgloss.Left)
			descSt = m.styles.DescSelected
			gap = m.styles.FocusedRow.Render(gap)
		}
		line := lipgloss.JoinHorizontal(lipgloss.Top, gutter, labelSt.Render(item.title), gap, descSt.Render(item.desc))
		b.WriteString(lipgloss.NewStyle().Width(width).Render(line))
		if i < len(aliasMenuHome)-1 {
			b.WriteString("\n")
		}
	}

	if m.aliasStatus != "" {
		b.WriteString("\n\n")
		b.WriteString(m.styles.FieldValue.Width(width).Render(m.aliasStatus))
	}
	return b.String()
}

func (m *Model) resolvedShellPath() string {
	p, err := alias.ResolveShellPath(m.config)
	if err != nil {
		return "(error resolving path)"
	}
	return p
}

func (m *Model) aliasAddView(width int) string {
	var b strings.Builder
	b.WriteString(m.banner(width))
	b.WriteString(m.sectionTitleBlock(width, "Add managed alias"))
	b.WriteString("\n\n")

	b.WriteString(m.styles.FieldLabel.Render("Alias name"))
	b.WriteString("\n")
	b.WriteString(m.aliasForm.nameTI.View())
	b.WriteString("\n\n\n")
	b.WriteString(m.styles.FieldLabel.Render("Expands to"))
	b.WriteString("\n")
	b.WriteString(m.aliasForm.targetTI.View())
	b.WriteString("\n\n\n")
	b.WriteString(m.styles.FieldLabel.Render("Description (optional; defaults to expansion)"))
	b.WriteString("\n")
	b.WriteString(m.aliasForm.descTI.View())
	b.WriteString("\n\n\n")
	b.WriteString(m.styles.FieldLabel.Render("Tags (optional)"))
	b.WriteString("\n")
	b.WriteString(m.aliasForm.tagsTI.View())
	return b.String()
}

func (m *Model) aliasListView(width int) string {
	var b strings.Builder
	b.WriteString(m.banner(width))
	b.WriteString(m.sectionTitleBlock(width, "Managed aliases"))
	b.WriteString("\n")

	rows := m.managedAliasRows()
	if len(rows) == 0 {
		b.WriteString(m.styles.EmptyHint.Width(width).Render("No managed aliases yet. Add one from the menu."))
		return b.String()
	}
	rowWidth := m.listRowWidth(width)
	preferredAliasWidth := 22
	if rowWidth >= 64 {
		for _, entry := range rows {
			preferredAliasWidth = max(preferredAliasWidth, lipgloss.Width(strings.TrimSpace(entry.Command)))
		}
		preferredAliasWidth = min(preferredAliasWidth, min(40, rowWidth*2/5+2))
	}
	for i, e := range rows {
		focused := i == m.aliasViewCursor
		markerW, commandW, gap, targetW := responsiveColumnWidths(rowWidth, 2, preferredAliasWidth, 8, 8)
		gutter := lipgloss.NewStyle().Width(markerW).Align(lipgloss.Left).Render("")
		if focused {
			gutter = m.styles.FocusMarker.Width(markerW).Align(lipgloss.Left).Render(truncateScanTail("›", markerW))
		}
		cmdSt := m.styles.CmdCol
		targetSt := m.styles.DescCol
		if focused {
			cmdSt = m.styles.CmdSelected
			targetSt = m.styles.DescSelected
		}
		parts := []string{
			gutter,
			cmdSt.Width(commandW).Render(truncateScanTail(e.Command, commandW)),
		}
		if targetW > 0 {
			gapCell := strings.Repeat(" ", gap)
			if focused {
				gapCell = m.styles.FocusedRow.Render(gapCell)
			}
			parts = append(parts, gapCell, targetSt.Width(targetW).Render(truncateScanTail(e.Target, targetW)))
		}
		line := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
		b.WriteString(line)
		if i < len(rows)-1 {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

func (m *Model) aliasDeleteConfirmView(width int) string {
	var b strings.Builder
	b.WriteString(m.banner(width))
	b.WriteString(m.sectionTitleBlock(width, "Delete managed alias?"))
	b.WriteString("\n\n")
	b.WriteString(m.styles.FieldLabel.Render("This cannot be undone."))
	b.WriteString("\n\n")
	b.WriteString(m.styles.FieldValue.Width(width).Render(
		fmt.Sprintf("Remove %q from Gloss", m.aliasDeletePending),
	))
	return b.String()
}

func (m *Model) aliasPreviewView(width int) string {
	var b strings.Builder
	b.WriteString(m.banner(width))
	b.WriteString(m.sectionTitleBlock(width, "Preview sync block"))
	b.WriteString("\n\n")

	block, err := alias.RenderManagedBlock(m.allEntries)
	if err != nil {
		b.WriteString(m.styles.EmptyHint.Width(width).Render("Unable to render managed aliases: " + err.Error()))
		return b.String()
	}
	b.WriteString(m.styles.FieldValue.Width(width).Render(block))
	return b.String()
}
