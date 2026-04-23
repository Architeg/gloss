package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
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
	gap = 2
	cmdW = 20
	if total < 54 {
		cmdW = 16
	}
	if total < 38 {
		cmdW = 12
	}
	descW = total - gutter - cmdW - gap
	if descW < 10 {
		descW = 10
	}
	return cmdW, gap, descW
}

func truncateVisual(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxW {
		return s
	}
	w := 0
	var b strings.Builder
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if rw < 0 {
			rw = 0
		}
		if w+rw > maxW-1 {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + "…"
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

	divLen := lipgloss.Width(head) + 10
	divLen = clamp(divLen, 24, 44)

	rule := m.styles.Divider.Render(strings.Repeat("─", divLen))
	return head + "\n" + rule
}

func (m *Model) commandsBrowseView(width int) string {
	var b strings.Builder

	b.WriteString(m.filterStatusBlock(width))
	b.WriteString("\n\n")

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
		descText := truncateVisual(descRaw, descW)
		if descText == "" {
			descText = truncateVisual("—", descW)
		}
		cmdPart := truncateVisual(row.Entry.Command, cmdW)

		selected := i == m.browseCursor && m.cmdFocus == commandsFocusList
		gutter := lipgloss.NewStyle().Width(2).Align(lipgloss.Left).Render("")
		if selected {
			gutter = lipgloss.NewStyle().
				Width(2).
				Align(lipgloss.Left).
				Render(m.styles.SelCaret.Render("›"))
		}

		var cmdCell, descCell string
		if selected {
			cmdCell = m.styles.CmdSelected.Width(cmdW).Render(cmdPart)
			descCell = m.styles.DescSelected.Width(descW).Render(descText)
		} else {
			cmdCell = m.styles.CmdCol.Width(cmdW).Render(cmdPart)
			descCell = m.styles.DescCol.Width(descW).Render(descText)
		}

		line := lipgloss.JoinHorizontal(
			lipgloss.Top,
			gutter,
			cmdCell,
			strings.Repeat(" ", gap),
			descCell,
		)
		b.WriteString(lipgloss.NewStyle().Width(width).Render(line))

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
		lipgloss.NewStyle().MarginTop(1).Width(width).Render(tagRow),
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
