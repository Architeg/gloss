package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/valeriybagrintsev/gloss/internal/alias"
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
		gutter := lipgloss.NewStyle().Width(2).Align(lipgloss.Left).Render("")
		if i == m.aliasMenuCursor {
			gutter = lipgloss.NewStyle().Width(2).Align(lipgloss.Left).Render(m.styles.SelCaret.Render("›"))
		}
		labelSt := m.styles.Item.Width(22).Align(lipgloss.Left)
		descSt := m.styles.HomeDesc
		if i == m.aliasMenuCursor {
			labelSt = m.styles.Selected.Width(22).Align(lipgloss.Left)
			descSt = m.styles.HomeSelDesc
		}
		line := lipgloss.JoinHorizontal(lipgloss.Top, gutter, labelSt.Render(item.title), "  ", descSt.Render(item.desc))
		b.WriteString(lipgloss.NewStyle().Width(width).Render(line))
		if i < len(aliasMenuHome)-1 {
			b.WriteString("\n\n")
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
	b.WriteString("\n\n")

	rows := m.managedAliasRows()
	if len(rows) == 0 {
		b.WriteString(m.styles.EmptyHint.Width(width).Render("No managed aliases yet. Add one from the menu."))
		return b.String()
	}
	for i, e := range rows {
		gutter := lipgloss.NewStyle().Width(2).Align(lipgloss.Left).Render("")
		if i == m.aliasViewCursor {
			gutter = lipgloss.NewStyle().Width(2).Align(lipgloss.Left).Render(m.styles.SelCaret.Render("›"))
		}
		cmdSt := m.styles.CmdCol
		if i == m.aliasViewCursor {
			cmdSt = m.styles.CmdSelected
		}
		tw := width - 28
		if tw < 16 {
			tw = 16
		}
		line := lipgloss.JoinHorizontal(lipgloss.Top, gutter, cmdSt.Render(e.Command), "  ", m.styles.DescCol.Render(truncateScanTail(e.Target, tw)))
		b.WriteString(lipgloss.NewStyle().Width(width).Render(line))
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

	block := alias.RenderManagedBlock(m.allEntries)
	b.WriteString(m.styles.FieldValue.Width(width).Render(block))
	return b.String()
}
