package scan

import (
	"os"
	"sort"
	"strings"

	"github.com/valeriybagrintsev/gloss/internal/model"
)

// Result is the outcome of a scan pass.
type Result struct {
	Sources         []string
	Suggestions     []model.ScanSuggestion
	SkippedExisting int
	SkippedPaths    []string // missing or unreadable paths (informational)
}

// Run resolves sources from config, scans files and directories, drops items whose
// command already exists in existingCommands (normalized), and dedupes by command within the scan.
func Run(cfg *model.Config, existingCommands map[string]struct{}) (Result, error) {
	paths, err := ResolveSources(cfg)
	if err != nil {
		return Result{}, err
	}

	existing := existingCommands
	if existing == nil {
		existing = map[string]struct{}{}
	}

	var r Result
	r.Sources = append(r.Sources, paths...)
	seenInScan := make(map[string]struct{})
	var pending []model.ScanSuggestion

	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			r.SkippedPaths = append(r.SkippedPaths, p)
			continue
		}
		switch {
		case fi.IsDir():
			list, err := ScanExecutableScripts(p)
			if err != nil {
				r.SkippedPaths = append(r.SkippedPaths, p)
				continue
			}
			pending = append(pending, list...)
		default:
			list, err := ParseShellFile(p)
			if err != nil {
				r.SkippedPaths = append(r.SkippedPaths, p)
				continue
			}
			pending = append(pending, list...)
		}
	}

	for _, s := range pending {
		cmd := model.NormalizeCommand(s.Command)
		if cmd == "" {
			continue
		}
		s.Command = cmd
		if _, dup := seenInScan[cmd]; dup {
			continue
		}
		if _, has := existing[cmd]; has {
			r.SkippedExisting++
			continue
		}
		seenInScan[cmd] = struct{}{}
		r.Suggestions = append(r.Suggestions, s)
	}

	sort.Slice(r.Suggestions, func(i, j int) bool {
		return strings.ToLower(r.Suggestions[i].Command) < strings.ToLower(r.Suggestions[j].Command)
	})
	return r, nil
}

// SuggestionToEntry maps a scan row to a DB entry (ManagedAlias=false, Tags empty).
func SuggestionToEntry(s model.ScanSuggestion) model.Entry {
	desc := ""
	switch s.Type {
	case model.EntryTypeAlias:
		desc = strings.TrimSpace(s.Target)
	default:
		desc = "Imported from scan"
	}
	return model.Entry{
		Command:      model.NormalizeCommand(s.Command),
		Description:  desc,
		Tags:         nil,
		Type:         s.Type,
		Source:       s.Source,
		Target:       s.Target,
		ManagedAlias: false,
	}
}
