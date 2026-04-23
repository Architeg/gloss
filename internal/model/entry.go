package model

import (
	"strings"
	"time"
)

// Entry type values stored in the database.
const (
	EntryTypeManual   = "manual"
	EntryTypeAlias    = "alias"
	EntryTypeFunction = "function"
	EntryTypeScript   = "script"
)

// Entry is a persisted glossary item.
type Entry struct {
	ID           int64
	Command      string
	Description  string
	Tags         []string
	Type         string
	Source       string
	Target       string
	ManagedAlias bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NormalizeCommand trims whitespace for consistent lookup keys.
func NormalizeCommand(cmd string) string {
	return strings.TrimSpace(cmd)
}
