package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Architeg/gloss/internal/alias"
	"github.com/Architeg/gloss/internal/buildinfo"
	"github.com/Architeg/gloss/internal/clipboard"
	"github.com/Architeg/gloss/internal/model"
	"github.com/Architeg/gloss/internal/storage"
	"github.com/Architeg/gloss/internal/update"
)

type updateChecker interface {
	Check(context.Context, string) (update.CheckResult, error)
}

// Options carries dependencies created during app bootstrap.
type Options struct {
	Config              *model.Config
	Repo                *storage.EntryRepo
	Clipboard           clipboard.Writer
	UpdateChecker       updateChecker
	Version             string
	UpdateState         update.StateStore
	InspectUpdateLayout func() (update.Layout, error)
	UpdateTimeout       time.Duration
}

// Model is the root Bubble Tea model for Gloss.
type Model struct {
	styles Styles
	keys   bindings
	width  int
	height int

	screen        Screen
	homeCursor    int
	homeSection   homeSection
	supportCursor int

	config *model.Config
	repo   *storage.EntryRepo
	clip   clipboard.Writer

	allEntries []model.Entry
	errBanner  string

	updateChecker       updateChecker
	version             string
	updateState         update.StateStore
	inspectUpdateLayout func() (update.Layout, error)
	updateTimeout       time.Duration
	updateCheckStarted  bool
	updateCheckFinished bool
	updateNotice        string

	cmdPhase          commandsPhase
	cmdFocus          commandsFocus
	cmdRows           []cmdRow
	browseCursor      int
	browseOffset      int
	selectedID        int64
	restoreID         int64
	detailEntry       model.Entry
	multiSelected     map[int64]struct{}
	bulkTagForm       bulkTagFormState
	bulkTargetIDs     []int64
	commandStatus     commandStatus
	commandHelpOpen   bool
	commandHelpOffset int

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

	aliasPhase         aliasPhase
	aliasMenuCursor    int
	aliasForm          aliasFormState
	aliasViewCursor    int
	aliasDeletePending string
	aliasStatus        string
}

// New returns the initial TUI model.
func New(opts Options) tea.Model {
	version := opts.Version
	if version == "" {
		version = buildinfo.Version()
	}
	cw := contentWidth(80)
	search := textinput.New()
	search.Placeholder = "substring in command or description"
	search.Width = inputWidth(cw - 10)
	search.Blur()

	tag := textinput.New()
	tag.Placeholder = "exact tag"
	tag.Width = inputWidth(cw - 10)
	tag.Blur()

	m := &Model{
		styles:              newStyles(),
		keys:                newBindings(),
		screen:              ScreenHome,
		config:              opts.Config,
		repo:                opts.Repo,
		clip:                opts.Clipboard,
		searchTI:            search,
		tagTI:               tag,
		form:                newFormState(cw),
		aliasForm:           newAliasFormState(cw),
		bulkTagForm:         newBulkTagFormState(cw),
		multiSelected:       make(map[int64]struct{}),
		updateChecker:       opts.UpdateChecker,
		version:             version,
		updateState:         opts.UpdateState,
		inspectUpdateLayout: opts.InspectUpdateLayout,
		updateTimeout:       opts.UpdateTimeout,
	}
	if m.clip == nil {
		m.clip = clipboard.System{}
	}
	m.form.applyTextInputTheme(m.styles)
	m.aliasForm.applyTheme(m.styles)
	m.bulkTagForm.applyTheme(m.styles)
	return m
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, loadEntriesCmd(m.repo), m.automaticUpdateCommand())
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		cw := contentWidth(msg.Width)
		m.form.resizeInputs(cw)
		m.searchTI.Width = inputWidth(cw - 10)
		m.tagTI.Width = inputWidth(cw - 10)
		m.aliasForm.resize(cw)
		m.bulkTagForm.resize(cw)
		m.clampCommandHelpOffset()
		m.ensureBrowseVisible(true)
		return m, nil

	case commandStatusExpiredMsg:
		m.expireCommandStatus(msg)
		return m, nil

	case automaticUpdateMsg:
		m.updateCheckFinished = true
		if msg.err != nil || msg.skipped || !msg.result.UpdateAvailable {
			return m, nil
		}
		if msg.homebrew {
			m.updateNotice = "Gloss " + msg.result.LatestVersion + " is available — " + update.HomebrewUpgradeCommand
		} else {
			m.updateNotice = "Gloss " + msg.result.LatestVersion + " is available — run gloss update --install"
		}
		return m, nil

	case entriesMsg:
		if msg.err != nil {
			m.errBanner = msg.err.Error()
			return m, nil
		}
		m.errBanner = ""
		m.allEntries = msg.entries
		m.pruneMultiSelection()
		m.rebuildBrowse()
		if m.screen == ScreenAliases && m.aliasPhase == aliasPhaseView {
			rows := m.managedAliasRows()
			if len(rows) == 0 {
				m.aliasViewCursor = 0
			} else if m.aliasViewCursor >= len(rows) {
				m.aliasViewCursor = len(rows) - 1
			}
		}
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
			if m.screen == ScreenAliases && m.aliasPhase == aliasPhaseDeleteConfirm {
				m.aliasPhase = aliasPhaseView
				m.aliasDeletePending = ""
				return m, nil
			}
			if m.deleteFromBrowse {
				m.cmdPhase = commandsBrowse
				m.deleteFromBrowse = false
			} else {
				m.cmdPhase = commandsDetail
			}
			return m, nil
		}
		m.errBanner = ""
		if m.screen == ScreenAliases && m.aliasPhase == aliasPhaseDeleteConfirm {
			m.aliasPhase = aliasPhaseView
			m.aliasDeletePending = ""
			return m, loadEntriesCmd(m.repo)
		}
		m.deleteFromBrowse = false
		m.cmdPhase = commandsBrowse
		delete(m.multiSelected, m.detailEntry.ID)
		m.deleteReset()
		return m, tea.Batch(m.setCommandStatus("Deleted", false), loadEntriesCmd(m.repo))

	case copyCommandMsg:
		if msg.err != nil {
			return m, m.setCommandStatus("Copy failed: "+msg.err.Error(), true)
		}
		return m, m.setCommandStatus("Copied", false)

	case bulkTagsMsg:
		m.cmdPhase = commandsBrowse
		m.bulkTagForm.blurAll()
		m.bulkTargetIDs = nil
		m.errBanner = ""
		if msg.err != nil {
			return m, m.setCommandStatus("Tags update failed: "+msg.err.Error(), true)
		}
		statusCmd := m.setCommandStatus("Tags updated", false)
		return m, tea.Batch(statusCmd, loadEntriesCmd(m.repo))

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

	case syncAliasesMsg:
		if msg.err != nil {
			m.errBanner = msg.err.Error()
			m.aliasStatus = ""
			return m, nil
		}
		m.errBanner = ""
		if msg.noop {
			m.aliasStatus = "Shell file already up to date."
			return m, nil
		}
		if msg.backupPath != "" {
			m.aliasStatus = fmt.Sprintf("Synced. Backup: %s", msg.backupPath)
		} else {
			m.aliasStatus = fmt.Sprintf("Wrote managed block to %s", msg.shellPath)
		}
		return m, loadEntriesCmd(m.repo)

	case openURLMsg:
		if msg.err != nil {
			m.errBanner = msg.err.Error()
		} else {
			m.errBanner = ""
		}
		return m, nil
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
	case ScreenAliases:
		return m.updateAliases(msg)
	case ScreenSettings:
		return m.updateSettings(msg)
	default:
		return m.updatePlaceholder(msg)
	}
}

func (m *Model) deleteReset() {
	m.detailEntry = model.Entry{}
}

func (m *Model) rebuildBrowse() {
	previousCursor := m.browseCursor
	filtered := filterEntries(m.allEntries, m.searchTI.Value(), m.tagTI.Value())
	m.cmdRows = buildCmdRows(filtered)
	m.reconcileBrowse(previousCursor)
}

func (m *Model) afterSave() (tea.Model, tea.Cmd) {
	if m.screen == ScreenAliases && m.aliasPhase == aliasPhaseAdd {
		m.aliasPhase = aliasPhaseMenu
		m.aliasForm.blurAll()
		m.aliasForm.prepare()
		m.aliasStatus = "Saved managed alias (run Sync to update shell file)"
		return m, loadEntriesCmd(m.repo)
	}
	if m.screen == ScreenAdd {
		if m.returnToCommandsAfterForm {
			m.returnToCommandsAfterForm = false
			m.screen = ScreenCommands
			m.cmdPhase = commandsBrowse
			m.cmdFocus = commandsFocusList
			m.form.prepareAdd()
			m.form.blurAll()
			return m, tea.Batch(m.setCommandStatus("Saved", false), loadEntriesCmd(m.repo))
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
		m.selectedID = e.ID
		m.restoreID = 0
		m.form.blurAll()
		if m.editFromBrowse {
			m.editFromBrowse = false
			m.cmdPhase = commandsBrowse
		} else {
			m.cmdPhase = commandsDetail
		}
		return m, tea.Batch(m.setCommandStatus("Saved", false), loadEntriesCmd(m.repo))
	}
	return m, loadEntriesCmd(m.repo)
}

func (m *Model) entryFromForm() (model.Entry, error) {
	rawCommand := m.form.cmdTI.Value()
	cmd := strings.TrimSpace(rawCommand)
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
	if e.ManagedAlias && e.Type == model.EntryTypeAlias {
		if err := alias.ValidateAliasName(rawCommand); err != nil {
			return model.Entry{}, err
		}
	}
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
	case key.Matches(km, m.keys.Back):
		if m.homeSection == homeSectionSupport {
			m.homeSection = homeSectionMenu
			m.homeCursor = len(HomeMenu) - 1
			return m, nil
		}
	case key.Matches(km, m.keys.Up):
		if m.homeSection == homeSectionSupport {
			m.homeSection = homeSectionMenu
			m.homeCursor = len(HomeMenu) - 1
			return m, nil
		}
		if m.homeCursor > 0 {
			m.homeCursor--
		}
		return m, nil
	case key.Matches(km, m.keys.Down):
		if m.homeSection == homeSectionSupport {
			return m, nil
		}
		if m.homeCursor < len(HomeMenu)-1 {
			m.homeCursor++
			return m, nil
		}
		m.homeSection = homeSectionSupport
		m.supportCursor = 0
		return m, nil
	case key.Matches(km, m.keys.Left):
		if m.homeSection == homeSectionSupport && m.supportCursor > 0 {
			m.supportCursor--
		}
		return m, nil
	case key.Matches(km, m.keys.Right):
		if m.homeSection == homeSectionSupport && m.supportCursor < len(HomeSupportLinks)-1 {
			m.supportCursor++
		}
		return m, nil
	case key.Matches(km, m.keys.Enter):
		if m.homeSection == homeSectionSupport {
			if m.supportCursor >= 0 && m.supportCursor < len(HomeSupportLinks) {
				return m, openURLCmd(HomeSupportLinks[m.supportCursor].URL)
			}
			return m, nil
		}
		item := HomeMenu[m.homeCursor]
		if item.OpenURL != "" {
			return m, openURLCmd(item.OpenURL)
		}
		m.screen = item.Screen
		switch m.screen {
		case ScreenAdd:
			m.errBanner = ""
			m.returnToCommandsAfterForm = false
			m.form.prepareAdd()
		case ScreenCommands:
			m.errBanner = ""
			m.cmdPhase = commandsBrowse
			m.cmdFocus = commandsFocusList
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
		case ScreenAliases:
			m.errBanner = ""
			m.aliasStatus = ""
			m.aliasPhase = aliasPhaseMenu
			m.aliasMenuCursor = 0
			m.aliasDeletePending = ""
			m.aliasForm.prepare()
			m.aliasForm.blurAll()
			return m, loadEntriesCmd(m.repo)
		case ScreenSettings:
			m.errBanner = ""
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

func (m *Model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.updatePlaceholder(msg)
}

func (m *Model) updateCommands(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.commandHelpOpen {
		return m.updateCommandHelp(msg)
	}
	if km, ok := msg.(tea.KeyMsg); ok && m.cmdPhase == commandsBrowse && km.String() == "?" {
		m.commandHelpOpen = true
		m.commandHelpOffset = 0
		return m, nil
	}
	switch m.cmdPhase {
	case commandsEdit:
		return m.updateForm(msg, true)
	case commandsBulkTags:
		return m.updateBulkTagForm(msg)
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
	case km.Type == tea.KeySpace:
		if !m.toggleFocusedSelection() {
			return m, m.setCommandStatus("No selectable command", true)
		}
		m.ensureBrowseVisible(true)
		return m, nil
	case km.Type == tea.KeyCtrlA:
		if !m.toggleVisibleSelection() {
			return m, m.setCommandStatus("No selectable commands", true)
		}
		m.ensureBrowseVisible(true)
		return m, nil
	case strings.EqualFold(km.String(), "t"):
		ids := m.selectedEntryIDs()
		if len(ids) == 0 {
			return m, m.setCommandStatus("Select one or more commands first", true)
		}
		m.bulkTargetIDs = ids
		m.bulkTagForm.prepare()
		m.errBanner = ""
		m.cmdPhase = commandsBulkTags
		return m, textinput.Blink
	case strings.EqualFold(km.String(), "c"):
		if !m.hasBrowseSelection() {
			return m, m.setCommandStatus("No command to copy", true)
		}
		return m, copyCommandCmd(m.clip, m.cmdRows[m.browseCursor].Entry.Command)
	case strings.EqualFold(km.String(), "a"):
		m.returnToCommandsAfterForm = true
		m.screen = ScreenAdd
		m.errBanner = ""
		m.form.prepareAdd()
		return m, textinput.Blink
	case strings.EqualFold(km.String(), "e"):
		if !m.hasBrowseSelection() {
			return m, nil
		}
		m.editFromBrowse = true
		m.detailEntry = m.cmdRows[m.browseCursor].Entry
		m.cmdPhase = commandsEdit
		m.form.prepareEdit(m.detailEntry)
		return m, textinput.Blink
	case strings.EqualFold(km.String(), "d"):
		if !m.hasBrowseSelection() {
			return m, nil
		}
		m.deleteFromBrowse = true
		m.detailEntry = m.cmdRows[m.browseCursor].Entry
		m.cmdPhase = commandsDeleteConfirm
		return m, nil
	case key.Matches(km, m.keys.Up):
		m.moveBrowseBy(-1)
	case key.Matches(km, m.keys.Down):
		m.moveBrowseBy(1)
	case key.Matches(km, m.keys.Home):
		m.selectBrowseIndex(0)
	case key.Matches(km, m.keys.End):
		m.selectBrowseIndex(len(m.cmdRows) - 1)
	case key.Matches(km, m.keys.PageUp):
		m.moveBrowsePage(-1)
	case key.Matches(km, m.keys.PageDown):
		m.moveBrowsePage(1)
	case km.String() == "[":
		m.jumpBrowseGroup(-1)
	case km.String() == "]":
		m.jumpBrowseGroup(1)
	case key.Matches(km, m.keys.Enter):
		if !m.hasBrowseSelection() {
			return m, nil
		}
		m.detailEntry = m.cmdRows[m.browseCursor].Entry
		m.cmdPhase = commandsDetail
	}
	return m, nil
}

func (m *Model) updateBulkTagForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case m.keys.shouldQuit(km):
			return m, tea.Quit
		case km.Type == tea.KeyEsc:
			m.errBanner = ""
			m.bulkTagForm.blurAll()
			m.bulkTargetIDs = nil
			m.cmdPhase = commandsBrowse
			m.ensureBrowseVisible(true)
			return m, nil
		case km.Type == tea.KeyCtrlS:
			changes := m.bulkTagForm.changes()
			if len(changes) == 0 {
				m.errBanner = "add or remove at least one tag"
				return m, nil
			}
			m.errBanner = ""
			return m, bulkTagsCmd(m.repo, append([]int64(nil), m.bulkTargetIDs...), changes)
		case km.Type == tea.KeyTab || km.Type == tea.KeyShiftTab:
			if m.bulkTagForm.focus == bulkTagFocusAdd {
				m.bulkTagForm.focusField(bulkTagFocusRemove)
			} else {
				m.bulkTagForm.focusField(bulkTagFocusAdd)
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.bulkTagForm.focus == bulkTagFocusRemove {
		m.bulkTagForm.removeTI, cmd = m.bulkTagForm.removeTI.Update(msg)
	} else {
		m.bulkTagForm.addTI, cmd = m.bulkTagForm.addTI.Update(msg)
	}
	return m, cmd
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
