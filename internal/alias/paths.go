package alias

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/valeriybagrintsev/gloss/internal/model"
)

// ResolveShellPath returns the absolute path to the zshrc (or configured shell file).
func ResolveShellPath(cfg *model.Config) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	p := strings.TrimSpace(cfg.ShellFile)
	if p == "" {
		return filepath.Join(home, ".zshrc"), nil
	}
	p = filepath.Clean(os.ExpandEnv(p))
	if p == "~" {
		return home, nil
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:]), nil
	}
	if !filepath.IsAbs(p) {
		return filepath.Join(home, p), nil
	}
	return p, nil
}
