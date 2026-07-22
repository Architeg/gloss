package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// footPart is one footer action: key (bright) + short label (muted).
type footPart struct {
	key        string
	label      string
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

func (m *Model) renderAdaptiveFooter(core, extra []footPart) string {
	available := m.footerAvailableWidth()
	if available <= 0 {
		return ""
	}
	all := append(append([]footPart(nil), core...), extra...)
	if rendered := m.renderFooterParts(all, false); lipgloss.Width(rendered) <= available {
		return rendered
	}
	if rendered := m.fitFooterParts(core, extra, false, available); rendered != "" {
		return rendered
	}
	if rendered := m.fitFooterParts(core, extra, true, available); rendered != "" {
		return rendered
	}

	var kept *footPart
	for i := range core {
		if core[i].keep {
			part := core[i]
			kept = &part
			break
		}
	}
	if kept == nil {
		return ""
	}
	keptRendered := m.renderFooterParts([]footPart{*kept}, true)
	if lipgloss.Width(keptRendered) > available {
		return ""
	}
	var chosen []footPart
	for _, part := range core {
		if part.keep {
			continue
		}
		candidate := append(append([]footPart(nil), chosen...), part, *kept)
		if lipgloss.Width(m.renderFooterParts(candidate, true)) <= available {
			chosen = append(chosen, part)
		}
	}
	chosen = append(chosen, *kept)
	return m.renderFooterParts(chosen, true)
}

func (m *Model) fitFooterParts(core, extra []footPart, compact bool, available int) string {
	if rendered := m.renderFooterParts(core, compact); lipgloss.Width(rendered) > available {
		return ""
	}
	chosen := append([]footPart(nil), core...)
	for _, part := range extra {
		candidate := append(append([]footPart(nil), chosen...), part)
		if lipgloss.Width(m.renderFooterParts(candidate, compact)) <= available {
			chosen = candidate
		}
	}
	return m.renderFooterParts(chosen, compact)
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
