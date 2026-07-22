package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Open opens (creating if needed) a SQLite database at path and runs migrations.
func Open(path string) (*sql.DB, error) {
	resolved, err := prepareDatabasePath(path)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", resolved)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pragma foreign_keys: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := Migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func prepareDatabasePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("database path is empty")
	}
	resolved, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("resolve database path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o700); err != nil {
		return "", fmt.Errorf("create db directory: %w", err)
	}

	info, err := os.Lstat(resolved)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("database path %s is a symlink", resolved)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("database path %s is not a regular file", resolved)
		}
		if err := os.Chmod(resolved, 0o600); err != nil {
			return "", fmt.Errorf("secure database file: %w", err)
		}
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect database file: %w", err)
	}

	f, err := os.OpenFile(resolved, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("create database file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close database file: %w", err)
	}
	if err := os.Chmod(resolved, 0o600); err != nil {
		return "", fmt.Errorf("secure database file: %w", err)
	}
	return resolved, nil
}
