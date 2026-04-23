package tui

// Screen identifies the active TUI view.
type Screen int

const (
	ScreenHome Screen = iota
	ScreenCommands
	ScreenAdd
	ScreenScan
	ScreenAliases
	ScreenSettings
)

type menuItem struct {
	Title  string
	Screen Screen
}

// HomeMenu is the main menu order and routing targets.
var HomeMenu = []menuItem{
	{Title: "Commands", Screen: ScreenCommands},
	{Title: "Add", Screen: ScreenAdd},
	{Title: "Scan", Screen: ScreenScan},
	{Title: "Aliases", Screen: ScreenAliases},
	{Title: "Settings", Screen: ScreenSettings},
}

func screenTitle(s Screen) string {
	switch s {
	case ScreenCommands:
		return "Commands"
	case ScreenAdd:
		return "Add"
	case ScreenScan:
		return "Scan"
	case ScreenAliases:
		return "Aliases"
	case ScreenSettings:
		return "Settings"
	default:
		return ""
	}
}

func placeholderBlurb(s Screen) string {
	switch s {
	case ScreenCommands:
		return "Browse saved commands"
	case ScreenAdd:
		return "Add a new entry"
	case ScreenScan:
		return "Find aliases, functions, and scripts"
	case ScreenAliases:
		return "Manage aliases and sync to zshrc"
	case ScreenSettings:
		return "Configure shell file and storage paths"
	default:
		return ""
	}
}
