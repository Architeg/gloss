package scan

import (
	"os"
	"path/filepath"

	"github.com/valeriybagrintsev/gloss/internal/model"
)

// ResolveSources returns unique absolute paths to scan. ~/.zshrc is always included first,
// then entries from cfg.ScanPaths (expanded, deduplicated).
func ResolveSources(cfg *model.Config) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	zshrc := filepath.Join(home, ".zshrc")
	seen := make(map[string]struct{})
	var out []string

	add := func(p string) {
		p = expandPath(home, p)
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}

	add(zshrc)
	for _, p := range cfg.ScanPaths {
		add(p)
	}
	return out, nil
}

func expandPath(home, p string) string {
	p = filepath.Clean(os.ExpandEnv(p))
	if p == "" || p == "." {
		return ""
	}
	if p == "~" {
		return home
	}
	if len(p) >= 2 && p[:2] == "~/" {
		p = filepath.Join(home, p[2:])
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(home, p)
	}
	return filepath.Clean(p)
}
