package main

import (
	"bytes"
	"context"
	"errors"
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
