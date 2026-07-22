package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadCreatesPrivateConfigPaths(t *testing.T) {
	requireUnixPermissions(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")

	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	assertMode(t, filepath.Join(home, relConfigDir), 0o700)
	assertMode(t, filepath.Join(home, relConfigDir, relConfigFile), 0o600)
}

func TestLoadTightensExistingConfigPaths(t *testing.T) {
	requireUnixPermissions(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")
	if err := os.Chmod(home, 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, relConfigDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(home, ".config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, relConfigFile)
	if err := os.WriteFile(path, []byte("use_color: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UseColor {
		t.Fatal("existing configuration was not loaded")
	}
	assertMode(t, dir, 0o700)
	assertMode(t, path, 0o600)
	assertMode(t, home, 0o755)
	assertMode(t, filepath.Join(home, ".config"), 0o755)
}

func TestLoadRejectsSymlinkConfigDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior differs on Windows")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	parent := filepath.Join(home, ".config")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(parent, "gloss")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("Load accepted a symlink config directory")
	}
}

func TestLoadRejectsSymlinkConfigFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior differs on Windows")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, relConfigDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(target, []byte("use_color: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, relConfigFile)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("Load accepted a symlink config file")
	}
}

func TestLoadRejectsNonRegularConfigFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, relConfigDir)
	if err := os.MkdirAll(filepath.Join(dir, relConfigFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("Load accepted a directory as the config file")
	}
}

func requireUnixPermissions(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not portable to Windows")
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	if got := modeOf(t, path); got != want {
		t.Fatalf("mode for %s = %04o, want %04o", path, got, want)
	}
}

func modeOf(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}
