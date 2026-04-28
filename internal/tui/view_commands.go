package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) commandsMainView(width int) string {
	switch m.cmdPhase {
	case commandsBrowse:
		return m.commandsBrowseView(width)
	case commandsDetail:
		return m.commandsDetailView(width)
	case commandsDeleteConfirm:
		return m.commandsDeleteView(width)
	case commandsEdit:
		return m.commandsEditView(width)
	default:
		return ""
	}
}

func (m *Model) banner(width int) string {
	if m.errBanner == "" {
		return ""
	}
	return m.styles.Err.Width(width).Render(m.errBanner) + "\n\n"
}

func browseColumnWidths(total int) (cmdW, gap, descW int) {
	const gutter = 2

	gap = 3
	cmdW = 18

	if total < 64 {
		cmdW = 16
		gap = 2
	}
	if total < 44 {
		cmdW = 12
		gap = 2
	}

	descW = total - gutter - cmdW - gap
	if descW < 12 {
		descW = 12
	}

	return cmdW, gap, descW
}

func wrapVisual(s string, width int) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		s = "—"
	}
	if width <= 0 {
		return []string{s}
	}

	wrapped := lipgloss.NewStyle().Width(width).Render(s)
	lines := strings.Split(wrapped, "\n")

	if len(lines) == 0 {
		return []string{"—"}
	}
	return lines
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func (m *Model) sectionTitleBlock(width int, title string) string {
	titleText := " " + title + " "
	ruleW := width - lipgloss.Width(titleText) - 8
	if ruleW < 8 {
		ruleW = 8
	}
	left := ruleW / 2
	right := ruleW - left

	return m.styles.Divider.Render(strings.Repeat("─", left)) +
		m.styles.Title.Render(titleText) +
		m.styles.Divider.Render(strings.Repeat("─", right))
}

func (m *Model) categoryHeaderBlock(width int, name string) string {
	head := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.styles.CategoryAccent.Render("› "),
		m.styles.CategoryPrefix.Render("Category: "),
		m.styles.CategoryName.Render(name),
	)

	divLen := lipgloss.Width(head) + 4
	if divLen < 14 {
		divLen = 14
	}
	if divLen > 24 {
		divLen = 24
	}

	rule := m.styles.Divider.Render(strings.Repeat("─", divLen))
	return head + "\n" + rule
}

func (m *Model) commandsBrowseView(width int) string {
	var b strings.Builder

	b.WriteString(m.banner(width))
	b.WriteString(m.sectionTitleBlock(width, "Commands"))
	b.WriteString("\n\n")

	b.WriteString(m.filterStatusBlock(width))
	b.WriteString("\n")

	cmdW, gap, descW := browseColumnWidths(width)

	if len(m.allEntries) == 0 {
		msg := "No commands saved yet.\n\nPress A to add one, or choose Add from the home menu."
		b.WriteString(m.styles.EmptyHint.Width(width).Render(msg))
		return b.String()
	}
	if len(m.cmdRows) == 0 {
		msg := "No matches for this search or tag filter.\n\nAdjust Search or Tag, then Esc to return to the list."
		b.WriteString(m.styles.EmptyHint.Width(width).Render(msg))
		return b.String()
	}

	for i, row := range m.cmdRows {
		if row.ShowGroup {
			if i > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(m.categoryHeaderBlock(width, row.Group))
			b.WriteString("\n\n")
		}

		descRaw := strings.TrimSpace(row.Entry.Description)
		cmdLines := wrapVisual(row.Entry.Command, cmdW)
		descLines := wrapVisual(descRaw, descW)

		selected := i == m.browseCursor && m.cmdFocus == commandsFocusList
		rowHeight := maxInt(len(cmdLines), len(descLines))

		for lineIdx := 0; lineIdx < rowHeight; lineIdx++ {
			gutter := "  "
			if selected && lineIdx == 0 {
				gutter = m.styles.SelCaret.Render("› ")
			}

			cmdText := ""
			if lineIdx < len(cmdLines) {
				cmdText = cmdLines[lineIdx]
			}

			descText := ""
			if lineIdx < len(descLines) {
				descText = descLines[lineIdx]
			}

			var cmdCell, descCell string
			if selected {
				cmdCell = m.styles.CmdSelected.Width(cmdW).Render(cmdText)
				descCell = m.styles.DescSelected.Width(descW).Render(descText)
			} else {
				cmdCell = m.styles.CmdCol.Width(cmdW).Render(cmdText)
				descCell = m.styles.DescCol.Width(descW).Render(descText)
			}

			line := lipgloss.JoinHorizontal(
				lipgloss.Top,
				gutter,
				cmdCell,
				strings.Repeat(" ", gap),
				descCell,
			)

			b.WriteString(line)

			if lineIdx < rowHeight-1 {
				b.WriteString("\n")
			}
		}

		if i < len(m.cmdRows)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m *Model) filterStatusBlock(width int) string {
	searchRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.styles.FilterLabel.Render("Search:"),
		" ",
		m.searchTI.View(),
	)
	tagRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.styles.FilterLabel.Render("Tag:"),
		" ",
		m.tagTI.View(),
	)
	inner := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Width(width).Render(searchRow),
		lipgloss.NewStyle().Width(width).Render(tagRow),
	)
	return m.styles.FilterWrap.Width(width).Render(inner)
}

func (m *Model) commandsDetailView(width int) string {
	var b strings.Builder
	b.WriteString(m.banner(width))
	b.WriteString(m.sectionTitleBlock(width, "Command"))
	b.WriteString("\n\n")
	e := m.detailEntry
	b.WriteString(m.styles.FieldValue.Width(width).Render(e.Command))
	b.WriteString("\n\n\n")

	writeField := func(label, value string) {
		b.WriteString(m.styles.FieldLabel.Render(label))
		b.WriteString("\n")
		b.WriteString(m.styles.FieldValue.Width(width).Render(value))
		b.WriteString("\n\n")
	}

	desc := strings.TrimSpace(e.Description)
	if desc == "" {
		desc = "—"
	}
	writeField("Description", desc)

	tagStr := "—"
	if len(e.Tags) > 0 {
		tagStr = strings.Join(e.Tags, ", ")
	}
	writeField("Tags", tagStr)

	writeField("Type / Source", fmt.Sprintf("%s / %s", e.Type, e.Source))
	if strings.TrimSpace(e.Target) != "" {
		writeField("Target", e.Target)
	}
	return b.String()
}

func (m *Model) commandsDeleteView(width int) string {
	var b strings.Builder
	b.WriteString(m.banner(width))
	b.WriteString(m.sectionTitleBlock(width, "Delete entry?"))
	b.WriteString("\n\n")
	b.WriteString(m.styles.FieldLabel.Render("This cannot be undone."))
	b.WriteString("\n\n")
	b.WriteString(m.styles.FieldValue.Width(width).Render(
		fmt.Sprintf("Remove %q", m.detailEntry.Command),
	))
	return b.String()
}

func (m *Model) commandsEditView(width int) string {
	return m.addFormViewWithTitle(width, "Edit entry")
}

func (m *Model) addFormView(width int) string {
	return m.addFormViewWithTitle(width, "Add entry")
}

func (m *Model) addFormViewWithTitle(width int, title string) string {
	var b strings.Builder
	b.WriteString(m.banner(width))
	b.WriteString(m.sectionTitleBlock(width, title))
	b.WriteString("\n\n")

	b.WriteString(m.styles.FieldLabel.Render("Command"))
	b.WriteString("\n")
	b.WriteString(m.form.cmdTI.View())
	b.WriteString("\n\n\n")

	b.WriteString(m.styles.FieldLabel.Render("Description"))
	b.WriteString("\n")
	b.WriteString(m.form.descTI.View())
	b.WriteString("\n\n\n")

	b.WriteString(m.styles.FieldLabel.Render("Tags"))
	b.WriteString("\n")
	b.WriteString(m.form.tagsTI.View())

	return b.String()
}
