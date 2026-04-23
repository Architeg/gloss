package tui

import (
	"path/filepath"
	"strings"
)

func (m *Model) settingsView(width int) string {
	var b strings.Builder
	b.WriteString(m.banner(width))
	b.WriteString(m.sectionTitleBlock(width, "Settings"))
	b.WriteString("\n\n")

	cfg := m.config
	if cfg == nil {
		b.WriteString(m.styles.EmptyHint.Width(width).Render("No configuration loaded."))
		return b.String()
	}

	b.WriteString(m.styles.FieldLabel.Render("Shell file"))
	b.WriteString("\n")
	b.WriteString(m.styles.FieldValue.Width(width).Render(cfg.ShellFile))
	b.WriteString("\n\n")

	b.WriteString(m.styles.FieldLabel.Render("Storage path"))
	b.WriteString("\n")
	b.WriteString(m.styles.FieldValue.Width(width).Render(cfg.StoragePath))
	b.WriteString("\n\n")

	b.WriteString(m.styles.FieldLabel.Render("Scan paths"))
	b.WriteString("\n")
	if len(cfg.ScanPaths) == 0 {
		b.WriteString(m.styles.DescCol.Width(width).Render("—"))
	} else {
		for _, p := range cfg.ScanPaths {
			b.WriteString(m.styles.FieldValue.Width(width).Render(p))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n\n\n")
	b.WriteString(m.styles.FieldLabel.Render("Gloss config file"))
	b.WriteString("\n")
	b.WriteString(m.styles.FieldValue.Width(width).Render(filepath.Join(cfg.StoragePath, "config.yaml")))
	b.WriteString("\n\n")
	b.WriteString(m.styles.EmptyHint.Width(width).Render("Note: You can use config.yaml to edit paths manually if needed."))

	return strings.TrimRight(b.String(), "\n")
}
