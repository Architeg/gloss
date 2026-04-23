package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/valeriybagrintsev/gloss/internal/alias"
	"github.com/valeriybagrintsev/gloss/internal/model"
)

type aliasPhase int

const (
	aliasPhaseMenu aliasPhase = iota
	aliasPhaseAdd
	aliasPhaseView
	aliasPhasePreview
	aliasPhaseDeleteConfirm
)

var aliasMenuHome = []struct {
	title string
	desc  string
}{
	{"Add Alias", "Store in Gloss; sync separately to shell"},
	{"View Managed Aliases", "Entries with managed alias flag"},
	{"Preview Sync Block", "Exact block written on sync"},
	{"Sync to shell file", "Backup if file exists, then write block"},
}

func (m *Model) updateAliases(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.aliasPhase {
	case aliasPhaseAdd:
		return m.updateAliasAdd(msg)
	case aliasPhaseView:
		return m.updateAliasViewList(msg)
	case aliasPhaseDeleteConfirm:
		return m.updateAliasDeleteConfirm(msg)
	case aliasPhasePreview:
		return m.updateAliasPreview(msg)
	default:
		return m.updateAliasMenu(msg)
	}
}

func (m *Model) updateAliasMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch {
	case m.keys.shouldQuit(km):
		return m, tea.Quit
	case m.keys.shouldBack(km):
		m.aliasDeletePending = ""
		m.screen = ScreenHome
		return m, nil
	case key.Matches(km, m.keys.Up):
		if m.aliasMenuCursor > 0 {
			m.aliasMenuCursor--
		}
	case key.Matches(km, m.keys.Down):
		if m.aliasMenuCursor < len(aliasMenuHome)-1 {
			m.aliasMenuCursor++
		}
	case key.Matches(km, m.keys.Enter):
		switch m.aliasMenuCursor {
		case 0:
			m.aliasPhase = aliasPhaseAdd
			m.errBanner = ""
			m.aliasForm.prepare()
			return m, textinput.Blink
		case 1:
			m.aliasPhase = aliasPhaseView
			m.aliasViewCursor = 0
			m.errBanner = ""
			return m, loadEntriesCmd(m.repo)
		case 2:
			m.aliasPhase = aliasPhasePreview
			m.errBanner = ""
			return m, loadEntriesCmd(m.repo)
		case 3:
			m.errBanner = ""
			m.aliasStatus = ""
			return m, syncAliasesCmd(m.config, m.repo)
		}
	}
	return m, nil
}

func (m *Model) updateAliasAdd(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case m.keys.shouldQuit(km):
			return m, tea.Quit
		case km.Type == tea.KeyEsc:
			m.errBanner = ""
			m.aliasForm.blurAll()
			m.aliasPhase = aliasPhaseMenu
			return m, nil
		case km.Type == tea.KeyCtrlS:
			return m.submitAliasForm()
		case km.Type == tea.KeyTab:
			m.aliasForm.cycleFocus(1)
			return m, nil
		case km.Type == tea.KeyShiftTab:
			m.aliasForm.cycleFocus(-1)
			return m, nil
		}
	}
	var cmd tea.Cmd
	switch m.aliasForm.focus {
	case aliasFieldName:
		m.aliasForm.nameTI, cmd = m.aliasForm.nameTI.Update(msg)
	case aliasFieldTarget:
		m.aliasForm.targetTI, cmd = m.aliasForm.targetTI.Update(msg)
	case aliasFieldDesc:
		m.aliasForm.descTI, cmd = m.aliasForm.descTI.Update(msg)
	case aliasFieldTags:
		m.aliasForm.tagsTI, cmd = m.aliasForm.tagsTI.Update(msg)
	}
	return m, cmd
}

func (m *Model) submitAliasForm() (tea.Model, tea.Cmd) {
	e, err := m.aliasForm.toEntry()
	if err != nil {
		m.errBanner = err.Error()
		return m, nil
	}
	return m, saveCreateCmd(m.repo, e)
}

func (m *Model) updateAliasViewList(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	rows := m.managedAliasRows()
	switch {
	case m.keys.shouldQuit(km):
		return m, tea.Quit
	case m.keys.shouldBack(km):
		m.aliasDeletePending = ""
		m.aliasPhase = aliasPhaseMenu
		return m, nil
	case strings.EqualFold(km.String(), "d"):
		if len(rows) == 0 || m.aliasViewCursor < 0 || m.aliasViewCursor >= len(rows) {
			return m, nil
		}
		m.errBanner = ""
		m.aliasDeletePending = rows[m.aliasViewCursor].Command
		m.aliasPhase = aliasPhaseDeleteConfirm
		return m, nil
	case key.Matches(km, m.keys.Up):
		if m.aliasViewCursor > 0 {
			m.aliasViewCursor--
		}
	case key.Matches(km, m.keys.Down):
		if m.aliasViewCursor < len(rows)-1 {
			m.aliasViewCursor++
		}
	}
	return m, nil
}

func (m *Model) updateAliasDeleteConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch {
	case m.keys.shouldQuit(km):
		return m, tea.Quit
	case km.Type == tea.KeyEsc || strings.EqualFold(km.String(), "n"):
		m.aliasDeletePending = ""
		m.aliasPhase = aliasPhaseView
	case strings.EqualFold(km.String(), "y"):
		if m.aliasDeletePending == "" {
			return m, nil
		}
		return m, deleteEntryCmd(m.repo, m.aliasDeletePending)
	}
	return m, nil
}

func (m *Model) updateAliasPreview(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch {
	case m.keys.shouldQuit(km):
		return m, tea.Quit
	case m.keys.shouldBack(km):
		m.aliasPhase = aliasPhaseMenu
		return m, nil
	}
	return m, nil
}

func (m *Model) managedAliasRows() []model.Entry {
	return alias.ManagedAliases(m.allEntries)
}
