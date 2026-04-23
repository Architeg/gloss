package alias

import (
	"fmt"
	"os"

	"github.com/valeriybagrintsev/gloss/internal/model"
)

// Result reports what happened during a sync.
type Result struct {
	ShellPath  string
	BackupPath string
	Noop       bool
}

// Sync writes the managed-alias block to shellPath when content would change.
// If the merged content matches the existing file, it returns Noop without backup or write.
// If the file already exists and will change, creates a timestamped backup and prunes older
// .gloss.bak-* siblings. If the file does not exist, it is created (no backup).
func Sync(shellPath string, entries []model.Entry, retainBackups int) (Result, error) {
	if shellPath == "" {
		return Result{}, fmt.Errorf("shell file path is empty")
	}
	if retainBackups < 1 {
		retainBackups = defaultRetain
	}

	block := RenderManagedBlock(entries)
	var res Result
	res.ShellPath = shellPath

	fi, statErr := os.Stat(shellPath)
	exists := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return res, fmt.Errorf("stat shell file: %w", statErr)
	}

	var content string
	if exists {
		data, err := os.ReadFile(shellPath)
		if err != nil {
			return res, fmt.Errorf("read shell file: %w", err)
		}
		content = string(data)
	}

	out := MergeShellContent(content, block)
	if exists && out == content {
		res.Noop = true
		return res, nil
	}

	if exists {
		bp, err := BackupShellFile(shellPath)
		if err != nil {
			return res, fmt.Errorf("backup: %w", err)
		}
		res.BackupPath = bp
		if err := PruneBackups(shellPath, retainBackups); err != nil {
			return res, fmt.Errorf("prune backups: %w", err)
		}
	}

	mode := os.FileMode(0o644)
	if exists && fi != nil {
		mode = fi.Mode().Perm()
	}
	if err := os.WriteFile(shellPath, []byte(out), mode); err != nil {
		return res, fmt.Errorf("write shell file: %w", err)
	}
	return res, nil
}
