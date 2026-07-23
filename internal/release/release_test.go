package release

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTargetContract(t *testing.T) {
	want := []Target{
		{GOOS: "darwin", GOARCH: "amd64", Executable: "gloss-darwin-amd64", Archive: "gloss-darwin-amd64.zip"},
		{GOOS: "darwin", GOARCH: "arm64", Executable: "gloss-darwin-arm64", Archive: "gloss-darwin-arm64.zip"},
		{GOOS: "linux", GOARCH: "amd64", Executable: "gloss-linux-amd64", Archive: "gloss-linux-amd64.zip"},
		{GOOS: "linux", GOARCH: "arm64", Executable: "gloss-linux-arm64", Archive: "gloss-linux-arm64.zip"},
	}
	got := Targets()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("Targets() = %#v, want %#v", got, want)
	}
}

func TestValidateArtifacts(t *testing.T) {
	dir := createFixtureArtifacts(t)
	if err := ValidateArtifacts(dir); err != nil {
		t.Fatalf("valid artifacts: %v", err)
	}
	checksums, err := os.ReadFile(filepath.Join(dir, checksumName))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(checksums)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || filepath.Base(fields[1]) != fields[1] {
			t.Fatalf("non-basename checksum line %q", line)
		}
	}
}

func TestValidateArtifactsRejectsIncompleteOrUnexpectedSets(t *testing.T) {
	t.Run("missing ZIP", func(t *testing.T) {
		dir := createFixtureArtifacts(t)
		if err := os.Remove(filepath.Join(dir, targets[0].Archive)); err != nil {
			t.Fatal(err)
		}
		if err := ValidateArtifacts(dir); err == nil {
			t.Fatal("missing ZIP accepted")
		}
	})
	t.Run("extra asset", func(t *testing.T) {
		dir := createFixtureArtifacts(t)
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("extra"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := ValidateArtifacts(dir); err == nil {
			t.Fatal("extra asset accepted")
		}
	})
	t.Run("path checksum", func(t *testing.T) {
		dir := createFixtureArtifacts(t)
		path := filepath.Join(dir, checksumName)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		data = []byte(strings.Replace(string(data), targets[0].Archive, "dist/"+targets[0].Archive, 1))
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := ValidateArtifacts(dir); err == nil {
			t.Fatal("path checksum accepted")
		}
	})
}

func TestValidateArtifactsRejectsWrongArchiveContents(t *testing.T) {
	for _, tt := range []struct {
		name    string
		entries []string
	}{
		{name: "generic executable", entries: []string{"gloss"}},
		{name: "extra entry", entries: []string{targets[0].Executable, "README.md"}},
		{name: "incorrect basename", entries: []string{"other-platform"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := createFixtureArtifacts(t)
			archivePath := filepath.Join(dir, targets[0].Archive)
			writeTestZIP(t, archivePath, tt.entries)
			rewriteFixtureChecksums(t, dir)
			if err := ValidateArtifacts(dir); err == nil {
				t.Fatalf("%s accepted", tt.name)
			}
		})
	}
}

func TestValidateEmptyOutputDirectory(t *testing.T) {
	if _, err := validateEmptyOutputDirectory(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	nonempty := t.TempDir()
	if err := os.WriteFile(filepath.Join(nonempty, "keep"), []byte("user data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := validateEmptyOutputDirectory(nonempty); err == nil {
		t.Fatal("nonempty output directory accepted")
	}
	if _, err := validateEmptyOutputDirectory(string(filepath.Separator)); err == nil {
		t.Fatal("filesystem root accepted")
	}
}

func createFixtureArtifacts(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, target := range targets {
		writeTestZIP(t, filepath.Join(dir, target.Archive), []string{target.Executable})
	}
	rewriteFixtureChecksums(t, dir)
	return dir
}

func writeTestZIP(t *testing.T, path string, names []string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, name := range names {
		header := &zip.FileHeader{Name: name}
		header.SetMode(0o755)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte("executable")); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func rewriteFixtureChecksums(t *testing.T, dir string) {
	t.Helper()
	var body strings.Builder
	for _, target := range targets {
		data, err := os.ReadFile(filepath.Join(dir, target.Archive))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		fmt.Fprintf(&body, "%x  %s\n", digest, target.Archive)
	}
	if err := os.WriteFile(filepath.Join(dir, checksumName), []byte(body.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}
