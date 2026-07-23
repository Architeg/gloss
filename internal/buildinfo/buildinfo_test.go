package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersionOrder(t *testing.T) {
	module := &debug.BuildInfo{Main: debug.Module{Path: modulePath, Version: "v2.3.4"}}
	tests := []struct {
		name     string
		injected string
		info     *debug.BuildInfo
		want     string
	}{
		{name: "injected", injected: "1.2.3", info: module, want: "1.2.3"},
		{name: "injected tag", injected: "v1.2.3", info: module, want: "1.2.3"},
		{name: "module fallback", info: module, want: "2.3.4"},
		{name: "malformed injected falls back", injected: "release", info: module, want: "2.3.4"},
		{name: "wrong module", info: &debug.BuildInfo{Main: debug.Module{Path: "example.com/other", Version: "v2.3.4"}}, want: DevelopmentVersion},
		{name: "devel module", info: &debug.BuildInfo{Main: debug.Module{Path: modulePath, Version: "(devel)"}}, want: DevelopmentVersion},
		{name: "local fallback", want: DevelopmentVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Resolve(tt.injected, tt.info); got != tt.want {
				t.Fatalf("Resolve(%q, %#v) = %q, want %q", tt.injected, tt.info, got, tt.want)
			}
		})
	}
}

func TestValidateReleaseTag(t *testing.T) {
	if got, err := ValidateReleaseTag("v0.1.1"); err != nil || got != "0.1.1" {
		t.Fatalf("ValidateReleaseTag(valid) = %q, %v", got, err)
	}
	for _, value := range []string{
		"0.1.1",
		"v0.1",
		"v0.1.1-rc.1",
		"v0.1.1+build",
		"v01.1.1",
		"release",
		"",
	} {
		t.Run(value, func(t *testing.T) {
			if _, err := ValidateReleaseTag(value); err == nil {
				t.Fatalf("ValidateReleaseTag(%q) succeeded", value)
			}
		})
	}
}

func TestDisplay(t *testing.T) {
	if got := Display("1.2.3"); got != "v1.2.3" {
		t.Fatalf("Display(stable) = %q", got)
	}
	if got := Display(DevelopmentVersion); got != DevelopmentVersion {
		t.Fatalf("Display(dev) = %q", got)
	}
}
