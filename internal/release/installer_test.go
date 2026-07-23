package release

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallerPlatformMapping(t *testing.T) {
	tests := []struct {
		system  string
		machine string
		want    string
	}{
		{system: "Darwin", machine: "x86_64", want: "darwin amd64 gloss-darwin-amd64.zip gloss-darwin-amd64"},
		{system: "Darwin", machine: "arm64", want: "darwin arm64 gloss-darwin-arm64.zip gloss-darwin-arm64"},
		{system: "Linux", machine: "amd64", want: "linux amd64 gloss-linux-amd64.zip gloss-linux-amd64"},
		{system: "Linux", machine: "aarch64", want: "linux arm64 gloss-linux-arm64.zip gloss-linux-arm64"},
	}
	for _, tt := range tests {
		t.Run(tt.system+"/"+tt.machine, func(t *testing.T) {
			got, err := runInstallerFunction(t, `detect_platform "$SYSTEM" "$MACHINE"`, "SYSTEM="+tt.system, "MACHINE="+tt.machine)
			if err != nil || strings.TrimSpace(got) != tt.want {
				t.Fatalf("mapping = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
	for _, values := range [][2]string{{"Windows", "amd64"}, {"Linux", "386"}} {
		if _, err := runInstallerFunction(t, `detect_platform "$SYSTEM" "$MACHINE"`, "SYSTEM="+values[0], "MACHINE="+values[1]); err == nil {
			t.Fatalf("unsupported mapping %v succeeded", values)
		}
	}
}

func TestInstallerVersionValidation(t *testing.T) {
	for _, value := range []string{"v0.1.1", "0.1.1"} {
		got, err := runInstallerFunction(t, `normalize_version "$VALUE"`, "VALUE="+value)
		if err != nil || strings.TrimSpace(got) != "v0.1.1" {
			t.Fatalf("normalize %q = %q, %v", value, got, err)
		}
	}
	for _, value := range []string{"v0.1", "v0.1.1-rc.1", "v0.1.1+build", "v01.1.1", "../v1.2.3", "release"} {
		if _, err := runInstallerFunction(t, `normalize_version "$VALUE"`, "VALUE="+value); err == nil {
			t.Fatalf("malformed version %q succeeded", value)
		}
	}
}

func TestInstallerChecksumLookup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checksums.txt")
	var body strings.Builder
	for _, target := range targets {
		digest := sha256.Sum256([]byte(target.Archive))
		fmt.Fprintf(&body, "%x  %s\n", digest, target.Archive)
	}
	if err := os.WriteFile(path, []byte(body.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := runInstallerFunction(t, `lookup_checksum "$FILE" "$ASSET"`, "FILE="+path, "ASSET="+targets[0].Archive)
	if err != nil || len(strings.TrimSpace(got)) != 64 {
		t.Fatalf("lookup = %q, %v", got, err)
	}

	duplicate := body.String() + strings.Split(body.String(), "\n")[0] + "\n"
	if err := os.WriteFile(path, []byte(duplicate), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runInstallerFunction(t, `lookup_checksum "$FILE" "$ASSET"`, "FILE="+path, "ASSET="+targets[0].Archive); err == nil {
		t.Fatal("duplicate checksum accepted")
	}
	if err := os.WriteFile(path, []byte(strings.Replace(body.String(), targets[0].Archive, "dist/"+targets[0].Archive, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runInstallerFunction(t, `lookup_checksum "$FILE" "$ASSET"`, "FILE="+path, "ASSET="+targets[0].Archive); err == nil {
		t.Fatal("unsafe checksum path accepted")
	}
	missing := strings.Replace(body.String(), targets[0].Archive, "another.zip", 1)
	if err := os.WriteFile(path, []byte(missing), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runInstallerFunction(t, `lookup_checksum "$FILE" "$ASSET"`, "FILE="+path, "ASSET="+targets[0].Archive); err == nil {
		t.Fatal("missing checksum accepted")
	}
	firstDigest := strings.Fields(body.String())[0]
	malformed := strings.Replace(body.String(), firstDigest, "not-a-digest", 1)
	if err := os.WriteFile(path, []byte(malformed), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runInstallerFunction(t, `lookup_checksum "$FILE" "$ASSET"`, "FILE="+path, "ASSET="+targets[0].Archive); err == nil {
		t.Fatal("malformed checksum accepted")
	}
}

func TestInstallerRejectsUnsafeDestinations(t *testing.T) {
	for _, path := range []string{"/opt/homebrew/Cellar/gloss/0.1.1/bin", "/usr/local/opt/gloss/bin"} {
		if _, err := runInstallerFunction(t, `is_homebrew_path "$VALUE"`, "VALUE="+path); err != nil {
			t.Fatalf("Homebrew path %q was not detected", path)
		}
	}
	if _, err := runInstallerFunction(t, `is_protected_path /usr/bin/gloss`); err != nil {
		t.Fatal("protected path was not detected")
	}
}

func runInstallerFunction(t *testing.T, expression string, environment ...string) (string, error) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "scripts", "install.sh")
	command := newInstallerTestCommand(t, false, "bash", "-c", `source "$INSTALL_SCRIPT"; `+expression)
	command.Env = append(os.Environ(), append(environment, "INSTALL_SCRIPT="+script)...)
	output, err := command.CombinedOutput()
	return string(output), err
}
