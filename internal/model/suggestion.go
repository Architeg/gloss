package model

// ScanSuggestion is a candidate item produced by a future scan pass.
type ScanSuggestion struct {
	Command  string
	Type     string
	Source   string
	Target   string
	Selected bool
}
