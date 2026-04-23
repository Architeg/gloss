package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/valeriybagrintsev/gloss/internal/model"
)

func scanRowWidths(width int) (gutter, markW, cmdW, typW, gap, detailW int) {
	gutter = 2
	gap = 2
	markW = 4
	cmdW = 18
	typW = 10
	if width < 50 {
		cmdW = 14
		typW = 8
	}
	detailW = width - gutter - markW - cmdW - typW - gap*2
	if detailW < 8 {
		detailW = 8
	}
	return gutter, markW, cmdW, typW, gap, detailW
}

func truncateScanTail(s string, maxW int) string {
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

func (m *Model) scanView(width int) string {
	var b strings.Builder
	b.WriteString(m.banner(width))
	b.WriteString(m.sectionTitleBlock(width, "Scan"))
	b.WriteString("\n\n")

	b.WriteString(m.styles.FieldLabel.Render("Sources"))
	b.WriteString("\n")
	if m.scanLoading && len(m.scanSources) == 0 {
		b.WriteString(m.styles.FieldValue.Width(width).Render("Scanning…"))
		b.WriteString("\n")
	} else if len(m.scanSources) == 0 {
		b.WriteString(m.styles.EmptyHint.Width(width).Render("No paths resolved."))
		b.WriteString("\n")
	} else {
		for _, p := range m.scanSources {
			b.WriteString(m.styles.FieldValue.Width(width).Render(p))
			b.WriteString("\n")
		}
	}

	if len(m.scanSkippedPaths) > 0 {
		b.WriteString("\n")
		b.WriteString(m.styles.FieldLabel.Render("Skipped (missing or unreadable)"))
		b.WriteString("\n")
		for _, p := range m.scanSkippedPaths {
			b.WriteString(m.styles.DescCol.Width(width).Render(p))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if m.scanLoading {
		b.WriteString(m.styles.DescCol.Render("Status: "))
		b.WriteString(m.styles.FieldValue.Render("Scanning…"))
	} else {
		line := fmt.Sprintf(
			"%d importable — %d already in glossary",
			len(m.scanRows),
			m.scanSkippedExisting,
		)
		b.WriteString(m.styles.DescCol.Width(width).Render(line))
	}
	if m.scanStatus != "" {
		b.WriteString("\n")
		b.WriteString(m.styles.FieldValue.Width(width).Render(m.scanStatus))
	}
	b.WriteString("\n\n")

	if !m.scanLoading && len(m.scanRows) > 0 {
		gutterW, markW, cmdW, typW, gap, detailW := scanRowWidths(width)
		for i, row := range m.scanRows {
			gutter := lipgloss.NewStyle().Width(gutterW).Align(lipgloss.Left).Render("")
			if i == m.scanCursor {
				gutter = lipgloss.NewStyle().Width(gutterW).Align(lipgloss.Left).Render(m.styles.SelCaret.Render("›"))
			}
			mark := "[ ]"
			if row.Selected {
				mark = "[x]"
			}
			if i == m.scanCursor {
				mark = m.styles.Selected.Render(mark)
			} else {
				mark = m.styles.Item.Render(mark)
			}
			markCell := lipgloss.NewStyle().Width(markW).Render(mark)

			cmdSt := m.styles.CmdCol
			typSt := m.styles.DescCol
			if i == m.scanCursor {
				cmdSt = m.styles.CmdSelected
				typSt = m.styles.DescSelected
			}
			cmdCell := cmdSt.Width(cmdW).Render(truncateScanTail(row.Command, cmdW))
			typCell := typSt.Width(typW).Render(truncateScanTail(row.Type, typW))

			detail := scanRowDetail(row)
			detail = truncateScanTail(detail, detailW)
			detailCell := m.styles.DescCol.Width(detailW).Render(detail)
			if i == m.scanCursor {
				detailCell = m.styles.DescSelected.Width(detailW).Render(detail)
			}

			line := lipgloss.JoinHorizontal(
				lipgloss.Top,
				gutter,
				markCell,
				cmdCell,
				strings.Repeat(" ", gap),
				typCell,
				strings.Repeat(" ", gap),
				detailCell,
			)
			b.WriteString(lipgloss.NewStyle().Width(width).Render(line))
			if i < len(m.scanRows)-1 {
				b.WriteString("\n")
			}
		}
	} else if !m.scanLoading && len(m.scanSources) > 0 {
		b.WriteString(m.styles.EmptyHint.Width(width).Render("No new items to import. Press r to rescan."))
	}

	return b.String()
}

func scanRowDetail(row model.ScanSuggestion) string {
	switch row.Type {
	case model.EntryTypeAlias:
		if strings.TrimSpace(row.Target) != "" {
			return row.Target
		}
	case model.EntryTypeScript:
		return row.Source
	default:
		if strings.TrimSpace(row.Target) != "" {
			return row.Target
		}
	}
	return row.Source
}
