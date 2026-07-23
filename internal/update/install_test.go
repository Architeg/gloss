package update

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInspectExecutableAcceptsWritableManualBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable permissions are required")
	}
	path := filepath.Join(t.TempDir(), "gloss")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	layout, err := InspectExecutable(path, "linux", "amd64")
	resolved, resolveErr := filepath.EvalSymlinks(path)
	if err != nil || resolveErr != nil || layout.Kind != LayoutManual || layout.Path != resolved {
		t.Fatalf("InspectExecutable = %#v, %v", layout, err)
	}
}

func TestInspectExecutableRejectsUnsafeLayouts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix symlink and permission behavior is required")
	}
	t.Run("nonregular", func(t *testing.T) {
		if _, err := InspectExecutable(t.TempDir(), "linux", "amd64"); err == nil {
			t.Fatal("directory accepted as executable")
		}
	})
	t.Run("unwritable", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "gloss")
		if err := os.WriteFile(path, []byte("old"), 0o555); err != nil {
			t.Fatal(err)
		}
		if _, err := InspectExecutable(path, "linux", "amd64"); err == nil {
			t.Fatal("unwritable executable accepted")
		}
	})
	t.Run("unsupported", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "gloss")
		if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := InspectExecutable(path, "windows", "amd64"); !errors.Is(err, ErrUnsupportedPlatform) {
			t.Fatalf("unsupported platform error = %v", err)
		}
	})
	t.Run("ambiguous symlink", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		link := filepath.Join(dir, "gloss")
		if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := InspectExecutable(link, "linux", "amd64"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("symlink error = %v", err)
		}
	})
}

func TestHomebrewPathsAreDetected(t *testing.T) {
	for _, path := range []string{
		"/usr/local/Cellar/gloss/0.1.1/bin/gloss",
		"/opt/homebrew/Cellar/gloss/0.1.1/bin/gloss",
		"/usr/local/opt/gloss/bin/gloss",
		"/opt/homebrew/opt/gloss/bin/gloss",
		"/home/linuxbrew/.linuxbrew/Cellar/gloss/0.1.1/bin/gloss",
	} {
		if !isHomebrewPath(path) {
			t.Fatalf("Homebrew path not detected: %s", path)
		}
	}
	if isHomebrewPath("/usr/local/bin/gloss") {
		t.Fatal("ordinary /usr/local/bin path classified as Homebrew without a resolved Cellar path")
	}
}

func TestInspectExecutableRejectsSymlinkResolvedIntoHomebrewCellar(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix symlink behavior is required")
	}
	root := t.TempDir()
	targetDir := filepath.Join(root, "Cellar", "gloss", "1.0.0", "bin")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(targetDir, "gloss")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "gloss")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := InspectExecutable(link, "linux", "amd64"); !IsHomebrew(err) {
		t.Fatalf("resolved Homebrew error = %v", err)
	}
}

func TestProtectedExecutablePaths(t *testing.T) {
	for _, path := range []string{"/bin/gloss", "/usr/bin/gloss", "/usr/sbin/gloss", "/System/bin/gloss"} {
		if !isProtectedExecutablePath(path) {
			t.Fatalf("protected path not detected: %s", path)
		}
	}
	if isProtectedExecutablePath("/usr/local/bin/gloss") {
		t.Fatal("manual /usr/local/bin path classified as protected")
	}
}

func TestInstallVerifiedAtomicallyReplacesExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable permissions are required")
	}
	path := filepath.Join(t.TempDir(), "gloss")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	layout, err := InspectExecutable(path, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	verified := VerifiedUpdate{
		Version:        "1.2.3",
		ExecutableName: layout.Platform.Executable,
		Data:           []byte("new executable"),
		Platform:       layout.Platform,
	}
	if err := InstallVerified(layout, verified); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "new executable" {
		t.Fatalf("installed data = %q, %v", data, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm()&0111 == 0 {
		t.Fatalf("installed mode = %v, %v", info.Mode(), err)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".gloss-update-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("staged files remain: %v, %v", matches, err)
	}
}

func TestInstallVerifiedDetectsTargetChangeAndPreservesReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable permissions are required")
	}
	path := filepath.Join(t.TempDir(), "gloss")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	layout, err := InspectExecutable(path, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(filepath.Dir(path), "replacement")
	if err := os.WriteFile(replacement, []byte("changed"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	verified := VerifiedUpdate{
		ExecutableName: layout.Platform.Executable,
		Data:           []byte("new"),
		Platform:       layout.Platform,
	}
	if err := InstallVerified(layout, verified); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("target-change error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "changed" {
		t.Fatalf("changed target was modified: %q, %v", data, err)
	}
}

func TestInstallVerifiedRejectsMismatchedAndUnvalidatedLayouts(t *testing.T) {
	if err := InstallVerified(Layout{}, VerifiedUpdate{Data: []byte("x")}); err == nil {
		t.Fatal("unvalidated layout accepted")
	}
	if err := InstallVerified(Layout{Kind: LayoutHomebrew, Path: "/opt/homebrew/Cellar/gloss"}, VerifiedUpdate{}); !IsHomebrew(err) {
		t.Fatalf("Homebrew layout error = %v", err)
	}
}

func TestInstallFailuresPreserveOriginalAndRemoveStaging(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable permissions are required")
	}
	for _, failure := range []string{"write", "chmod", "sync", "close", "rename"} {
		t.Run(failure, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "gloss")
			if err := os.WriteFile(path, []byte("original"), 0o755); err != nil {
				t.Fatal(err)
			}
			layout, err := InspectExecutable(path, "linux", "amd64")
			if err != nil {
				t.Fatal(err)
			}
			verified := VerifiedUpdate{
				ExecutableName: layout.Platform.Executable,
				Data:           []byte("replacement"),
				Platform:       layout.Platform,
			}
			fs := &faultFileSystem{failure: failure}
			if err := installVerified(fs, layout, verified); err == nil {
				t.Fatalf("%s failure succeeded", failure)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "original" {
				t.Fatalf("original changed after %s failure: %q, %v", failure, data, err)
			}
			matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".gloss-update-*"))
			if err != nil || len(matches) != 0 {
				t.Fatalf("staged files remain after %s failure: %v, %v", failure, matches, err)
			}
		})
	}
}

type faultFileSystem struct {
	failure string
}

func (f *faultFileSystem) CreateTemp(dir, pattern string) (stagedFile, error) {
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	return &faultStagedFile{File: file, failure: f.failure}, nil
}

func (f *faultFileSystem) Lstat(name string) (os.FileInfo, error) { return os.Lstat(name) }
func (f *faultFileSystem) Remove(name string) error               { return os.Remove(name) }
func (f *faultFileSystem) Rename(oldPath, newPath string) error {
	if f.failure == "rename" {
		return errors.New("injected rename failure")
	}
	return os.Rename(oldPath, newPath)
}

type faultStagedFile struct {
	*os.File
	failure string
}

func (f *faultStagedFile) Write(data []byte) (int, error) {
	if f.failure == "write" {
		if len(data) > 0 {
			_, _ = f.File.Write(data[:1])
		}
		return 1, io.ErrUnexpectedEOF
	}
	return f.File.Write(data)
}

func (f *faultStagedFile) Chmod(mode os.FileMode) error {
	if f.failure == "chmod" {
		return errors.New("injected chmod failure")
	}
	return f.File.Chmod(mode)
}

func (f *faultStagedFile) Sync() error {
	if f.failure == "sync" {
		return errors.New("injected sync failure")
	}
	return f.File.Sync()
}

func (f *faultStagedFile) Close() error {
	if f.failure == "close" {
		_ = f.File.Close()
		return errors.New("injected close failure")
	}
	return f.File.Close()
}
