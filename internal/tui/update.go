package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Architeg/gloss/internal/update"
)

const defaultAutomaticUpdateTimeout = 10 * time.Second

type automaticUpdateMsg struct {
	result   update.CheckResult
	err      error
	skipped  bool
	homebrew bool
}

func (m *Model) automaticUpdateCommand() tea.Cmd {
	if m.config == nil || !m.config.CheckForUpdates || m.updateChecker == nil || m.updateCheckStarted {
		return nil
	}
	interval := m.config.UpdateCheckInterval.Duration()
	if interval <= 0 {
		return nil
	}
	m.updateCheckStarted = true
	timeout := m.updateTimeout
	if timeout <= 0 {
		timeout = defaultAutomaticUpdateTimeout
	}
	return automaticUpdateCheckCmd(
		m.updateChecker,
		m.updateState,
		m.updateVersion,
		interval,
		timeout,
		m.inspectUpdateLayout,
	)
}

func automaticUpdateCheckCmd(
	checker updateChecker,
	state update.StateStore,
	current string,
	interval time.Duration,
	timeout time.Duration,
	inspect func() (update.Layout, error),
) tea.Cmd {
	return func() tea.Msg {
		if !state.Due(interval) {
			return automaticUpdateMsg{skipped: true}
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result, err := checker.Check(ctx, current)
		if err != nil {
			return automaticUpdateMsg{err: err}
		}
		if err := state.MarkCompleted(result.LatestVersion); err != nil {
			// State persistence must not turn a successful check into startup noise.
		}
		msg := automaticUpdateMsg{result: result}
		if result.UpdateAvailable && inspect != nil {
			layout, layoutErr := inspect()
			msg.homebrew = update.IsHomebrew(layoutErr) || layout.Kind == update.LayoutHomebrew
		}
		return msg
	}
}
