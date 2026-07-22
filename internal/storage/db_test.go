package storage

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpenCreatesPrivateDatabaseAndRunsMigrations(t *testing.T) {
	requireUnixPermissions(t)
	path := filepath.Join(t.TempDir(), "private", "gloss.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	assertFileMode(t, path, 0o600)
	assertFileMode(t, filepath.Dir(path), 0o700)
	var table string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'entries'`).Scan(&table); err != nil {
		t.Fatalf("entries migration missing: %v", err)
	}
}

func TestOpenTightensExistingDatabase(t *testing.T) {
	requireUnixPermissions(t)
	parent := filepath.Join(t.TempDir(), "existing")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "gloss.db")
	if err := os.WriteFile(path, nil, 0o666); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o666); err != nil {
		t.Fatal(err)
	}
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	assertFileMode(t, path, 0o600)
	assertFileMode(t, parent, 0o755)
}

func TestOpenRejectsSymlinkDatabase(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior differs on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.db")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "gloss.db")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := Open(link); err == nil {
		t.Fatal("Open accepted a symlink database")
	}
}

func TestOpenRejectsNonRegularDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gloss.db")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("Open accepted a directory as the database")
	}
}

func requireUnixPermissions(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not portable to Windows")
	}
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode for %s = %04o, want %04o", path, got, want)
	}
}
