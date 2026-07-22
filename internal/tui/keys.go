package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type bindings struct {
	Up       key.Binding
	Down     key.Binding
	Home     key.Binding
	End      key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Enter    key.Binding
	Quit     key.Binding
	Back     key.Binding
	Left     key.Binding
	Right    key.Binding
}

func newBindings() bindings {
	return bindings{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
		),
		Home: key.NewBinding(
			key.WithKeys("home"),
		),
		End: key.NewBinding(
			key.WithKeys("end"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
		),
	}
}

func (b bindings) shouldQuit(msg tea.KeyMsg) bool {
	return key.Matches(msg, b.Quit)
}

func (b bindings) shouldBack(msg tea.KeyMsg) bool {
	return key.Matches(msg, b.Back) || key.Matches(msg, b.Left)
}
