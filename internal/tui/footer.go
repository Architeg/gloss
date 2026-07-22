package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// footPart is one footer action: key (bright) + short label (muted).
type footPart struct {
	key   string
	label string
	keep  bool
}

func (m *Model) renderFooter(parts []footPart) string {
	available := m.footerAvailableWidth()
	if available <= 0 {
		return ""
	}
	var chosen []footPart
	for _, part := range parts {
		candidate := append(append([]footPart(nil), chosen...), part)
		if lipgloss.Width(m.renderFooterParts(candidate)) > available {
			break
		}
		chosen = candidate
	}
	return m.renderFooterParts(chosen)
}

// renderPriorityFooter reserves the complete discoverability hint, then keeps
// the longest priority prefix whose complete key-and-description hints fit.
// Only the reserved hint may fall back to its bare key at emergency widths.
func (m *Model) renderPriorityFooter(parts []footPart) string {
	available := m.footerAvailableWidth()
	if available <= 0 {
		return ""
	}
	if rendered := m.renderFooterParts(parts); lipgloss.Width(rendered) <= available {
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
		if lipgloss.Width(m.renderSelectedFooterParts(parts, selected)) > available {
			fallback := m.styles.FooterKey.Render(parts[kept].key)
			if lipgloss.Width(fallback) <= available {
				return fallback
			}
			return ""
		}
	}

	for i := range parts {
		if i == kept {
			continue
		}
		selected[i] = true
		if lipgloss.Width(m.renderSelectedFooterParts(parts, selected)) > available {
			selected[i] = false
			break
		}
	}
	return m.renderSelectedFooterParts(parts, selected)
}

func (m *Model) renderSelectedFooterParts(parts []footPart, selected []bool) string {
	var chosen []footPart
	for i, part := range parts {
		if !selected[i] {
			continue
		}
		chosen = append(chosen, part)
	}
	return m.renderFooterParts(chosen)
}

func (m *Model) renderFooterParts(parts []footPart) string {
	var b strings.Builder
	sep := m.styles.FooterBar.Render(" │ ")
	for i, part := range parts {
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(m.styles.FooterKey.Render(part.key))
		if part.label != "" {
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
