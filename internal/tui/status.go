package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const commandStatusDuration = 2500 * time.Millisecond

type commandStatus struct {
	text       string
	isError    bool
	generation uint64
}

type commandStatusExpiredMsg struct {
	generation uint64
}

func (m *Model) setCommandStatus(text string, isError bool) tea.Cmd {
	m.commandStatus.generation++
	m.commandStatus.text = text
	m.commandStatus.isError = isError
	m.ensureBrowseVisible(true)
	generation := m.commandStatus.generation
	return tea.Tick(commandStatusDuration, func(time.Time) tea.Msg {
		return commandStatusExpiredMsg{generation: generation}
	})
}

func (m *Model) expireCommandStatus(msg commandStatusExpiredMsg) {
	if msg.generation != m.commandStatus.generation {
		return
	}
	m.commandStatus.text = ""
	m.commandStatus.isError = false
	m.ensureBrowseVisible(true)
}
