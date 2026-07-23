// Package buildinfo resolves the version embedded in a Gloss binary.
package buildinfo

import (
	"fmt"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
)

const DevelopmentVersion = "dev"

const modulePath = "github.com/Architeg/gloss"

var stableVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// InjectedVersion is set only by tagged release builds:
//
//	go build -ldflags "-X github.com/Architeg/gloss/internal/buildinfo.InjectedVersion=v1.2.3"
var InjectedVersion string

// Version returns the canonical version for the running binary without a
// leading v. Untagged local builds return DevelopmentVersion.
func Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		info = nil
	}
	return Resolve(InjectedVersion, info)
}

// Resolve applies the release-injection, module-build-info, development
// fallback order. It is exported so tests can use synthetic build metadata
// without mutating process-global state.
func Resolve(injected string, info *debug.BuildInfo) string {
	if version, ok := NormalizeStable(injected); ok {
		return version
	}
	if info != nil && info.Main.Path == modulePath {
		if version, ok := NormalizeStable(info.Main.Version); ok {
			return version
		}
	}
	return DevelopmentVersion
}

// NormalizeStable accepts a stable three-component semantic version with an
// optional leading v and returns the canonical version without that prefix.
func NormalizeStable(value string) (string, bool) {
	if strings.HasPrefix(value, "v") {
		value = strings.TrimPrefix(value, "v")
	}
	if !stableVersionPattern.MatchString(value) {
		return "", false
	}
	for _, component := range strings.Split(value, ".") {
		if _, err := strconv.Atoi(component); err != nil {
			return "", false
		}
	}
	return value, true
}

// ValidateReleaseTag requires the exact stable release-tag form vX.Y.Z.
func ValidateReleaseTag(tag string) (string, error) {
	if !strings.HasPrefix(tag, "v") {
		return "", fmt.Errorf("release tag must match vMAJOR.MINOR.PATCH: %q", tag)
	}
	version, ok := NormalizeStable(tag)
	if !ok {
		return "", fmt.Errorf("release tag must match vMAJOR.MINOR.PATCH: %q", tag)
	}
	return version, nil
}

// Display adds the conventional v prefix to stable versions while leaving
// development labels unchanged.
func Display(version string) string {
	if stable, ok := NormalizeStable(version); ok {
		return "v" + stable
	}
	return version
}
