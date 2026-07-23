// Package release builds and validates Gloss release artifacts.
package release

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Architeg/gloss/internal/buildinfo"
	"github.com/Architeg/gloss/internal/update"
)

const (
	checksumName = "checksums.txt"
	maxBinary    = 64 << 20
)

// Target defines one supported release artifact.
type Target struct {
	GOOS       string
	GOARCH     string
	Executable string
	Archive    string
}

var targets = []Target{
	{GOOS: "darwin", GOARCH: "amd64", Executable: "gloss-darwin-amd64", Archive: "gloss-darwin-amd64.zip"},
	{GOOS: "darwin", GOARCH: "arm64", Executable: "gloss-darwin-arm64", Archive: "gloss-darwin-arm64.zip"},
	{GOOS: "linux", GOARCH: "amd64", Executable: "gloss-linux-amd64", Archive: "gloss-linux-amd64.zip"},
	{GOOS: "linux", GOARCH: "arm64", Executable: "gloss-linux-arm64", Archive: "gloss-linux-arm64.zip"},
}

// Targets returns a copy of the exact supported release matrix.
func Targets() []Target {
	return append([]Target(nil), targets...)
}

// Build creates and validates the complete five-file release set.
func Build(ctx context.Context, tag, outputDir, repositoryRoot string) error {
	if _, err := buildinfo.ValidateReleaseTag(tag); err != nil {
		return err
	}
	outputDir, err := validateEmptyOutputDirectory(outputDir)
	if err != nil {
		return err
	}
	repositoryRoot, err = filepath.Abs(repositoryRoot)
	if err != nil {
		return fmt.Errorf("resolve repository root: %w", err)
	}
	if info, err := os.Stat(filepath.Join(repositoryRoot, "go.mod")); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("repository root does not contain a regular go.mod")
	}

	staging, err := os.MkdirTemp(outputDir, ".gloss-release-stage-")
	if err != nil {
		return fmt.Errorf("create release staging directory: %w", err)
	}
	defer os.RemoveAll(staging)
	assetDir := filepath.Join(staging, "assets")
	binaryDir := filepath.Join(staging, "binaries")
	if err := os.Mkdir(assetDir, 0o700); err != nil {
		return err
	}
	if err := os.Mkdir(binaryDir, 0o700); err != nil {
		return err
	}

	for _, target := range targets {
		binaryPath := filepath.Join(binaryDir, target.Executable)
		if err := buildTarget(ctx, repositoryRoot, tag, target, binaryPath); err != nil {
			return err
		}
		if err := writeArchive(filepath.Join(assetDir, target.Archive), binaryPath, target.Executable); err != nil {
			return err
		}
	}
	if err := writeChecksums(assetDir); err != nil {
		return err
	}
	if err := ValidateArtifacts(assetDir); err != nil {
		return fmt.Errorf("validate staged release: %w", err)
	}
	for _, name := range expectedAssetNames() {
		if err := os.Rename(filepath.Join(assetDir, name), filepath.Join(outputDir, name)); err != nil {
			return fmt.Errorf("publish staged asset %s: %w", name, err)
		}
	}
	if err := os.RemoveAll(staging); err != nil {
		return fmt.Errorf("remove release staging directory: %w", err)
	}
	return ValidateArtifacts(outputDir)
}

func buildTarget(ctx context.Context, root, tag string, target Target, output string) error {
	ldflag := "-X github.com/Architeg/gloss/internal/buildinfo.InjectedVersion=" + tag
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags", ldflag, "-o", output, "./cmd/gloss")
	cmd.Dir = root
	cmd.Env = append(filteredBuildEnvironment(os.Environ()),
		"CGO_ENABLED=0",
		"GOOS="+target.GOOS,
		"GOARCH="+target.GOARCH,
	)
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build %s/%s: %w: %s", target.GOOS, target.GOARCH, err, strings.TrimSpace(string(outputBytes)))
	}
	info, err := os.Lstat(output)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", target.Executable, err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("build produced invalid executable %s", target.Executable)
	}
	if err := os.Chmod(output, 0o755); err != nil {
		return fmt.Errorf("chmod %s: %w", target.Executable, err)
	}
	return nil
}

func filteredBuildEnvironment(environment []string) []string {
	result := make([]string, 0, len(environment))
	for _, value := range environment {
		if strings.HasPrefix(value, "GOOS=") ||
			strings.HasPrefix(value, "GOARCH=") ||
			strings.HasPrefix(value, "CGO_ENABLED=") {
			continue
		}
		result = append(result, value)
	}
	return result
}

func writeArchive(archivePath, binaryPath, executableName string) (retErr error) {
	data, err := os.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", executableName, err)
	}
	if len(data) == 0 {
		return fmt.Errorf("executable %s is empty", executableName)
	}
	file, err := os.OpenFile(archivePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", filepath.Base(archivePath), err)
	}
	defer func() {
		if err := file.Close(); retErr == nil && err != nil {
			retErr = err
		}
	}()
	writer := zip.NewWriter(file)
	header := &zip.FileHeader{Name: executableName, Method: zip.Deflate}
	header.SetMode(0o755)
	header.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
	entry, err := writer.CreateHeader(header)
	if err != nil {
		_ = writer.Close()
		return fmt.Errorf("create ZIP entry %s: %w", executableName, err)
	}
	if _, err := entry.Write(data); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write ZIP entry %s: %w", executableName, err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close %s: %w", filepath.Base(archivePath), err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", filepath.Base(archivePath), err)
	}
	return nil
}

func writeChecksums(assetDir string) error {
	var body strings.Builder
	for _, target := range targets {
		data, err := os.ReadFile(filepath.Join(assetDir, target.Archive))
		if err != nil {
			return err
		}
		digest := sha256.Sum256(data)
		fmt.Fprintf(&body, "%x  %s\n", digest, target.Archive)
	}
	return os.WriteFile(filepath.Join(assetDir, checksumName), []byte(body.String()), 0o600)
}

// ValidateArtifacts verifies the exact release set, checksum manifest, and ZIP
// contract consumed by the updater and installer.
func ValidateArtifacts(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("unexpected non-file release asset %s", entry.Name())
		}
		got = append(got, entry.Name())
	}
	sort.Strings(got)
	want := expectedAssetNames()
	sort.Strings(want)
	if !equalStrings(got, want) {
		return fmt.Errorf("release assets = %v, want %v", got, want)
	}

	checksums, err := os.ReadFile(filepath.Join(dir, checksumName))
	if err != nil {
		return err
	}
	if nonemptyLineCount(checksums) != len(targets) {
		return fmt.Errorf("checksums.txt must contain exactly %d entries", len(targets))
	}
	for _, target := range targets {
		archivePath := filepath.Join(dir, target.Archive)
		archive, err := os.ReadFile(archivePath)
		if err != nil {
			return err
		}
		expected, err := update.ParseChecksums(checksums, target.Archive)
		if err != nil {
			return err
		}
		actual := sha256.Sum256(archive)
		if !bytes.Equal(expected, actual[:]) {
			return fmt.Errorf("checksum mismatch for %s", target.Archive)
		}
		if _, err := update.ValidateArchive(archive, target.Executable, maxBinary); err != nil {
			return fmt.Errorf("%s: %w", target.Archive, err)
		}
	}
	return nil
}

func validateEmptyOutputDirectory(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("output directory is required")
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	if absolute == string(filepath.Separator) || absolute == filepath.Clean(home) || absolute == filepath.Clean(cwd) {
		return "", fmt.Errorf("refusing unsafe output directory %s", absolute)
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return "", fmt.Errorf("inspect output directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("output path must be an existing non-symlink directory")
	}
	entries, err := os.ReadDir(absolute)
	if err != nil {
		return "", err
	}
	if len(entries) != 0 {
		return "", fmt.Errorf("output directory must be empty")
	}
	return absolute, nil
}

func expectedAssetNames() []string {
	names := make([]string, 0, len(targets)+1)
	for _, target := range targets {
		names = append(names, target.Archive)
	}
	return append(names, checksumName)
}

func nonemptyLineCount(data []byte) int {
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
