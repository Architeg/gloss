package alias

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const defaultRetain = 5

// BackupShellFile copies shellPath to shellPath.gloss.bak-YYYYMMDD-HHMMSS.
func BackupShellFile(shellPath string) (backupPath string, err error) {
	src, err := os.Open(shellPath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	ts := time.Now().Format("20060102-150405")
	backupPath = shellPath + ".gloss.bak-" + ts
	dst, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("create backup: %w", err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(backupPath)
		return "", fmt.Errorf("write backup: %w", err)
	}
	return backupPath, nil
}

// PruneBackups removes older shellPath.gloss.bak-* files, keeping the newest retain files.
func PruneBackups(shellPath string, retain int) error {
	if retain < 1 {
		retain = defaultRetain
	}
	pattern := shellPath + ".gloss.bak-*"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	if len(matches) <= retain {
		return nil
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i] > matches[j]
	})
	for _, p := range matches[retain:] {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove old backup %s: %w", p, err)
		}
	}
	return nil
}
