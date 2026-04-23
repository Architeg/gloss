package scan

import (
	"os"
	"path/filepath"

	"github.com/valeriybagrintsev/gloss/internal/model"
)

// ScanExecutableScripts returns one suggestion per executable regular file in dir (non-recursive).
func ScanExecutableScripts(dir string) ([]model.ScanSuggestion, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []model.ScanSuggestion
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		full := filepath.Join(dir, e.Name())
		fi, err := os.Stat(full)
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}
		if fi.Mode().Perm()&0111 == 0 {
			continue
		}
		out = append(out, model.ScanSuggestion{
			Command:  e.Name(),
			Type:     model.EntryTypeScript,
			Source:   full,
			Target:   "",
			Selected: true,
		})
	}
	return out, nil
}
