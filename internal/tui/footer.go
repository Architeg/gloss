package tui

import "strings"

// footPart is one footer action: key (bright) + short label (muted).
type footPart struct {
	key   string
	label string
}

func (m *Model) renderFooter(parts []footPart) string {
	if len(parts) == 0 {
		return ""
	}
	var b strings.Builder
	sep := m.styles.FooterBar.Render(" │ ")
	for i, p := range parts {
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(m.styles.FooterKey.Render(p.key))
		if p.label != "" {
			b.WriteString(m.styles.FooterLbl.Render(" " + p.label))
		}
	}
	return b.String()
}
