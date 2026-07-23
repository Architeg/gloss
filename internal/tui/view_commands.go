package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

func (m *Model) commandsMainView(width int) string {
	if m.commandHelpOpen {
		return m.commandHelpView(width)
	}
	switch m.cmdPhase {
	case commandsBrowse:
		return m.commandsBrowseView(width)
	case commandsDetail:
		return m.commandsDetailView(width)
	case commandsDeleteConfirm:
		return m.commandsDeleteView(width)
	case commandsEdit:
		return m.commandsEditView(width)
	case commandsBulkTags:
		return m.commandsBulkTagsView(width)
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

const (
	commandMarkerWidth         = 4
	minimumCommandWidth        = 8
	comfortableCommandWidth    = 18
	maximumCommandWidth        = 40
	minimumDescWidth           = 8
	maximumDescWidth           = 80
	compactCommandScreenHeight = 12
)

func browseColumnWidths(total int) (markerW, cmdW, gap, descW int) {
	targetCommand := 18
	if total < 64 {
		targetCommand = 16
	}
	if total < 44 {
		targetCommand = 12
	}
	return responsiveColumnWidths(total, commandMarkerWidth, targetCommand, minimumCommandWidth, minimumDescWidth)
}

func (m *Model) contentAwareBrowseColumnWidths(total int) (markerW, cmdW, gap, descW int) {
	preferredDescription := minimumDescWidth
	for _, row := range m.cmdRows {
		description := strings.TrimSpace(row.Entry.Description)
		if description != "" {
			preferredDescription = max(preferredDescription, runewidth.StringWidth(description))
		}
	}
	if total < 64 {
		markerW, cmdW, gap, remaining := browseColumnWidths(total)
		if remaining == 0 {
			return markerW, cmdW, gap, 0
		}
		return markerW, cmdW, gap, min(remaining, min(preferredDescription, maximumDescWidth))
	}

	preferredCommand := comfortableCommandWidth
	for _, row := range m.cmdRows {
		preferredCommand = max(preferredCommand, runewidth.StringWidth(strings.TrimSpace(row.Entry.Command)))
	}
	// Two cells of tolerance keeps the share near 40% while avoiding an
	// unnecessary wrap for ordinary commands near that boundary.
	commandShareCap := max(minimumCommandWidth, total*2/5+2)
	preferredCommand = min(preferredCommand, commandShareCap)
	preferredCommand = min(preferredCommand, maximumCommandWidth)
	markerW, cmdW, gap, remaining := responsiveColumnWidths(
		total, commandMarkerWidth, preferredCommand, minimumCommandWidth, minimumDescWidth,
	)
	if remaining == 0 {
		return markerW, cmdW, gap, 0
	}
	descW = min(remaining, min(preferredDescription, maximumDescWidth))
	return markerW, cmdW, gap, descW
}

func responsiveColumnWidths(total, marker, target, minimum, minimumTail int) (markerW, leadingW, gap, tailW int) {
	if total <= 0 {
		return 0, 0, 0, 0
	}
	markerW = min(marker, total)
	available := total - markerW
	if available <= 0 {
		return markerW, 0, 0, 0
	}

	gap = min(preferredColumnGap(total), max(available-minimum-minimumTail, 0))
	content := available - gap
	if content <= minimum {
		return markerW, content, 0, 0
	}
	leadingW = min(target, content)
	tailW = content - leadingW
	if tailW < minimumTail && content > minimum {
		leadingW = max(minimum, content-minimumTail)
		tailW = content - leadingW
	}
	if tailW == 0 {
		gap = 0
		leadingW = available
	}
	return markerW, leadingW, gap, tailW
}

func preferredColumnGap(total int) int {
	switch {
	case total >= 64:
		return 4
	case total >= 44:
		return 3
	case total >= 28:
		return 2
	case total > 0:
		return 1
	default:
		return 0
	}
}

func wrapVisual(s string, width int) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		s = "—"
	}
	if width <= 0 {
		return nil
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
	if width <= 0 {
		return ""
	}
	titleText := " " + title + " "
	ruleW := width - lipgloss.Width(titleText) - 8
	if ruleW < 0 {
		return m.styles.Title.Render(truncateScanTail(title, width))
	}
	left := ruleW / 2
	right := ruleW - left

	return m.styles.Divider.Render(strings.Repeat("─", left)) +
		m.styles.Title.Render(titleText) +
		m.styles.Divider.Render(strings.Repeat("─", right))
}

func (m *Model) categoryHeaderBlock(width int, name string) string {
	if width <= 0 {
		return ""
	}
	prefixWidth := lipgloss.Width("› Category: ")
	if width <= prefixWidth {
		head := m.styles.CategoryName.Render(truncateScanTail("› "+name, width))
		rule := m.styles.Divider.Render(strings.Repeat("─", width))
		return head + "\n" + rule
	}
	nameWidth := max(width-prefixWidth, 1)
	head := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.styles.CategoryAccent.Render("› "),
		m.styles.CategoryPrefix.Render("Category: "),
		m.styles.CategoryName.Render(truncateScanTail(name, nameWidth)),
	)

	divLen := lipgloss.Width(head) + 4
	if divLen < 14 && width >= 14 {
		divLen = 14
	}
	if divLen > 24 {
		divLen = 24
	}
	if divLen > width {
		divLen = width
	}

	rule := m.styles.Divider.Render(strings.Repeat("─", divLen))
	return head + "\n" + rule
}

func (m *Model) commandsBrowseView(width int) string {
	var b strings.Builder
	b.WriteString(m.commandsBrowseFixedBlock(width))

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
	listHeight, separate := m.commandViewportLayout(width)
	if separate {
		b.WriteString("\n")
	}
	rows, _ := m.renderCommandViewport(m.listRowWidth(width), listHeight, m.browseOffset)
	b.WriteString(rows)
	return b.String()
}

func (m *Model) commandsBrowseFixedBlock(width int) string {
	var b strings.Builder
	b.WriteString(m.banner(width))
	b.WriteString(m.sectionTitleBlock(width, "Commands"))
	b.WriteString("\n\n")
	b.WriteString(m.filterStatusBlock(width))
	b.WriteString("\n")
	if status := m.commandStatusBlock(width); status != "" {
		b.WriteString(status)
		b.WriteString("\n")
	}
	normal := b.String()
	if len(m.cmdRows) == 0 || m.height <= 0 || m.height > compactCommandScreenHeight || lipgloss.Height(normal) < m.mainContentHeight() {
		return normal
	}

	status := m.commandStatusBlock(width)
	compact := m.sectionTitleBlock(width, "Commands") + "\n"
	if status != "" {
		compact += status + "\n"
	}
	if lipgloss.Height(compact) < m.mainContentHeight() {
		return compact
	}
	if status != "" {
		status += "\n"
		if lipgloss.Height(status) < m.mainContentHeight() {
			return status
		}
	}
	return ""
}

func (m *Model) commandListHeight(width int) int {
	height, _ := m.commandViewportLayout(width)
	return height
}

func (m *Model) commandViewportLayout(width int) (height int, separate bool) {
	height = m.mainContentHeight() - lipgloss.Height(m.commandsBrowseFixedBlock(width))
	if height < 0 {
		return 0, false
	}
	if width <= 0 || height <= 1 || len(m.cmdRows) == 0 {
		return height, false
	}
	firstRow := firstRenderedLines(m.renderCommandEntry(m.listRowWidth(width), 0), 1)
	minimumBlock := m.categoryHeaderBlock(width, m.cmdRows[0].Group) + "\n\n" + firstRow
	if height > lipgloss.Height(minimumBlock) {
		return height - 1, true
	}
	return height, false
}

// renderCommandViewport renders complete entries until the next entry would
// exceed height. The first entry is always represented when height is positive;
// its group heading is omitted only when that is necessary to expose the row.
func (m *Model) renderCommandViewport(width, height, start int) (string, int) {
	if height <= 0 || len(m.cmdRows) == 0 {
		return "", clamp(start, 0, len(m.cmdRows))
	}
	start = clamp(start, 0, len(m.cmdRows)-1)
	var b strings.Builder
	end := start
	for i := start; i < len(m.cmdRows); i++ {
		entry := m.renderCommandEntry(width, i)
		var prefix string
		if i == start {
			prefix = m.categoryHeaderBlock(width, m.cmdRows[i].Group) + "\n\n"
		} else {
			prefix = "\n"
			if m.cmdRows[i].ShowGroup {
				prefix += "\n\n" + m.categoryHeaderBlock(width, m.cmdRows[i].Group) + "\n\n"
			}
		}
		candidate := prefix + entry
		if lipgloss.Height(b.String()+candidate) > height {
			if i != start {
				break
			}
			candidate = entry
			if lipgloss.Height(candidate) > height {
				candidate = firstRenderedLines(candidate, height)
			}
		}
		b.WriteString(candidate)
		end = i + 1
	}
	return b.String(), end
}

func firstRenderedLines(rendered string, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(rendered, "\n")
	if len(lines) <= height {
		return rendered
	}
	return strings.Join(lines[:height], "\n")
}

func (m *Model) renderCommandEntry(width, index int) string {
	if index < 0 || index >= len(m.cmdRows) {
		return ""
	}
	row := m.cmdRows[index]
	markerW, cmdW, gap, descW := m.contentAwareBrowseColumnWidths(width)
	descRaw := strings.TrimSpace(row.Entry.Description)
	cmdLines := wrapVisual(row.Entry.Command, cmdW)
	descLines := wrapVisual(descRaw, descW)
	focused := index == m.browseCursor && m.cmdFocus == commandsFocusList
	multiSelected := m.isMultiSelected(row.Entry.ID)
	rowHeight := maxInt(len(cmdLines), len(descLines))
	if rowHeight == 0 && markerW > 0 {
		rowHeight = 1
	}
	var b strings.Builder

	for lineIdx := 0; lineIdx < rowHeight; lineIdx++ {
		marker := m.commandRowMarker(markerW, focused, focused && lineIdx == 0, multiSelected && lineIdx == 0)

		cmdText := ""
		if lineIdx < len(cmdLines) {
			cmdText = cmdLines[lineIdx]
		}
		descText := ""
		if lineIdx < len(descLines) {
			descText = descLines[lineIdx]
		}

		var cmdCell, descCell string
		if focused {
			cmdCell = m.styles.CmdSelected.Width(cmdW).Render(cmdText)
		} else {
			cmdCell = m.styles.CmdCol.Width(cmdW).Render(cmdText)
		}
		if descW > 0 && focused {
			descCell = m.styles.DescSelected.Width(descW).Render(descText)
		} else if descW > 0 {
			descCell = m.styles.DescCol.Width(descW).Render(descText)
		}

		parts := []string{marker, cmdCell}
		if descW > 0 {
			gapCell := strings.Repeat(" ", gap)
			if focused {
				gapCell = m.styles.FocusedRow.Render(gapCell)
			}
			parts = append(parts, gapCell, descCell)
		}
		line := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
		b.WriteString(line)

		if lineIdx < rowHeight-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m *Model) commandRowMarker(width int, active, focused, selected bool) string {
	if width <= 0 {
		return ""
	}
	focusW := min(2, width)
	selectW := width - focusW
	focus := strings.Repeat(" ", focusW)
	selection := strings.Repeat(" ", selectW)
	if active {
		if focused {
			focus = m.styles.FocusMarker.Width(focusW).Render(truncateScanTail("›", focusW))
		} else {
			focus = m.styles.FocusedRow.Width(focusW).Render("")
		}
		if selected && selectW > 0 {
			selection = m.styles.FocusMarker.Width(selectW).Render(truncateScanTail("✓", selectW))
		} else if selectW > 0 {
			selection = m.styles.FocusedRow.Width(selectW).Render("")
		}
		return focus + selection
	}
	if focused {
		focus = m.styles.SelCaret.Width(focusW).Render(truncateScanTail("›", focusW))
	}
	if selected && selectW > 0 {
		selection = m.styles.CategoryAccent.Width(selectW).Render(truncateScanTail("✓", selectW))
	}
	return focus + selection
}

func (m *Model) commandStatusBlock(width int) string {
	var parts []string
	if m.hasBrowseSelection() {
		parts = append(parts, fmt.Sprintf("Item %d of %d", m.browseCursor+1, len(m.cmdRows)))
	}
	if count := len(m.multiSelected); count > 0 {
		parts = append(parts, fmt.Sprintf("%d selected", count))
	}
	if m.commandStatus.text != "" {
		parts = append(parts, m.commandStatus.text)
	}
	if len(parts) == 0 {
		return ""
	}
	text := truncateScanTail(strings.Join(parts, "  ·  "), width)
	if m.commandStatus.text != "" && m.commandStatus.isError {
		return m.styles.Err.Render(text)
	}
	return m.styles.FieldValue.Render(text)
}

func (m *Model) temporaryCommandStatusBlock(width int) string {
	if m.commandStatus.text == "" {
		return ""
	}
	text := truncateScanTail(m.commandStatus.text, width)
	if m.commandStatus.isError {
		return m.styles.Err.Render(text)
	}
	return m.styles.FieldValue.Render(text)
}

func (m *Model) filterStatusBlock(width int) string {
	innerWidth := width - m.styles.FilterWrap.GetHorizontalFrameSize()
	if innerWidth <= 0 {
		return ""
	}
	labelWidth := min(9, max(innerWidth-2, 1))
	inputWidth := innerWidth - labelWidth - 1
	if inputWidth <= 0 {
		return m.styles.FilterWrap.Width(innerWidth).Render(
			m.styles.FilterLabel.Width(innerWidth).Render(truncateScanTail("Search", innerWidth)),
		)
	}
	m.searchTI.Width = inputWidth
	m.tagTI.Width = inputWidth
	searchRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.styles.FilterLabel.Width(labelWidth).Render(truncateScanTail("Search:", labelWidth)),
		" ",
		m.searchTI.View(),
	)
	tagRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.styles.FilterLabel.Width(labelWidth).Render(truncateScanTail("Tag:", labelWidth)),
		" ",
		m.tagTI.View(),
	)
	inner := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Width(innerWidth).Render(searchRow),
		lipgloss.NewStyle().Width(innerWidth).Render(tagRow),
	)
	return m.styles.FilterWrap.Width(innerWidth).Render(inner)
}

func (m *Model) commandsDetailView(width int) string {
	var b strings.Builder
	b.WriteString(m.banner(width))
	b.WriteString(m.sectionTitleBlock(width, "Command"))
	b.WriteString("\n\n")
	if status := m.temporaryCommandStatusBlock(width); status != "" {
		b.WriteString(status)
		b.WriteString("\n\n")
	}
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

func (m *Model) commandsBulkTagsView(width int) string {
	var b strings.Builder
	b.WriteString(m.banner(width))
	b.WriteString(m.sectionTitleBlock(width, "Bulk tags"))
	b.WriteString("\n\n")
	b.WriteString(m.styles.FieldValue.Width(width).Render(fmt.Sprintf("Updating %d selected commands", len(m.bulkTargetIDs))))
	b.WriteString("\n\n\n")
	b.WriteString(m.styles.FieldLabel.Render("Add tags"))
	b.WriteString("\n")
	b.WriteString(m.bulkTagForm.addTI.View())
	b.WriteString("\n\n\n")
	b.WriteString(m.styles.FieldLabel.Render("Remove tags"))
	b.WriteString("\n")
	b.WriteString(m.bulkTagForm.removeTI.View())
	return b.String()
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
