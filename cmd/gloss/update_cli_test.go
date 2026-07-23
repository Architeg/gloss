package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Architeg/gloss/internal/update"
)

type fakeUpdateClient struct {
	result       update.CheckResult
	checkErr     error
	verified     update.VerifiedUpdate
	downloadErr  error
	downloadCall int
	current      string
}

func (f *fakeUpdateClient) Check(_ context.Context, current string) (update.CheckResult, error) {
	f.current = current
	return f.result, f.checkErr
}

func (f *fakeUpdateClient) DownloadVerified(context.Context, update.Release) (update.VerifiedUpdate, error) {
	f.downloadCall++
	return f.verified, f.downloadErr
}

func TestRunUpdateCLICheckNeverInstalls(t *testing.T) {
	client := &fakeUpdateClient{result: update.CheckResult{
		CurrentValid:      true,
		LatestVersion:     "0.2.0",
		UpdateAvailable:   true,
		PlatformSupported: true,
	}}
	var installed bool
	var out bytes.Buffer
	err := runUpdateCLI(context.Background(), &out, false, "0.1.0", client,
		func() (update.Layout, error) { return update.Layout{Kind: update.LayoutManual}, nil },
		func(update.Layout, update.VerifiedUpdate) error {
			installed = true
			return nil
		},
	)
	if err != nil || installed || client.downloadCall != 0 {
		t.Fatalf("check result: err=%v installed=%v downloads=%d", err, installed, client.downloadCall)
	}
	if client.current != "0.1.0" {
		t.Fatalf("updater received current version %q", client.current)
	}
	if !strings.Contains(out.String(), "Current version: 0.1.0") ||
		!strings.Contains(out.String(), "Latest stable version: 0.2.0") ||
		!strings.Contains(out.String(), "Run: gloss update --install") {
		t.Fatalf("check output = %q", out.String())
	}
}

func TestRunUpdateCLIInstall(t *testing.T) {
	platform, _ := update.PlatformFor("darwin", "amd64")
	verified := update.VerifiedUpdate{
		Version:        "0.2.0",
		ExecutableName: platform.Executable,
		Data:           []byte("new"),
		Platform:       platform,
	}
	client := &fakeUpdateClient{
		result: update.CheckResult{
			CurrentValid:      true,
			LatestVersion:     "0.2.0",
			UpdateAvailable:   true,
			PlatformSupported: true,
		},
		verified: verified,
	}
	var installed update.VerifiedUpdate
	var out bytes.Buffer
	err := runUpdateCLI(context.Background(), &out, true, "0.1.0", client,
		func() (update.Layout, error) {
			return update.Layout{Kind: update.LayoutManual, Platform: platform}, nil
		},
		func(_ update.Layout, candidate update.VerifiedUpdate) error {
			installed = candidate
			return nil
		},
	)
	if err != nil || client.downloadCall != 1 || installed.Version != "0.2.0" {
		t.Fatalf("install result: err=%v downloads=%d installed=%#v", err, client.downloadCall, installed)
	}
	if !strings.Contains(out.String(), "Installed Gloss 0.2.0.") || !strings.Contains(out.String(), "Rerun Gloss") {
		t.Fatalf("install output = %q", out.String())
	}
}

func TestRunUpdateCLIWithLocalVerifiedRelease(t *testing.T) {
	platform, _ := update.PlatformFor("darwin", "amd64")
	executable := []byte("#!/bin/sh\nprintf 'gloss 0.1.1\\n'\n")
	archive := commandUpdateZIP(t, platform.Executable, executable)
	digest := sha256.Sum256(archive)
	checksums := []byte(fmt.Sprintf("%x  %s\n", digest, platform.Archive))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"tag_name": "v0.1.1",
				"assets": []map[string]string{
					{"name": platform.Archive, "browser_download_url": "http://" + r.Host + "/archive.zip"},
					{"name": "checksums.txt", "browser_download_url": "http://" + r.Host + "/checksums.txt"},
				},
			}})
		case "/archive.zip":
			_, _ = w.Write(archive)
		case "/checksums.txt":
			_, _ = w.Write(checksums)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := update.NewClient(server.Client())
	client.ReleasesURL = server.URL + "/releases"
	client.GOOS, client.GOARCH = platform.GOOS, platform.GOARCH
	target := filepath.Join(t.TempDir(), "gloss")
	original := []byte("old executable")
	if err := os.WriteFile(target, original, 0o755); err != nil {
		t.Fatal(err)
	}
	inspect := func() (update.Layout, error) {
		return update.InspectExecutable(target, platform.GOOS, platform.GOARCH)
	}

	var checkOutput bytes.Buffer
	if err := runUpdateCLI(
		context.Background(),
		&checkOutput,
		false,
		"0.1.0",
		client,
		inspect,
		update.InstallVerified,
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(checkOutput.String(), "Latest stable version: 0.1.1") ||
		!strings.Contains(checkOutput.String(), "Run: gloss update --install") {
		t.Fatalf("check output = %q", checkOutput.String())
	}
	if data, err := os.ReadFile(target); err != nil || !bytes.Equal(data, original) {
		t.Fatalf("check-only target = %q, %v", data, err)
	}

	var installOutput bytes.Buffer
	if err := runUpdateCLI(
		context.Background(),
		&installOutput,
		true,
		"0.1.0",
		client,
		inspect,
		update.InstallVerified,
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(installOutput.String(), "Installed Gloss 0.1.1.") {
		t.Fatalf("install output = %q", installOutput.String())
	}
	output, err := exec.Command(target, "version").CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "gloss 0.1.1" {
		t.Fatalf("installed fixture = %q, %v", output, err)
	}
}

func TestRunUpdateCLIAlreadyCurrent(t *testing.T) {
	client := &fakeUpdateClient{result: update.CheckResult{CurrentValid: true, LatestVersion: "0.1.0"}}
	var out bytes.Buffer
	if err := runUpdateCLI(context.Background(), &out, true, "0.1.0", client,
		func() (update.Layout, error) { t.Fatal("inspect called"); return update.Layout{}, nil },
		func(update.Layout, update.VerifiedUpdate) error { t.Fatal("replace called"); return nil },
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "up to date") || client.downloadCall != 0 {
		t.Fatalf("current output = %q, downloads=%d", out.String(), client.downloadCall)
	}
}

func commandUpdateZIP(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o755)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func TestRunUpdateCLIHomebrewGuidance(t *testing.T) {
	client := &fakeUpdateClient{result: update.CheckResult{
		CurrentValid: true, LatestVersion: "0.2.0", UpdateAvailable: true, PlatformSupported: true,
	}}
	for _, install := range []bool{false, true} {
		var out bytes.Buffer
		err := runUpdateCLI(context.Background(), &out, install, "0.1.0", client,
			func() (update.Layout, error) {
				return update.Layout{Kind: update.LayoutHomebrew}, &update.HomebrewError{Path: "/opt/homebrew/Cellar/gloss"}
			},
			func(update.Layout, update.VerifiedUpdate) error { t.Fatal("replace called"); return nil },
		)
		if !strings.Contains(out.String(), update.HomebrewUpgradeCommand) {
			t.Fatalf("Homebrew output = %q", out.String())
		}
		if install && err == nil {
			t.Fatal("Homebrew install succeeded")
		}
		if !install && err != nil {
			t.Fatalf("Homebrew check error = %v", err)
		}
	}
}

func TestRunUpdateCLIRejectsUnsafeCases(t *testing.T) {
	tests := []struct {
		name   string
		result update.CheckResult
		err    error
	}{
		{name: "unsupported", result: update.CheckResult{LatestVersion: "0.2.0", UpdateAvailable: true}},
		{name: "development", result: update.CheckResult{LatestVersion: "0.2.0", UpdateAvailable: true, PlatformSupported: true}},
		{name: "layout", result: update.CheckResult{CurrentValid: true, LatestVersion: "0.2.0", UpdateAvailable: true, PlatformSupported: true}, err: errors.New("unsafe layout")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeUpdateClient{result: tt.result}
			err := runUpdateCLI(context.Background(), &bytes.Buffer{}, true, "0.1.0", client,
				func() (update.Layout, error) { return update.Layout{}, tt.err },
				func(update.Layout, update.VerifiedUpdate) error { t.Fatal("replace called"); return nil },
			)
			if err == nil || client.downloadCall != 0 {
				t.Fatalf("unsafe result: err=%v downloads=%d", err, client.downloadCall)
			}
		})
	}
}
