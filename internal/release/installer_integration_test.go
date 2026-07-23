package release

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const installerTestAsset = "gloss-darwin-amd64.zip"
const installerTestExecutable = "gloss-darwin-amd64"

type installerFixture struct {
	tag       string
	archive   []byte
	checksums []byte
	status    map[string]int
}

func TestInstallerEndToEndSuccess(t *testing.T) {
	fixture := validInstallerFixture(t)
	server := newInstallerServer(t, fixture)

	for _, tt := range []struct {
		name        string
		version     string
		pathPresent bool
	}{
		{name: "latest", version: "latest"},
		{name: "plain version", version: "0.1.1"},
		{name: "tagged version", version: "v0.1.1"},
		{name: "destination in PATH", version: "v0.1.1", pathPresent: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result := runInstallerIntegration(t, server.URL, tt.version, fixture, tt.pathPresent, nil)
			if result.err != nil {
				t.Fatalf("installer failed: %v\n%s", result.err, result.output)
			}
			if !strings.Contains(result.output, "Gloss 0.1.1 installed.") ||
				!strings.Contains(result.output, "Destination: "+result.target) {
				t.Fatalf("success output = %q", result.output)
			}
			if tt.pathPresent {
				if !strings.Contains(result.output, "Run: gloss version") {
					t.Fatalf("PATH-present output = %q", result.output)
				}
			} else if !strings.Contains(result.output, "is not in PATH") {
				t.Fatalf("PATH-absent output = %q", result.output)
			}
			info, err := os.Lstat(result.target)
			if err != nil {
				t.Fatal(err)
			}
			if !info.Mode().IsRegular() || info.Mode().Perm() != 0o755 {
				t.Fatalf("installed mode = %v", info.Mode())
			}
			command := exec.Command(result.target, "version")
			output, err := command.CombinedOutput()
			if err != nil || strings.TrimSpace(string(output)) != "gloss 0.1.1" {
				t.Fatalf("installed version = %q, %v", output, err)
			}
		})
	}
}

func TestInstallerAtomicallyReplacesEligibleFile(t *testing.T) {
	fixture := validInstallerFixture(t)
	server := newInstallerServer(t, fixture)
	result := runInstallerIntegration(t, server.URL, "v0.1.1", fixture, false, []byte("old executable"))
	if result.err != nil {
		t.Fatalf("replace failed: %v\n%s", result.err, result.output)
	}
	data, err := os.ReadFile(result.target)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(data, result.original) || !bytes.Equal(data, installerExecutableData()) {
		t.Fatalf("replacement data = %q", data)
	}
	assertNoInstallerTemps(t, result)
}

func TestInstallerFailuresPreserveExistingTarget(t *testing.T) {
	valid := validInstallerFixture(t)
	tests := []struct {
		name       string
		version    string
		mutate     func(*testing.T, *installerFixture)
		extraEnv   []string
		want       string
		unameS     string
		unameM     string
		installDir func(*testing.T, string) string
	}{
		{
			name: "unsupported OS", want: "unsupported operating system",
			unameS: "Windows", unameM: "amd64",
		},
		{
			name: "unsupported architecture", want: "unsupported architecture",
			unameS: "Darwin", unameM: "386",
		},
		{name: "malformed version", version: "../v0.1.1", want: "version must match"},
		{name: "prerelease version", version: "v0.1.1-rc.1", want: "version must match"},
		{
			name: "missing release", version: "v0.1.2", want: "download failed",
		},
		{
			name:    "prerelease latest",
			version: "latest",
			mutate: func(_ *testing.T, f *installerFixture) {
				f.tag = "v0.1.1-rc.1"
			},
			want: "version must match",
		},
		{
			name: "missing checksums",
			mutate: func(_ *testing.T, f *installerFixture) {
				f.status = map[string]int{"checksums.txt": http.StatusNotFound}
			},
			want: "download failed",
		},
		{
			name: "malformed checksum",
			mutate: func(_ *testing.T, f *installerFixture) {
				f.checksums = []byte("not-a-digest  " + installerTestAsset + "\n")
			},
			want: "invalid SHA-256",
		},
		{
			name: "checksum path",
			mutate: func(_ *testing.T, f *installerFixture) {
				f.checksums = checksumManifest(f.archive, "dist/"+installerTestAsset)
			},
			want: "unsafe checksum filename",
		},
		{
			name: "missing checksum",
			mutate: func(_ *testing.T, f *installerFixture) {
				f.checksums = checksumManifest(f.archive, "another.zip")
			},
			want: "expected exactly one checksum",
		},
		{
			name: "duplicate checksum",
			mutate: func(_ *testing.T, f *installerFixture) {
				f.checksums = append(f.checksums, f.checksums...)
			},
			want: "checksums.txt must contain exactly four entries",
		},
		{
			name: "wrong checksum",
			mutate: func(_ *testing.T, f *installerFixture) {
				f.checksums = checksumManifest([]byte("different"), installerTestAsset)
			},
			want: "SHA-256 mismatch",
		},
		{
			name: "truncated ZIP after valid checksum",
			mutate: func(_ *testing.T, f *installerFixture) {
				f.archive = []byte("not a ZIP")
				f.checksums = checksumManifest(f.archive, installerTestAsset)
			},
			want: "cannot inspect release ZIP",
		},
		{
			name: "archive HTTP 404",
			mutate: func(_ *testing.T, f *installerFixture) {
				f.status = map[string]int{installerTestAsset: http.StatusNotFound}
			},
			want: "download failed",
		},
		{
			name: "empty archive",
			mutate: func(t *testing.T, f *installerFixture) {
				f.archive = installerZIP(t, nil)
				f.checksums = checksumManifest(f.archive, installerTestAsset)
			},
			want: "cannot inspect release ZIP",
		},
		{
			name: "generic executable",
			mutate: func(t *testing.T, f *installerFixture) {
				f.archive = installerZIP(t, []installerZIPEntry{{name: "gloss", mode: 0o755, data: installerExecutableData()}})
				f.checksums = checksumManifest(f.archive, installerTestAsset)
			},
			want: "ZIP must contain exactly one entry",
		},
		{
			name: "wrong platform executable",
			mutate: func(t *testing.T, f *installerFixture) {
				f.archive = installerZIP(t, []installerZIPEntry{{name: "gloss-linux-amd64", mode: 0o755, data: installerExecutableData()}})
				f.checksums = checksumManifest(f.archive, installerTestAsset)
			},
			want: "ZIP must contain exactly one entry",
		},
		{
			name: "extra ZIP entry",
			mutate: func(t *testing.T, f *installerFixture) {
				f.archive = installerZIP(t, []installerZIPEntry{
					{name: installerTestExecutable, mode: 0o755, data: installerExecutableData()},
					{name: "README.md", mode: 0o644, data: []byte("extra")},
				})
				f.checksums = checksumManifest(f.archive, installerTestAsset)
			},
			want: "ZIP must contain exactly one entry",
		},
		{
			name: "directory entry",
			mutate: func(t *testing.T, f *installerFixture) {
				f.archive = installerZIP(t, []installerZIPEntry{{name: installerTestExecutable + "/", mode: os.ModeDir | 0o755}})
				f.checksums = checksumManifest(f.archive, installerTestAsset)
			},
			want: "ZIP must contain exactly one entry",
		},
		{
			name: "nested path",
			mutate: func(t *testing.T, f *installerFixture) {
				f.archive = installerZIP(t, []installerZIPEntry{{name: "bin/" + installerTestExecutable, mode: 0o755, data: installerExecutableData()}})
				f.checksums = checksumManifest(f.archive, installerTestAsset)
			},
			want: "ZIP must contain exactly one entry",
		},
		{
			name: "traversal path",
			mutate: func(t *testing.T, f *installerFixture) {
				f.archive = installerZIP(t, []installerZIPEntry{{name: "../" + installerTestExecutable, mode: 0o755, data: installerExecutableData()}})
				f.checksums = checksumManifest(f.archive, installerTestAsset)
			},
			want: "ZIP must contain exactly one entry",
		},
		{
			name: "absolute path",
			mutate: func(t *testing.T, f *installerFixture) {
				f.archive = installerZIP(t, []installerZIPEntry{{name: "/" + installerTestExecutable, mode: 0o755, data: installerExecutableData()}})
				f.checksums = checksumManifest(f.archive, installerTestAsset)
			},
			want: "ZIP must contain exactly one entry",
		},
		{
			name: "symlink entry",
			mutate: func(t *testing.T, f *installerFixture) {
				f.archive = installerZIP(t, []installerZIPEntry{{name: installerTestExecutable, mode: os.ModeSymlink | 0o777, data: []byte("target")}})
				f.checksums = checksumManifest(f.archive, installerTestAsset)
			},
			want: "not a regular executable",
		},
		{
			name: "nonregular entry",
			mutate: func(t *testing.T, f *installerFixture) {
				f.archive = installerZIP(t, []installerZIPEntry{{name: installerTestExecutable, mode: os.ModeDevice | 0o755, data: []byte("device")}})
				f.checksums = checksumManifest(f.archive, installerTestAsset)
			},
			want: "not a regular executable",
		},
		{
			name: "nonexecutable entry",
			mutate: func(t *testing.T, f *installerFixture) {
				f.archive = installerZIP(t, []installerZIPEntry{{name: installerTestExecutable, mode: 0o644, data: installerExecutableData()}})
				f.checksums = checksumManifest(f.archive, installerTestAsset)
			},
			want: "not a regular executable",
		},
		{
			name: "empty executable",
			mutate: func(t *testing.T, f *installerFixture) {
				f.archive = installerZIP(t, []installerZIPEntry{{name: installerTestExecutable, mode: 0o755}})
				f.checksums = checksumManifest(f.archive, installerTestAsset)
			},
			want: "unsafe size",
		},
		{
			name: "missing checksum utility", extraEnv: []string{"GLOSS_CHECKSUM_TOOL=none"},
			want: "neither sha256sum nor shasum is available",
		},
		{
			name: "Homebrew destination",
			installDir: func(t *testing.T, root string) string {
				path := filepath.Join(root, "Cellar", "gloss", "0.1.1", "bin")
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
				return path
			},
			want: "refusing Homebrew-managed destination",
		},
		{
			name: "destination symlink",
			installDir: func(t *testing.T, root string) string {
				real := filepath.Join(root, "real-bin")
				if err := os.Mkdir(real, 0o700); err != nil {
					t.Fatal(err)
				}
				link := filepath.Join(root, "linked-bin")
				if err := os.Symlink(real, link); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				return link
			},
			want: "non-symlink directory",
		},
		{
			name: "symlinked parent",
			installDir: func(t *testing.T, root string) string {
				realParent := filepath.Join(root, "real-parent")
				path := filepath.Join(realParent, "bin")
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
				linkParent := filepath.Join(root, "linked-parent")
				if err := os.Symlink(realParent, linkParent); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				return filepath.Join(linkParent, "bin")
			},
			want: "symlinked parent",
		},
		{
			name: "existing nonregular target",
			installDir: func(t *testing.T, root string) string {
				path := filepath.Join(root, "bin")
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(path, "gloss"), 0o700); err != nil {
					t.Fatal(err)
				}
				return path
			},
			want: "not a regular file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := valid
			fixture.status = nil
			if tt.mutate != nil {
				tt.mutate(t, &fixture)
			}
			server := newInstallerServer(t, fixture)
			version := tt.version
			if version == "" {
				version = "v0.1.1"
			}
			result := runInstallerIntegrationWithOptions(t, server.URL, version, fixture, false, []byte("original"), tt.extraEnv, tt.unameS, tt.unameM, tt.installDir)
			if result.err == nil {
				t.Fatalf("installer unexpectedly succeeded:\n%s", result.output)
			}
			if !strings.Contains(result.output, tt.want) {
				t.Fatalf("error output %q does not contain %q", result.output, tt.want)
			}
			if result.target != "" {
				data, err := os.ReadFile(result.target)
				if err == nil && !bytes.Equal(data, result.original) {
					t.Fatalf("existing target changed: %q", data)
				}
			}
			assertNoInstallerTemps(t, result)
		})
	}
}

func TestInstallerRejectsUnwritableDestination(t *testing.T) {
	path, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o700) })
	output, err := runInstallerFunction(
		t,
		`validate_destination "$DESTINATION"`,
		"DESTINATION="+path,
		"HOME="+filepath.Dir(path),
	)
	if err == nil {
		t.Skip("current execution environment reports a mode-0500 directory as writable")
	}
	if !strings.Contains(output, "is not writable") {
		t.Fatalf("unwritable output = %q", output)
	}
}

func TestInstallerRejectsOversizedExecutable(t *testing.T) {
	data := make([]byte, maxBinary+1)
	archive := installerZIP(t, []installerZIPEntry{{
		name: installerTestExecutable,
		mode: 0o755,
		data: data,
	}})
	fixture := validInstallerFixture(t)
	fixture.archive = archive
	fixture.checksums = checksumManifest(archive, installerTestAsset)
	server := newInstallerServer(t, fixture)
	result := runInstallerIntegration(t, server.URL, "v0.1.1", fixture, false, []byte("original"))
	if result.err == nil || !strings.Contains(result.output, "unsafe size") {
		t.Fatalf("oversized result = %v, %q", result.err, result.output)
	}
	dataOnDisk, err := os.ReadFile(result.target)
	if err != nil || !bytes.Equal(dataOnDisk, result.original) {
		t.Fatalf("target after oversized archive = %q, %v", dataOnDisk, err)
	}
}

func TestInstallerVerifiesChecksumBeforeInspectingZIP(t *testing.T) {
	fixture := validInstallerFixture(t)
	fixture.archive = []byte("not a ZIP")
	fixture.checksums = checksumManifest([]byte("different"), installerTestAsset)
	server := newInstallerServer(t, fixture)
	result := runInstallerIntegration(t, server.URL, "v0.1.1", fixture, false, []byte("original"))
	if result.err == nil || !strings.Contains(result.output, "SHA-256 mismatch") ||
		strings.Contains(result.output, "cannot inspect release ZIP") {
		t.Fatalf("verification ordering output = %q, err=%v", result.output, result.err)
	}
}

func TestInstallerStagingFailurePreservesTarget(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "gloss")
	temporary := filepath.Join(dir, "temporary")
	if err := os.Mkdir(temporary, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, installerExecutableData(), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("original")
	if err := os.WriteFile(target, original, 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("bash", "-c", `
source "$INSTALL_SCRIPT"
temporary_dir="$TEST_DIR"
trap cleanup EXIT
cp() { return 1; }
install_atomically "$SOURCE" "$TARGET"
`)
	command.Env = append(os.Environ(),
		"INSTALL_SCRIPT="+filepath.Join(root, "scripts", "install.sh"),
		"TEST_DIR="+temporary,
		"SOURCE="+source,
		"TARGET="+target,
	)
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("injected staging failure succeeded: %q", output)
	}
	data, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatalf("target after staging failure = %q, %v", data, err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".gloss-install.*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("staged files after injected failure = %v, %v", matches, err)
	}
}

func TestInstallerRejectsUnsafeTestingReleaseRoot(t *testing.T) {
	for _, root := range []string{
		"http://example.com",
		"file:///tmp/releases",
		"ftp://127.0.0.1/releases",
	} {
		if _, err := runInstallerFunction(t, `safe_test_release_root "$ROOT"`, "ROOT="+root); err == nil {
			t.Fatalf("unsafe testing release root %q accepted", root)
		}
	}
}

func TestInstallerDetectsTargetReplacementRace(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "gloss")
	if err := os.WriteFile(source, installerExecutableData(), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("original")
	if err := os.WriteFile(target, original, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "identity-read")
	temporary := filepath.Join(dir, "temporary")
	if err := os.Mkdir(temporary, 0o700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("bash", "-c", `
source "$INSTALL_SCRIPT"
temporary_dir="$TEST_DIR"
trap cleanup EXIT
file_identity() {
  if [[ -e "$MARKER" ]]; then
    printf 'changed-identity\n'
  else
    : > "$MARKER"
    printf 'original-identity\n'
  fi
}
install_atomically "$SOURCE" "$TARGET"
`)
	command.Env = append(os.Environ(),
		"INSTALL_SCRIPT="+filepath.Join(root, "scripts", "install.sh"),
		"TEST_DIR="+temporary,
		"MARKER="+marker,
		"SOURCE="+source,
		"TARGET="+target,
	)
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "installation target changed") {
		t.Fatalf("race result = %v, %q", err, output)
	}
	data, readErr := os.ReadFile(target)
	if readErr != nil || !bytes.Equal(data, original) {
		t.Fatalf("target after race = %q, %v", data, readErr)
	}
	matches, globErr := filepath.Glob(filepath.Join(dir, ".gloss-install.*"))
	if globErr != nil || len(matches) != 0 {
		t.Fatalf("staged files after race = %v, %v", matches, globErr)
	}
}

type installerRunResult struct {
	output   string
	err      error
	root     string
	tempDir  string
	install  string
	target   string
	original []byte
}

func runInstallerIntegration(
	t *testing.T,
	releaseRoot, version string,
	fixture installerFixture,
	pathPresent bool,
	original []byte,
) installerRunResult {
	t.Helper()
	return runInstallerIntegrationWithOptions(t, releaseRoot, version, fixture, pathPresent, original, nil, "", "", nil)
}

func runInstallerIntegrationWithOptions(
	t *testing.T,
	releaseRoot, version string,
	_ installerFixture,
	pathPresent bool,
	original []byte,
	extraEnv []string,
	unameS, unameM string,
	installDir func(*testing.T, string) string,
) installerRunResult {
	t.Helper()
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	tempDir := filepath.Join(root, "tmp")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(tempDir, 0o700); err != nil {
		t.Fatal(err)
	}
	install := filepath.Join(root, "bin")
	if installDir != nil {
		install = installDir(t, root)
	} else if err := os.Mkdir(install, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(install, "gloss")
	if original != nil {
		if info, statErr := os.Stat(install); statErr == nil && info.IsDir() {
			if _, targetErr := os.Lstat(target); os.IsNotExist(targetErr) {
				if err := os.WriteFile(target, original, 0o755); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	systemPath := os.Getenv("PATH")
	if pathPresent {
		systemPath = install + string(os.PathListSeparator) + systemPath
	}
	if unameS == "" {
		unameS = "Darwin"
	}
	if unameM == "" {
		unameM = "x86_64"
	}
	command := exec.Command("bash", filepath.Join(repositoryRoot, "scripts", "install.sh"))
	command.Env = append(os.Environ(),
		"GLOSS_INSTALL_TESTING=1",
		"GLOSS_RELEASE_ROOT="+releaseRoot,
		"GLOSS_TEST_UNAME_S="+unameS,
		"GLOSS_TEST_UNAME_M="+unameM,
		"VERSION="+version,
		"HOME="+home,
		"TMPDIR="+tempDir,
		"INSTALL_DIR="+install,
		"PATH="+systemPath,
	)
	command.Env = append(command.Env, extraEnv...)
	output, runErr := command.CombinedOutput()
	return installerRunResult{
		output:   string(output),
		err:      runErr,
		root:     root,
		tempDir:  tempDir,
		install:  install,
		target:   target,
		original: original,
	}
}

func assertNoInstallerTemps(t *testing.T, result installerRunResult) {
	t.Helper()
	for _, pattern := range []string{
		filepath.Join(result.tempDir, "gloss-install.*"),
		filepath.Join(result.install, ".gloss-install.*"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 0 {
			t.Fatalf("installer temporary files remain: %v", matches)
		}
	}
}

func validInstallerFixture(t *testing.T) installerFixture {
	t.Helper()
	archive := installerZIP(t, []installerZIPEntry{{
		name: installerTestExecutable,
		mode: 0o755,
		data: installerExecutableData(),
	}})
	return installerFixture{
		tag:       "v0.1.1",
		archive:   archive,
		checksums: checksumManifest(archive, installerTestAsset),
	}
}

func newInstallerServer(t *testing.T, fixture installerFixture) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/releases/latest":
			http.Redirect(w, r, "http://"+r.Host+"/releases/tag/"+fixture.tag, http.StatusFound)
		case r.URL.Path == "/releases/tag/"+fixture.tag:
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/releases/download/"+fixture.tag+"/checksums.txt":
			if code := fixture.status["checksums.txt"]; code != 0 {
				http.Error(w, "fixture error", code)
				return
			}
			_, _ = w.Write(fixture.checksums)
		case r.URL.Path == "/releases/download/"+fixture.tag+"/"+installerTestAsset:
			if code := fixture.status[installerTestAsset]; code != 0 {
				http.Error(w, "fixture error", code)
				return
			}
			_, _ = w.Write(fixture.archive)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

type installerZIPEntry struct {
	name string
	mode os.FileMode
	data []byte
}

func installerZIP(t *testing.T, entries []installerZIPEntry) []byte {
	t.Helper()
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	for _, item := range entries {
		header := &zip.FileHeader{Name: item.name, Method: zip.Deflate}
		header.SetMode(item.mode)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(item.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func installerExecutableData() []byte {
	return []byte("#!/bin/sh\nprintf 'gloss 0.1.1\\n'\n")
}

func checksumManifest(archive []byte, expected string) []byte {
	digest := sha256.Sum256(archive)
	var body strings.Builder
	fmt.Fprintf(&body, "%x  %s\n", digest, expected)
	for _, name := range []string{
		"gloss-darwin-arm64.zip",
		"gloss-linux-amd64.zip",
		"gloss-linux-arm64.zip",
	} {
		fmt.Fprintf(&body, "%064x  %s\n", 0, name)
	}
	return []byte(body.String())
}
