package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type updatePreferenceSource uint8

const (
	updatePreferenceOnboarding updatePreferenceSource = iota
	updatePreferenceSettings
)

type updatePreferenceSavedMsg struct {
	enabled bool
	source  updatePreferenceSource
	err     error
}

func (m *Model) beginUpdatePreferenceSave(enabled bool, source updatePreferenceSource) tea.Cmd {
	if m.updatePreferenceSaving {
		return nil
	}
	m.updatePreferenceSaving = true
	m.updatePreferencePending = enabled
	m.updatePreferenceSource = source
	if source == updatePreferenceOnboarding {
		m.updatePreferenceError = ""
	} else {
		m.settingsStatus = ""
		m.settingsStatusError = false
	}
	writer := m.saveUpdatePreference
	return func() tea.Msg {
		if writer == nil {
			return updatePreferenceSavedMsg{
				enabled: enabled,
				source:  source,
				err:     errors.New("config preference writer is unavailable"),
			}
		}
		return updatePreferenceSavedMsg{
			enabled: enabled,
			source:  source,
			err:     writer(enabled),
		}
	}
}

func (m *Model) handleUpdatePreferenceSaved(msg updatePreferenceSavedMsg) (tea.Model, tea.Cmd) {
	if !m.updatePreferenceSaving || msg.source != m.updatePreferenceSource ||
		msg.enabled != m.updatePreferencePending {
		return m, nil
	}
	m.updatePreferenceSaving = false
	if msg.err != nil {
		text := "Could not save update preference: " + msg.err.Error()
		if msg.source == updatePreferenceOnboarding {
			m.updatePreferenceError = text
		} else {
			m.settingsStatus = text
			m.settingsStatusError = true
		}
		return m, nil
	}
	if m.config == nil {
		return m, nil
	}
	m.config.CheckForUpdates = msg.enabled
	m.config.CheckForUpdatesSet = true
	if msg.source == updatePreferenceOnboarding {
		m.updatePromptVisible = false
		m.updatePreferenceError = ""
	} else {
		state := "disabled"
		if msg.enabled {
			state = "enabled"
		}
		m.settingsStatus = "Automatic update checks " + state + "."
		m.settingsStatusError = false
	}
	if !msg.enabled {
		return m, nil
	}
	return m, m.automaticUpdateCommand()
}

func (m *Model) updateUpdatePreferencePrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch {
	case m.keys.shouldQuit(km):
		return m, tea.Quit
	case km.Type == tea.KeyEsc:
		if !m.updatePreferenceSaving {
			m.updatePromptVisible = false
			m.updatePreferenceError = ""
		}
		return m, nil
	case key.Matches(km, m.keys.Left), key.Matches(km, m.keys.Up):
		if !m.updatePreferenceSaving {
			m.updatePromptCursor = 0
		}
		return m, nil
	case key.Matches(km, m.keys.Right), key.Matches(km, m.keys.Down), km.Type == tea.KeyTab:
		if !m.updatePreferenceSaving {
			m.updatePromptCursor = 1
		}
		return m, nil
	case km.Type == tea.KeyShiftTab:
		if !m.updatePreferenceSaving {
			m.updatePromptCursor = 0
		}
		return m, nil
	case key.Matches(km, m.keys.Enter):
		return m, m.beginUpdatePreferenceSave(m.updatePromptCursor == 0, updatePreferenceOnboarding)
	}
	return m, nil
}

func (m *Model) updatePreferenceView(width int) string {
	var b strings.Builder
	b.WriteString(m.renderHomeBanner(width))
	b.WriteString("\n\n")
	b.WriteString(m.sectionTitleBlock(width, "Automatic updates"))
	b.WriteString("\n\n")
	b.WriteString(m.styles.Title.Width(width).Render("Check for Gloss updates automatically?"))
	b.WriteString("\n\n")
	b.WriteString(m.styles.FieldValue.Width(width).Render(
		fmt.Sprintf(
			"Gloss will check GitHub at most once every %s.\nUpdates are never installed automatically.",
			m.updateIntervalLabel(),
		),
	))
	if m.updatePreferenceError != "" {
		b.WriteString("\n\n")
		b.WriteString(m.styles.Err.Width(width).Render(m.updatePreferenceError))
	}
	b.WriteString("\n\n")

	buttons := []string{"[ Enable ]", "[ Not now ]"}
	rendered := make([]string, len(buttons))
	for i, button := range buttons {
		style := m.styles.Item
		if i == m.updatePromptCursor {
			style = m.styles.Selected
		}
		rendered[i] = style.Render(button)
	}
	if width >= 28 {
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, rendered[0], "  ", rendered[1]))
	} else {
		b.WriteString(lipgloss.JoinVertical(lipgloss.Left, rendered[0], rendered[1]))
	}
	if m.updatePreferenceSaving {
		b.WriteString("\n\n")
		b.WriteString(m.styles.EmptyHint.Render("Saving…"))
	}
	return strings.TrimRight(b.String(), "\n")
}

func updateSettingLabel(enabled bool) string {
	if enabled {
		return "On"
	}
	return "Off"
}

func (m *Model) updatePreferenceStatus(width int) string {
	if m.settingsStatus == "" {
		return ""
	}
	if m.settingsStatusError {
		return m.styles.Err.Width(width).Render(m.settingsStatus)
	}
	return m.styles.BannerTagline.Width(width).Render(m.settingsStatus)
}

func (m *Model) updatePreferenceDescription(width int) string {
	return m.styles.EmptyHint.Width(width).Render(
		fmt.Sprintf("Checks GitHub at most once every %s. Never installs automatically.", m.updateIntervalLabel()),
	)
}

func (m *Model) updateIntervalLabel() string {
	interval := "the configured interval"
	if m.config != nil && m.config.UpdateCheckInterval.Duration() > 0 {
		duration := m.config.UpdateCheckInterval.Duration()
		if duration%time.Hour == 0 {
			hours := duration / time.Hour
			unit := "hours"
			if hours == 1 {
				unit = "hour"
			}
			interval = fmt.Sprintf("%d %s", hours, unit)
		} else {
			interval = duration.String()
		}
	}
	return interval
}
