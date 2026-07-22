package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// footPart is one footer action: key (bright) + short label (muted).
type footPart struct {
	key        string
	label      string
	shortLabel string
	compactKey string
	keep       bool
}

func (m *Model) renderFooter(parts []footPart) string {
	available := m.footerAvailableWidth()
	if available <= 0 {
		return ""
	}
	var chosen []footPart
	for _, part := range parts {
		candidate := append(append([]footPart(nil), chosen...), part)
		if lipgloss.Width(m.renderFooterParts(candidate, false)) > available {
			continue
		}
		chosen = candidate
	}
	return m.renderFooterParts(chosen, false)
}

// renderPriorityFooter keeps the longest compact priority prefix that fits,
// while always reserving room for the discoverability action marked keep. It
// then expands labels in priority order without dropping a higher-priority
// action to make room for a lower-priority one.
func (m *Model) renderPriorityFooter(parts []footPart) string {
	available := m.footerAvailableWidth()
	if available <= 0 {
		return ""
	}
	if rendered := m.renderFooterParts(parts, false); lipgloss.Width(rendered) <= available {
		return rendered
	}

	kept := -1
	for i := range parts {
		if parts[i].keep {
			kept = i
			break
		}
	}
	selected := make([]bool, len(parts))
	if kept >= 0 {
		selected[kept] = true
		if lipgloss.Width(m.renderSelectedFooterParts(parts, selected, nil)) > available {
			return ""
		}
	}

	for i := range parts {
		if i == kept {
			continue
		}
		selected[i] = true
		if lipgloss.Width(m.renderSelectedFooterParts(parts, selected, nil)) > available {
			selected[i] = false
			break
		}
	}

	levels := make([]int, len(parts))
	for i := range parts {
		if !selected[i] {
			continue
		}
		for level := 1; level <= 2; level++ {
			levels[i] = level
			if lipgloss.Width(m.renderSelectedFooterParts(parts, selected, levels)) > available {
				levels[i] = level - 1
				break
			}
		}
	}
	return m.renderSelectedFooterParts(parts, selected, levels)
}

func (m *Model) renderSelectedFooterParts(parts []footPart, selected []bool, levels []int) string {
	var chosen []footPart
	for i, part := range parts {
		if !selected[i] {
			continue
		}
		level := 0
		if levels != nil {
			level = levels[i]
		}
		if level == 0 {
			part.label = ""
			if part.compactKey != "" {
				part.key = part.compactKey
			}
		} else if level == 1 {
			if part.shortLabel == "" {
				part.label = ""
				if part.compactKey != "" {
					part.key = part.compactKey
				}
			} else {
				part.label = part.shortLabel
			}
		}
		chosen = append(chosen, part)
	}
	return m.renderFooterParts(chosen, false)
}

func (m *Model) renderFooterParts(parts []footPart, compact bool) string {
	var b strings.Builder
	sep := m.styles.FooterBar.Render(" │ ")
	for i, part := range parts {
		if i > 0 {
			b.WriteString(sep)
		}
		keyText := part.key
		if compact && part.compactKey != "" {
			keyText = part.compactKey
		}
		b.WriteString(m.styles.FooterKey.Render(keyText))
		if !compact && part.label != "" {
			b.WriteString(m.styles.FooterLbl.Render(" " + part.label))
		}
	}
	return b.String()
}

func (m *Model) footerAvailableWidth() int {
	width := m.width
	if width <= 0 {
		width = 80
	}
	return max(width-m.styles.Padding.GetHorizontalFrameSize(), 0)
}
