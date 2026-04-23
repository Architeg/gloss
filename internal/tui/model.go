package tui

import (
	"github.com/charmbracelet/bubbles/key"
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
	screen Screen
	cursor int

	config *model.Config
	repo   *storage.EntryRepo
}

// New returns the initial TUI model.
func New(opts Options) Model {
	return Model{
		styles: newStyles(),
		keys:   newBindings(),
		screen: ScreenHome,
		config: opts.Config,
		repo:   opts.Repo,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.screen == ScreenHome {
		switch {
		case m.keys.shouldQuit(msg):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(HomeMenu)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Enter):
			m.screen = HomeMenu[m.cursor].Screen
		}
		return m, nil
	}

	if m.keys.shouldQuit(msg) {
		return m, tea.Quit
	}
	if m.keys.shouldBack(msg) {
		m.screen = ScreenHome
	}
	return m, nil
}
