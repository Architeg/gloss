package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// footPart is one footer action: key (bright) + short label (muted).
type footPart struct {
	key   string
	label string
}

func (m *Model) renderFooter(parts []footPart) string {
	if len(parts) == 0 {
		return ""
	}
	available := m.width - m.styles.Padding.GetHorizontalFrameSize()
	if m.width <= 0 {
		available = 80 - m.styles.Padding.GetHorizontalFrameSize()
	}
	if available <= 0 {
		return ""
	}
	var b strings.Builder
	sep := m.styles.FooterBar.Render(" │ ")
	for _, p := range parts {
		var part strings.Builder
		part.WriteString(m.styles.FooterKey.Render(p.key))
		if p.label != "" {
			part.WriteString(m.styles.FooterLbl.Render(" " + p.label))
		}
		candidate := part.String()
		if b.Len() > 0 {
			candidate = sep + candidate
		}
		if lipgloss.Width(b.String()+candidate) > available {
			continue
		}
		b.WriteString(candidate)
	}
	return b.String()
}
