package update

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const HomebrewUpgradeCommand = "brew upgrade Architeg/tap/gloss"

// LayoutKind classifies how the running executable is managed.
type LayoutKind int

const (
	LayoutManual LayoutKind = iota
	LayoutHomebrew
)

// Layout is a validated executable replacement target.
type Layout struct {
	Kind     LayoutKind
	Invoked  string
	Path     string
	Mode     os.FileMode
	Platform Platform
	identity os.FileInfo
}

// HomebrewError explains why self-replacement is forbidden.
type HomebrewError struct {
	Path string
}

func (e *HomebrewError) Error() string {
	return fmt.Sprintf("Homebrew-managed installation at %s; run: %s", e.Path, HomebrewUpgradeCommand)
}

// IsHomebrew reports whether an error identifies a Homebrew installation.
func IsHomebrew(err error) bool {
	var target *HomebrewError
	return errors.As(err, &target)
}

// InspectRunningExecutable validates the current executable's installation layout.
func InspectRunningExecutable() (Layout, error) {
	path, err := os.Executable()
	if err != nil {
		return Layout{}, fmt.Errorf("resolve executable: %w", err)
	}
	return InspectExecutable(path, runtime.GOOS, runtime.GOARCH)
}

// InspectExecutable validates a candidate executable without modifying it.
func InspectExecutable(executablePath, goos, goarch string) (Layout, error) {
	platform, err := PlatformFor(goos, goarch)
	if err != nil {
		return Layout{}, err
	}
	if strings.TrimSpace(executablePath) == "" || !filepath.IsAbs(executablePath) {
		return Layout{}, errors.New("executable path is not absolute")
	}
	invoked := filepath.Clean(executablePath)
	info, err := os.Lstat(invoked)
	if err != nil {
		return Layout{}, fmt.Errorf("inspect executable: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(invoked)
	if err != nil {
		return Layout{}, fmt.Errorf("resolve executable symlinks: %w", err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return Layout{}, fmt.Errorf("normalize executable path: %w", err)
	}
	resolved = filepath.Clean(resolved)

	if isHomebrewPath(invoked) || isHomebrewPath(resolved) {
		return Layout{Kind: LayoutHomebrew, Invoked: invoked, Path: resolved, Platform: platform}, &HomebrewError{Path: resolved}
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return Layout{}, fmt.Errorf("ambiguous symlinked executable layout: %s -> %s", invoked, resolved)
	}
	if isProtectedExecutablePath(resolved) {
		return Layout{}, fmt.Errorf("refusing protected executable path %s", resolved)
	}
	if !info.Mode().IsRegular() {
		return Layout{}, fmt.Errorf("executable is not a regular file: %s", resolved)
	}
	if info.Mode().Perm()&0111 == 0 {
		return Layout{}, fmt.Errorf("executable does not have an executable mode: %s", resolved)
	}
	if info.Mode().Perm()&0222 == 0 {
		return Layout{}, fmt.Errorf("executable is not writable: %s", resolved)
	}
	dir := filepath.Dir(resolved)
	dirInfo, err := os.Lstat(dir)
	if err != nil {
		return Layout{}, fmt.Errorf("inspect executable directory: %w", err)
	}
	if !dirInfo.IsDir() || dirInfo.Mode()&os.ModeSymlink != 0 {
		return Layout{}, fmt.Errorf("executable directory is unsafe: %s", dir)
	}
	if dirInfo.Mode().Perm()&0222 == 0 {
		return Layout{}, fmt.Errorf("executable directory is not writable: %s", dir)
	}
	return Layout{
		Kind:     LayoutManual,
		Invoked:  invoked,
		Path:     resolved,
		Mode:     info.Mode().Perm(),
		Platform: platform,
		identity: info,
	}, nil
}

func isHomebrewPath(value string) bool {
	clean := filepath.ToSlash(filepath.Clean(value))
	return strings.Contains(clean, "/Cellar/") ||
		strings.HasPrefix(clean, "/usr/local/opt/") ||
		strings.HasPrefix(clean, "/opt/homebrew/opt/") ||
		strings.Contains(clean, "/homebrew/opt/") ||
		strings.Contains(clean, "/.linuxbrew/opt/")
}

func isProtectedExecutablePath(value string) bool {
	clean := filepath.ToSlash(filepath.Clean(value))
	for _, prefix := range []string{"/bin/", "/sbin/", "/usr/bin/", "/usr/sbin/", "/System/"} {
		if strings.HasPrefix(clean, prefix) {
			return true
		}
	}
	return false
}

// InstallVerified atomically replaces an eligible manual executable.
func InstallVerified(layout Layout, verified VerifiedUpdate) error {
	return installVerified(osFileSystem{}, layout, verified)
}

type stagedFile interface {
	io.Writer
	Chmod(os.FileMode) error
	Sync() error
	Close() error
	Name() string
}

type fileSystem interface {
	CreateTemp(string, string) (stagedFile, error)
	Lstat(string) (os.FileInfo, error)
	Rename(string, string) error
	Remove(string) error
}

type osFileSystem struct{}

func (osFileSystem) CreateTemp(dir, pattern string) (stagedFile, error) {
	return os.CreateTemp(dir, pattern)
}
func (osFileSystem) Lstat(name string) (os.FileInfo, error) { return os.Lstat(name) }
func (osFileSystem) Rename(oldPath, newPath string) error   { return os.Rename(oldPath, newPath) }
func (osFileSystem) Remove(name string) error               { return os.Remove(name) }

func installVerified(fs fileSystem, layout Layout, verified VerifiedUpdate) (retErr error) {
	if layout.Kind == LayoutHomebrew {
		return &HomebrewError{Path: layout.Path}
	}
	if layout.Kind != LayoutManual || layout.identity == nil {
		return errors.New("executable layout has not been safely validated")
	}
	if verified.ExecutableName != layout.Platform.Executable ||
		verified.Platform.GOOS != layout.Platform.GOOS ||
		verified.Platform.GOARCH != layout.Platform.GOARCH {
		return errors.New("verified executable does not match the installed platform")
	}
	if len(verified.Data) == 0 {
		return errors.New("verified executable is empty")
	}
	if err := revalidateTarget(fs, layout); err != nil {
		return err
	}

	temp, err := fs.CreateTemp(filepath.Dir(layout.Path), ".gloss-update-*")
	if err != nil {
		return fmt.Errorf("create staged executable: %w", err)
	}
	tempPath := temp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temp.Close()
		}
		if retErr != nil {
			_ = fs.Remove(tempPath)
		}
	}()

	if err := writeComplete(temp, verified.Data); err != nil {
		return fmt.Errorf("write staged executable: %w", err)
	}
	mode := layout.Mode.Perm()
	if mode&0111 == 0 {
		mode = 0o755
	}
	if err := temp.Chmod(mode); err != nil {
		return fmt.Errorf("chmod staged executable: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync staged executable: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close staged executable: %w", err)
	}
	closed = true
	if err := revalidateTarget(fs, layout); err != nil {
		return err
	}
	if err := fs.Rename(tempPath, layout.Path); err != nil {
		return fmt.Errorf("replace executable: %w", err)
	}
	return nil
}

func writeComplete(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func revalidateTarget(fs fileSystem, layout Layout) error {
	info, err := fs.Lstat(layout.Path)
	if err != nil {
		return fmt.Errorf("revalidate executable: %w", err)
	}
	if !info.Mode().IsRegular() || !os.SameFile(layout.identity, info) {
		return errors.New("executable changed during update")
	}
	return nil
}
