package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/valeriybagrintsev/gloss/internal/model"
	"github.com/valeriybagrintsev/gloss/internal/storage"
)

// Options carries dependencies created during app bootstrap.
type Options struct {
	Config *model.Config
	Repo   *storage.EntryRepo
}

// Model is the root Bubble Tea model for Gloss.
type Model struct {
	styles Styles
	keys   bindings
	width  int
	height int

	screen     Screen
	homeCursor int

	config *model.Config
	repo   *storage.EntryRepo

	allEntries []model.Entry
	errBanner  string

	cmdPhase     commandsPhase
	cmdFocus     commandsFocus
	cmdRows      []cmdRow
	browseCursor int
	detailEntry  model.Entry

	searchTI textinput.Model
	tagTI    textinput.Model

	form formState

	editFromBrowse            bool
	deleteFromBrowse          bool
	returnToCommandsAfterForm bool

	scanLoading         bool
	scanSources         []string
	scanRows            []model.ScanSuggestion
	scanCursor          int
	scanSkippedExisting int
	scanSkippedPaths    []string
	scanStatus          string
}

// New returns the initial TUI model.
func New(opts Options) tea.Model {
	cw := contentWidth(80)
	search := textinput.New()
	search.Placeholder = "substring in command or description"
	search.Width = max(cw-8, 14)
	search.Blur()

	tag := textinput.New()
	tag.Placeholder = "exact tag"
	tag.Width = max(cw-8, 14)
	tag.Blur()

	m := &Model{
		styles:   newStyles(),
		keys:     newBindings(),
		screen:   ScreenHome,
		config:   opts.Config,
		repo:     opts.Repo,
		searchTI: search,
		tagTI:    tag,
		form:     newFormState(cw),
	}
	m.form.applyTextInputTheme(m.styles)
	return m
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, loadEntriesCmd(m.repo))
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		cw := contentWidth(msg.Width)
		m.form.resizeInputs(cw)
		m.searchTI.Width = max(cw-8, 14)
		m.tagTI.Width = max(cw-8, 14)
		return m, nil

	case entriesMsg:
		if msg.err != nil {
			m.errBanner = msg.err.Error()
			return m, nil
		}
		m.errBanner = ""
		m.allEntries = msg.entries
		m.rebuildBrowse()
		return m, nil

	case saveMsg:
		if msg.err != nil {
			m.errBanner = msg.err.Error()
			return m, nil
		}
		m.errBanner = ""
		return m.afterSave()

	case deleteMsg:
		if msg.err != nil {
			m.errBanner = msg.err.Error()
			if m.deleteFromBrowse {
				m.cmdPhase = commandsBrowse
				m.deleteFromBrowse = false
			} else {
				m.cmdPhase = commandsDetail
			}
			return m, nil
		}
		m.errBanner = ""
		m.deleteFromBrowse = false
		m.cmdPhase = commandsBrowse
		m.deleteReset()
		return m, loadEntriesCmd(m.repo)

	case scanMsg:
		m.scanLoading = false
		if msg.err != nil {
			m.errBanner = msg.err.Error()
			m.scanRows = nil
			m.scanSources = nil
			return m, nil
		}
		m.errBanner = ""
		m.scanSources = append([]string(nil), msg.sources...)
		m.scanRows = append([]model.ScanSuggestion(nil), msg.suggestions...)
		m.scanSkippedExisting = msg.skippedExisting
		m.scanSkippedPaths = append([]string(nil), msg.skippedPaths...)
		m.scanStatus = ""
		if m.scanCursor >= len(m.scanRows) {
			if len(m.scanRows) == 0 {
				m.scanCursor = 0
			} else {
				m.scanCursor = len(m.scanRows) - 1
			}
		}
		return m, nil

	case importScanMsg:
		switch {
		case msg.imported > 0:
			m.errBanner = ""
			m.scanStatus = fmt.Sprintf("Imported %d", msg.imported)
		case msg.err != nil:
			m.errBanner = msg.err.Error()
			m.scanStatus = ""
		default:
			m.errBanner = ""
			m.scanStatus = "Nothing imported (no rows selected)"
		}
		return m, tea.Batch(loadEntriesCmd(m.repo), runScanCmd(m.config, m.repo))
	}

	switch m.screen {
	case ScreenHome:
		return m.updateHome(msg)
	case ScreenCommands:
		return m.updateCommands(msg)
	case ScreenAdd:
		return m.updateAddScreen(msg)
	case ScreenScan:
		return m.updateScan(msg)
	default:
		return m.updatePlaceholder(msg)
	}
}

func (m *Model) deleteReset() {
	m.detailEntry = model.Entry{}
}

func (m *Model) rebuildBrowse() {
	filtered := filterEntries(m.allEntries, m.searchTI.Value(), m.tagTI.Value())
	m.cmdRows = buildCmdRows(filtered)
	if len(m.cmdRows) == 0 {
		m.browseCursor = 0
		return
	}
	if m.browseCursor >= len(m.cmdRows) {
		m.browseCursor = len(m.cmdRows) - 1
	}
}

func (m *Model) afterSave() (tea.Model, tea.Cmd) {
	if m.screen == ScreenAdd {
		if m.returnToCommandsAfterForm {
			m.returnToCommandsAfterForm = false
			m.screen = ScreenCommands
			m.cmdPhase = commandsBrowse
			m.cmdFocus = commandsFocusList
			m.form.prepareAdd()
			m.form.blurAll()
			return m, loadEntriesCmd(m.repo)
		}
		m.screen = ScreenHome
		m.form.prepareAdd()
		m.form.blurAll()
		return m, loadEntriesCmd(m.repo)
	}
	if m.cmdPhase == commandsEdit {
		e, err := m.entryFromForm()
		if err != nil {
			m.errBanner = err.Error()
			return m, nil
		}
		m.detailEntry = e
		m.form.blurAll()
		if m.editFromBrowse {
			m.editFromBrowse = false
			m.cmdPhase = commandsBrowse
		} else {
			m.cmdPhase = commandsDetail
		}
		return m, loadEntriesCmd(m.repo)
	}
	return m, loadEntriesCmd(m.repo)
}

func (m *Model) entryFromForm() (model.Entry, error) {
	cmd := strings.TrimSpace(m.form.cmdTI.Value())
	if cmd == "" {
		return model.Entry{}, errors.New("command is required")
	}
	if m.form.mode == formModeAdd {
		return model.Entry{
			Command:      model.NormalizeCommand(cmd),
			Description:  m.form.descTI.Value(),
			Tags:         model.ParseTagsCSV(m.form.tagsTI.Value()),
			Type:         model.EntryTypeManual,
			Source:       "manual",
			Target:       "",
			ManagedAlias: false,
		}, nil
	}
	e := m.form.base
	e.Command = model.NormalizeCommand(cmd)
	e.Description = m.form.descTI.Value()
	e.Tags = model.ParseTagsCSV(m.form.tagsTI.Value())
	return e, nil
}

func (m *Model) updateHome(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch {
	case m.keys.shouldQuit(km):
		return m, tea.Quit
	case key.Matches(km, m.keys.Up):
		if m.homeCursor > 0 {
			m.homeCursor--
		}
	case key.Matches(km, m.keys.Down):
		if m.homeCursor < len(HomeMenu)-1 {
			m.homeCursor++
		}
	case key.Matches(km, m.keys.Enter):
		m.screen = HomeMenu[m.homeCursor].Screen
		switch m.screen {
		case ScreenAdd:
			m.errBanner = ""
			m.returnToCommandsAfterForm = false
			m.form.prepareAdd()
		case ScreenCommands:
			m.errBanner = ""
			m.cmdPhase = commandsBrowse
			m.cmdFocus = commandsFocusList
			m.browseCursor = 0
			m.searchTI.SetValue("")
			m.tagTI.SetValue("")
			m.rebuildBrowse()
			return m, loadEntriesCmd(m.repo)
		case ScreenScan:
			m.errBanner = ""
			m.scanStatus = ""
			m.scanCursor = 0
			m.scanRows = nil
			m.scanSources = nil
			m.scanSkippedPaths = nil
			m.scanSkippedExisting = 0
			m.scanLoading = true
			return m, runScanCmd(m.config, m.repo)
		}
	}
	return m, nil
}

func (m *Model) updateScan(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch {
	case m.keys.shouldQuit(km):
		return m, tea.Quit
	case m.keys.shouldBack(km):
		m.screen = ScreenHome
		return m, nil
	case strings.EqualFold(km.String(), "r"):
		m.scanLoading = true
		m.scanStatus = ""
		m.scanRows = nil
		m.scanCursor = 0
		return m, runScanCmd(m.config, m.repo)
	case strings.EqualFold(km.String(), "a"):
		for i := range m.scanRows {
			m.scanRows[i].Selected = true
		}
		return m, nil
	case strings.EqualFold(km.String(), "c"):
		for i := range m.scanRows {
			m.scanRows[i].Selected = false
		}
		return m, nil
	case km.Type == tea.KeySpace:
		if len(m.scanRows) > 0 && m.scanCursor < len(m.scanRows) {
			m.scanRows[m.scanCursor].Selected = !m.scanRows[m.scanCursor].Selected
		}
		return m, nil
	case key.Matches(km, m.keys.Enter):
		if len(m.scanRows) == 0 {
			return m, nil
		}
		rows := append([]model.ScanSuggestion(nil), m.scanRows...)
		return m, importScanCmd(m.repo, rows)
	case key.Matches(km, m.keys.Up):
		if m.scanCursor > 0 {
			m.scanCursor--
		}
	case key.Matches(km, m.keys.Down):
		if m.scanCursor < len(m.scanRows)-1 {
			m.scanCursor++
		}
	}
	return m, nil
}

func (m *Model) updatePlaceholder(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.keys.shouldQuit(km) {
		return m, tea.Quit
	}
	if m.keys.shouldBack(km) {
		m.screen = ScreenHome
	}
	return m, nil
}

func (m *Model) updateCommands(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.cmdPhase {
	case commandsEdit:
		return m.updateForm(msg, true)
	case commandsBrowse:
		if m.cmdFocus != commandsFocusList {
			return m.updateCommandsFilters(msg)
		}
		km, ok := msg.(tea.KeyMsg)
		if ok {
			return m.updateBrowseKeys(km)
		}
	case commandsDetail:
		km, ok := msg.(tea.KeyMsg)
		if ok {
			return m.updateDetailKeys(km)
		}
	case commandsDeleteConfirm:
		km, ok := msg.(tea.KeyMsg)
		if ok {
			return m.updateDeleteKeys(km)
		}
	}
	return m, nil
}

func (m *Model) updateCommandsFilters(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && km.Type == tea.KeyEsc {
		m.cmdFocus = commandsFocusList
		m.searchTI.Blur()
		m.tagTI.Blur()
		return m, nil
	}
	var ti *textinput.Model
	switch m.cmdFocus {
	case commandsFocusSearch:
		ti = &m.searchTI
	case commandsFocusTag:
		ti = &m.tagTI
	default:
		return m, nil
	}
	var cmd tea.Cmd
	*ti, cmd = ti.Update(msg)
	m.rebuildBrowse()
	return m, cmd
}

func (m *Model) updateBrowseKeys(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.shouldQuit(km):
		return m, tea.Quit
	case m.keys.shouldBack(km):
		m.screen = ScreenHome
		m.cmdFocus = commandsFocusList
		m.searchTI.Blur()
		m.tagTI.Blur()
		return m, nil
	case km.String() == "/":
		m.cmdFocus = commandsFocusSearch
		m.searchTI.Focus()
		return m, textinput.Blink
	case strings.EqualFold(km.String(), "f"):
		m.cmdFocus = commandsFocusTag
		m.tagTI.Focus()
		return m, textinput.Blink
	case strings.EqualFold(km.String(), "a"):
		m.returnToCommandsAfterForm = true
		m.screen = ScreenAdd
		m.errBanner = ""
		m.form.prepareAdd()
		return m, textinput.Blink
	case strings.EqualFold(km.String(), "e"):
		if len(m.cmdRows) == 0 {
			return m, nil
		}
		m.editFromBrowse = true
		m.detailEntry = m.cmdRows[m.browseCursor].Entry
		m.cmdPhase = commandsEdit
		m.form.prepareEdit(m.detailEntry)
		return m, textinput.Blink
	case strings.EqualFold(km.String(), "d"):
		if len(m.cmdRows) == 0 {
			return m, nil
		}
		m.deleteFromBrowse = true
		m.detailEntry = m.cmdRows[m.browseCursor].Entry
		m.cmdPhase = commandsDeleteConfirm
		return m, nil
	case key.Matches(km, m.keys.Up):
		if m.browseCursor > 0 {
			m.browseCursor--
		}
	case key.Matches(km, m.keys.Down):
		if m.browseCursor < len(m.cmdRows)-1 {
			m.browseCursor++
		}
	case key.Matches(km, m.keys.Enter):
		if len(m.cmdRows) == 0 {
			return m, nil
		}
		m.detailEntry = m.cmdRows[m.browseCursor].Entry
		m.cmdPhase = commandsDetail
	}
	return m, nil
}

func (m *Model) updateDetailKeys(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.shouldQuit(km):
		return m, tea.Quit
	case m.keys.shouldBack(km):
		m.cmdPhase = commandsBrowse
		return m, nil
	case strings.EqualFold(km.String(), "e"):
		m.editFromBrowse = false
		m.cmdPhase = commandsEdit
		m.form.prepareEdit(m.detailEntry)
		return m, textinput.Blink
	case strings.EqualFold(km.String(), "d"):
		m.deleteFromBrowse = false
		m.cmdPhase = commandsDeleteConfirm
	}
	return m, nil
}

func (m *Model) updateDeleteKeys(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.keys.shouldQuit(km):
		return m, tea.Quit
	case km.Type == tea.KeyEsc || strings.EqualFold(km.String(), "n"):
		if m.deleteFromBrowse {
			m.cmdPhase = commandsBrowse
			m.deleteFromBrowse = false
		} else {
			m.cmdPhase = commandsDetail
		}
	case strings.EqualFold(km.String(), "y"):
		return m, deleteEntryCmd(m.repo, m.detailEntry.Command)
	}
	return m, nil
}

func (m *Model) updateAddScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.updateForm(msg, false)
}

func (m *Model) updateForm(msg tea.Msg, fromCommands bool) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case m.keys.shouldQuit(km):
			return m, tea.Quit
		case km.Type == tea.KeyEsc:
			return m.cancelForm(fromCommands)
		case km.Type == tea.KeyCtrlS:
			return m.submitForm()
		case km.Type == tea.KeyTab:
			m.cycleFormFocus(1)
			return m, nil
		case km.Type == tea.KeyShiftTab:
			m.cycleFormFocus(-1)
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.form.focus {
	case formFocusCommand:
		m.form.cmdTI, cmd = m.form.cmdTI.Update(msg)
	case formFocusDesc:
		m.form.descTI, cmd = m.form.descTI.Update(msg)
	case formFocusTags:
		m.form.tagsTI, cmd = m.form.tagsTI.Update(msg)
	}
	return m, cmd
}

func (m *Model) cycleFormFocus(delta int) {
	next := int(m.form.focus) + delta
	for next < 0 {
		next += 3
	}
	next %= 3
	m.form.focusField(formFocus(next))
}

func (m *Model) cancelForm(fromCommands bool) (tea.Model, tea.Cmd) {
	m.errBanner = ""
	m.form.blurAll()
	if fromCommands {
		if m.editFromBrowse {
			m.editFromBrowse = false
			m.cmdPhase = commandsBrowse
		} else {
			m.cmdPhase = commandsDetail
		}
		return m, nil
	}
	if m.returnToCommandsAfterForm {
		m.returnToCommandsAfterForm = false
		m.screen = ScreenCommands
		m.cmdPhase = commandsBrowse
		m.cmdFocus = commandsFocusList
		return m, loadEntriesCmd(m.repo)
	}
	m.screen = ScreenHome
	return m, nil
}

func (m *Model) submitForm() (tea.Model, tea.Cmd) {
	e, err := m.entryFromForm()
	if err != nil {
		m.errBanner = err.Error()
		return m, nil
	}
	if m.form.mode == formModeAdd {
		return m, saveCreateCmd(m.repo, e)
	}
	return m, saveUpdateCmd(m.repo, e)
}
