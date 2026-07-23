package update

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestStateStoreDueAndAtomicWrite(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "update-state.json")
	store := StateStore{Path: path, Now: func() time.Time { return now }}
	if !store.Due(24 * time.Hour) {
		t.Fatal("missing state was not due")
	}
	if err := store.MarkCompleted("1.2.3"); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil || !state.LastCompleted.Equal(now) || state.LatestVersion != "1.2.3" {
		t.Fatalf("state = %#v, %v", state, err)
	}
	if store.Due(24 * time.Hour) {
		t.Fatal("fresh state was due")
	}
	store.Now = func() time.Time { return now.Add(24 * time.Hour) }
	if !store.Due(24 * time.Hour) {
		t.Fatal("expired state was not due")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("state mode = %v, %v", info.Mode(), err)
		}
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".update-state-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary state files remain: %v, %v", matches, err)
	}
}

func TestStateStoreMalformedStateDoesNotBlockDueCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update-state.json")
	if err := os.WriteFile(path, []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := StateStore{Path: path}
	if !store.Due(24 * time.Hour) {
		t.Fatal("malformed state prevented a due check")
	}
}

func TestStateStoreRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior differs on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "state")
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	store := StateStore{Path: path}
	if err := store.MarkCompleted("1.0.0"); err == nil {
		t.Fatal("symlink state accepted")
	}
}
