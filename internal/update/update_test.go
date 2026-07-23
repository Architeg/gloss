package update

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseVersionAndComparison(t *testing.T) {
	for _, value := range []string{"1.2.3", "v1.2.3"} {
		got, err := ParseVersion(value)
		if err != nil || got.String() != "1.2.3" {
			t.Fatalf("ParseVersion(%q) = %v, %v", value, got, err)
		}
	}
	for _, value := range []string{"", "dev", "1.2", "1.2.3-beta", "01.2.3", "v"} {
		if _, err := ParseVersion(value); err == nil {
			t.Fatalf("ParseVersion(%q) succeeded", value)
		}
	}
	if compareVersions(Version{1, 2, 4}, Version{1, 2, 3}) <= 0 {
		t.Fatal("newer patch did not compare greater")
	}
}

func TestPlatformForExactMappings(t *testing.T) {
	tests := []struct {
		goos, goarch, archive, executable string
	}{
		{"darwin", "amd64", "gloss-darwin-amd64.zip", "gloss-darwin-amd64"},
		{"darwin", "arm64", "gloss-darwin-arm64.zip", "gloss-darwin-arm64"},
		{"linux", "amd64", "gloss-linux-amd64.zip", "gloss-linux-amd64"},
		{"linux", "arm64", "gloss-linux-arm64.zip", "gloss-linux-arm64"},
	}
	for _, tt := range tests {
		p, err := PlatformFor(tt.goos, tt.goarch)
		if err != nil || p.Archive != tt.archive || p.Executable != tt.executable {
			t.Fatalf("PlatformFor(%s/%s) = %#v, %v", tt.goos, tt.goarch, p, err)
		}
	}
	for _, target := range [][2]string{{"windows", "amd64"}, {"linux", "386"}, {"darwin", "386"}} {
		if _, err := PlatformFor(target[0], target[1]); !errors.Is(err, ErrUnsupportedPlatform) {
			t.Fatalf("PlatformFor(%s/%s) error = %v", target[0], target[1], err)
		}
	}
}

func TestClientCheckSelectsLatestStableAndExactAssets(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent")
		}
		releases := []githubRelease{
			{TagName: "v9.0.0", Draft: true},
			{TagName: "v8.0.0", Prerelease: true},
			{TagName: "v1.2.0", Assets: releaseAssets(server.URL, "linux", "amd64")},
			{TagName: "v1.3.0", Assets: releaseAssets(server.URL, "linux", "amd64")},
		}
		_ = json.NewEncoder(w).Encode(releases)
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.ReleasesURL = server.URL
	client.GOOS, client.GOARCH = "linux", "amd64"
	for _, tt := range []struct {
		current   string
		available bool
		valid     bool
	}{
		{"1.2.9", true, true},
		{"1.3.0", false, true},
		{"2.0.0", false, true},
		{"dev", true, false},
	} {
		result, err := client.Check(context.Background(), tt.current)
		if err != nil {
			t.Fatal(err)
		}
		if result.LatestVersion != "1.3.0" || result.UpdateAvailable != tt.available || result.CurrentValid != tt.valid {
			t.Fatalf("current %q result = %#v", tt.current, result)
		}
		if result.Release.Archive.Name != "gloss-linux-amd64.zip" || result.Release.Checksums.Name != "checksums.txt" {
			t.Fatalf("selected assets = %#v", result.Release)
		}
	}
}

func TestClientCheckRejectsMalformedAndDuplicateAssets(t *testing.T) {
	tests := []struct {
		name     string
		releases []githubRelease
	}{
		{name: "malformed tag", releases: []githubRelease{{TagName: "latest"}}},
		{name: "no stable", releases: []githubRelease{{TagName: "v1.0.0", Prerelease: true}}},
		{name: "missing archive", releases: []githubRelease{{TagName: "v1.0.0", Assets: []githubAsset{{Name: "checksums.txt", URL: "https://example.test/checksums"}}}}},
		{name: "duplicate archive", releases: []githubRelease{{TagName: "v1.0.0", Assets: []githubAsset{
			{Name: "checksums.txt", URL: "https://example.test/checksums"},
			{Name: "gloss-linux-amd64.zip", URL: "https://example.test/one"},
			{Name: "gloss-linux-amd64.zip", URL: "https://example.test/two"},
		}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(tt.releases)
			}))
			defer server.Close()
			client := NewClient(server.Client())
			client.ReleasesURL = server.URL
			client.GOOS, client.GOARCH = "linux", "amd64"
			if _, err := client.Check(context.Background(), "0.1.0"); err == nil {
				t.Fatal("Check succeeded")
			}
		})
	}
}

func TestClientCheckUnsupportedPlatformStillReportsRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]githubRelease{{TagName: "v1.0.0"}})
	}))
	defer server.Close()
	client := NewClient(server.Client())
	client.ReleasesURL = server.URL
	client.GOOS, client.GOARCH = "windows", "amd64"
	result, err := client.Check(context.Background(), "0.1.0")
	if err != nil || result.PlatformSupported || !result.UpdateAvailable || result.LatestVersion != "1.0.0" {
		t.Fatalf("unsupported result = %#v, %v", result, err)
	}
}

func TestDownloadVerifiedChecksDigestBeforeArchiveValidation(t *testing.T) {
	platform, _ := PlatformFor("linux", "amd64")
	goodArchive := makeZIP(t, platform.Executable, []byte("new executable"), 0o755)
	goodDigest := sha256.Sum256(goodArchive)
	var archiveBody = goodArchive
	var checksumBody = []byte(hex.EncodeToString(goodDigest[:]) + "  " + platform.Archive + "\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/checksums.txt":
			_, _ = w.Write(checksumBody)
		case "/archive.zip":
			_, _ = w.Write(archiveBody)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewClient(server.Client())
	release := Release{
		Version:   Version{1, 2, 3},
		Platform:  platform,
		Archive:   Asset{Name: platform.Archive, URL: server.URL + "/archive.zip"},
		Checksums: Asset{Name: "checksums.txt", URL: server.URL + "/checksums.txt"},
	}
	verified, err := client.DownloadVerified(context.Background(), release)
	if err != nil || string(verified.Data) != "new executable" || verified.Version != "1.2.3" {
		t.Fatalf("verified = %#v, %v", verified, err)
	}

	archiveBody = []byte("not a ZIP")
	checksumBody = []byte(strings.Repeat("0", 64) + "  " + platform.Archive + "\n")
	if _, err := client.DownloadVerified(context.Background(), release); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("bad checksum error = %v", err)
	}
}

func TestParseChecksums(t *testing.T) {
	expected := "gloss-linux-amd64.zip"
	digest := strings.Repeat("a", 64)
	for _, line := range []string{digest + "  " + expected, digest + " *" + expected} {
		got, err := ParseChecksums([]byte(line+"\n"), expected)
		if err != nil || hex.EncodeToString(got) != digest {
			t.Fatalf("ParseChecksums(%q) = %x, %v", line, got, err)
		}
	}
	for _, data := range []string{
		strings.Repeat("a", 64) + "  other.zip\n",
		digest + "  " + expected + "\n" + digest + "  " + expected + "\n",
		"bad line\n",
		strings.Repeat("g", 64) + "  " + expected + "\n",
		strings.Repeat("a", 63) + "  " + expected + "\n",
		digest + "  ../" + expected + "\n",
		digest + "  /tmp/" + expected + "\n",
		digest + "  dir/" + expected + "\n",
	} {
		if _, err := ParseChecksums([]byte(data), expected); err == nil {
			t.Fatalf("ParseChecksums accepted %q", data)
		}
	}
}

func TestValidateArchiveContract(t *testing.T) {
	const expected = "gloss-linux-amd64"
	valid := makeZIP(t, expected, []byte("binary"), 0o755)
	if got, err := ValidateArchive(valid, expected, 1024); err != nil || string(got) != "binary" {
		t.Fatalf("ValidateArchive(valid) = %q, %v", got, err)
	}
	tests := []struct {
		name string
		data []byte
		max  uint64
	}{
		{"empty", makeZIP(t, expected, nil, 0o755), 1024},
		{"generic name", makeZIP(t, "gloss", []byte("x"), 0o755), 1024},
		{"nested", makeZIP(t, "dir/"+expected, []byte("x"), 0o755), 1024},
		{"traversal", makeZIP(t, "../"+expected, []byte("x"), 0o755), 1024},
		{"absolute", makeZIP(t, "/"+expected, []byte("x"), 0o755), 1024},
		{"directory", makeZIP(t, expected+"/", nil, os.ModeDir|0o755), 1024},
		{"symlink", makeZIP(t, expected, []byte("target"), os.ModeSymlink|0o777), 1024},
		{"not executable", makeZIP(t, expected, []byte("x"), 0o644), 1024},
		{"oversized", makeZIP(t, expected, []byte("too large"), 0o755), 2},
		{"extra", makeZIPEntries(t, []zipEntry{{expected, []byte("x"), 0o755}, {"README", []byte("x"), 0o644}}), 1024},
		{"duplicate", makeZIPEntries(t, []zipEntry{{expected, []byte("x"), 0o755}, {expected, []byte("x"), 0o755}}), 1024},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ValidateArchive(tt.data, expected, tt.max); err == nil {
				t.Fatal("ValidateArchive succeeded")
			}
		})
	}
}

func TestHTTPFailuresAreBoundedAndContextAware(t *testing.T) {
	t.Run("non-2xx", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "no", http.StatusTooManyRequests)
		}))
		defer server.Close()
		client := NewClient(server.Client())
		client.ReleasesURL = server.URL
		if _, err := client.Check(context.Background(), "0.1.0"); err == nil {
			t.Fatal("non-2xx succeeded")
		}
	})
	t.Run("oversized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("x", 32)))
		}))
		defer server.Close()
		client := NewClient(server.Client())
		client.ReleasesURL = server.URL
		client.ReleaseMax = 8
		if _, err := client.Check(context.Background(), "0.1.0"); err == nil {
			t.Fatal("oversized response succeeded")
		}
	})
	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		client := NewClient(http.DefaultClient)
		client.ReleasesURL = "https://example.invalid/releases"
		if _, err := client.Check(ctx, "0.1.0"); err == nil {
			t.Fatal("canceled request succeeded")
		}
	})
	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		defer server.Close()
		client := NewClient(&http.Client{Timeout: 10 * time.Millisecond})
		client.ReleasesURL = server.URL
		if _, err := client.Check(context.Background(), "0.1.0"); err == nil {
			t.Fatal("timed out request succeeded")
		}
	})
}

func releaseAssets(baseURL, goos, goarch string) []githubAsset {
	p, _ := PlatformFor(goos, goarch)
	return []githubAsset{
		{Name: p.Archive, URL: baseURL + "/" + p.Archive},
		{Name: "checksums.txt", URL: baseURL + "/checksums.txt"},
	}
}

type zipEntry struct {
	name string
	data []byte
	mode os.FileMode
}

func makeZIP(t *testing.T, name string, data []byte, mode os.FileMode) []byte {
	t.Helper()
	return makeZIPEntries(t, []zipEntry{{name: name, data: data, mode: mode}})
}

func makeZIPEntries(t *testing.T, entries []zipEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		header.SetMode(entry.mode)
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
