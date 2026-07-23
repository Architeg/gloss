// Package update implements release discovery and secure self-update primitives.
package update

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	DefaultReleasesURL = "https://api.github.com/repos/Architeg/gloss/releases?per_page=20"
	DefaultUserAgent   = "gloss-updater"
	defaultReleaseMax  = 2 << 20
	defaultChecksumMax = 1 << 20
	defaultArchiveMax  = 64 << 20
	defaultBinaryMax   = 64 << 20
)

var ErrUnsupportedPlatform = errors.New("unsupported update platform")

// Platform is the exact release-asset contract for one supported target.
type Platform struct {
	GOOS       string
	GOARCH     string
	Archive    string
	Executable string
}

var supportedPlatforms = map[string]Platform{
	"darwin/amd64": {GOOS: "darwin", GOARCH: "amd64", Archive: "gloss-darwin-amd64.zip", Executable: "gloss-darwin-amd64"},
	"darwin/arm64": {GOOS: "darwin", GOARCH: "arm64", Archive: "gloss-darwin-arm64.zip", Executable: "gloss-darwin-arm64"},
	"linux/amd64":  {GOOS: "linux", GOARCH: "amd64", Archive: "gloss-linux-amd64.zip", Executable: "gloss-linux-amd64"},
	"linux/arm64":  {GOOS: "linux", GOARCH: "arm64", Archive: "gloss-linux-arm64.zip", Executable: "gloss-linux-arm64"},
}

// PlatformFor returns the exact asset mapping for a supported target.
func PlatformFor(goos, goarch string) (Platform, error) {
	p, ok := supportedPlatforms[goos+"/"+goarch]
	if !ok {
		return Platform{}, fmt.Errorf("%w: %s/%s", ErrUnsupportedPlatform, goos, goarch)
	}
	return p, nil
}

// Version is a stable three-component semantic version.
type Version struct {
	Major int
	Minor int
	Patch int
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// ParseVersion accepts an optional leading v and rejects prerelease,
// build-metadata, and incomplete versions.
func ParseVersion(value string) (Version, error) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("invalid release version %q", value)
	}
	values := [3]int{}
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return Version{}, fmt.Errorf("invalid release version %q", value)
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return Version{}, fmt.Errorf("invalid release version %q", value)
		}
		values[i] = n
	}
	return Version{Major: values[0], Minor: values[1], Patch: values[2]}, nil
}

func compareVersions(a, b Version) int {
	switch {
	case a.Major != b.Major:
		return compareInt(a.Major, b.Major)
	case a.Minor != b.Minor:
		return compareInt(a.Minor, b.Minor)
	default:
		return compareInt(a.Patch, b.Patch)
	}
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// Asset is a release asset selected by exact name.
type Asset struct {
	Name string
	URL  string
}

// Release is the selected stable release and its required assets.
type Release struct {
	Tag       string
	Version   Version
	Platform  Platform
	Archive   Asset
	Checksums Asset
}

// CheckResult describes a stable release relative to the running version.
type CheckResult struct {
	CurrentVersion    string
	CurrentValid      bool
	LatestVersion     string
	UpdateAvailable   bool
	PlatformSupported bool
	Release           Release
}

// VerifiedUpdate contains an archive payload only after checksum and ZIP
// validation have both succeeded.
type VerifiedUpdate struct {
	Version        string
	ExecutableName string
	Data           []byte
	Platform       Platform
}

// Client performs update HTTP operations. Fields are injectable for tests.
type Client struct {
	HTTP          *http.Client
	ReleasesURL   string
	UserAgent     string
	GOOS          string
	GOARCH        string
	ReleaseMax    int64
	ChecksumMax   int64
	ArchiveMax    int64
	ExecutableMax uint64
}

// NewClient returns a client for the running platform.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		HTTP:          httpClient,
		ReleasesURL:   DefaultReleasesURL,
		UserAgent:     DefaultUserAgent,
		GOOS:          runtime.GOOS,
		GOARCH:        runtime.GOARCH,
		ReleaseMax:    defaultReleaseMax,
		ChecksumMax:   defaultChecksumMax,
		ArchiveMax:    defaultArchiveMax,
		ExecutableMax: defaultBinaryMax,
	}
}

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Check discovers the highest stable release returned by the configured API.
func (c *Client) Check(ctx context.Context, current string) (CheckResult, error) {
	body, err := c.get(ctx, c.ReleasesURL, c.limit(c.ReleaseMax, defaultReleaseMax))
	if err != nil {
		return CheckResult{}, fmt.Errorf("release lookup: %w", err)
	}
	var releases []githubRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		var single githubRelease
		if singleErr := json.Unmarshal(body, &single); singleErr != nil {
			return CheckResult{}, fmt.Errorf("decode releases: %w", err)
		}
		releases = []githubRelease{single}
	}

	var selected *githubRelease
	var selectedVersion Version
	for i := range releases {
		release := &releases[i]
		if release.Draft || release.Prerelease {
			continue
		}
		version, err := ParseVersion(release.TagName)
		if err != nil {
			return CheckResult{}, fmt.Errorf("release tag: %w", err)
		}
		if selected == nil || compareVersions(version, selectedVersion) > 0 {
			selected = release
			selectedVersion = version
		}
	}
	if selected == nil {
		return CheckResult{}, errors.New("no stable release found")
	}

	result := CheckResult{
		CurrentVersion: current,
		LatestVersion:  selectedVersion.String(),
	}
	currentVersion, currentErr := ParseVersion(current)
	result.CurrentValid = currentErr == nil
	result.UpdateAvailable = currentErr != nil || compareVersions(selectedVersion, currentVersion) > 0

	platform, platformErr := PlatformFor(c.GOOS, c.GOARCH)
	if platformErr != nil {
		return result, nil
	}
	result.PlatformSupported = true
	archive, checksums, err := selectAssets(selected.Assets, platform)
	if err != nil {
		return CheckResult{}, err
	}
	result.Release = Release{
		Tag:       selected.TagName,
		Version:   selectedVersion,
		Platform:  platform,
		Archive:   archive,
		Checksums: checksums,
	}
	return result, nil
}

func selectAssets(assets []githubAsset, platform Platform) (Asset, Asset, error) {
	var archives, checksums []Asset
	for _, asset := range assets {
		switch asset.Name {
		case platform.Archive:
			archives = append(archives, Asset{Name: asset.Name, URL: asset.URL})
		case "checksums.txt":
			checksums = append(checksums, Asset{Name: asset.Name, URL: asset.URL})
		}
	}
	if len(archives) != 1 {
		return Asset{}, Asset{}, fmt.Errorf("release requires exactly one %s asset (found %d)", platform.Archive, len(archives))
	}
	if len(checksums) != 1 {
		return Asset{}, Asset{}, fmt.Errorf("release requires exactly one checksums.txt asset (found %d)", len(checksums))
	}
	if err := validateDownloadURL(archives[0].URL); err != nil {
		return Asset{}, Asset{}, fmt.Errorf("archive URL: %w", err)
	}
	if err := validateDownloadURL(checksums[0].URL); err != nil {
		return Asset{}, Asset{}, fmt.Errorf("checksums URL: %w", err)
	}
	return archives[0], checksums[0], nil
}

func validateDownloadURL(value string) error {
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return fmt.Errorf("invalid download URL %q", value)
	}
	if u.Scheme == "http" && !isLoopbackHost(u.Hostname()) {
		return fmt.Errorf("insecure download URL %q", value)
	}
	return nil
}

func isLoopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// DownloadVerified downloads checksums and the platform archive, verifies the
// archive digest, then validates and reads its sole executable.
func (c *Client) DownloadVerified(ctx context.Context, release Release) (VerifiedUpdate, error) {
	if release.Archive.Name != release.Platform.Archive {
		return VerifiedUpdate{}, fmt.Errorf("unexpected archive asset %q", release.Archive.Name)
	}
	if release.Checksums.Name != "checksums.txt" {
		return VerifiedUpdate{}, fmt.Errorf("unexpected checksum asset %q", release.Checksums.Name)
	}
	checksumBody, err := c.get(ctx, release.Checksums.URL, c.limit(c.ChecksumMax, defaultChecksumMax))
	if err != nil {
		return VerifiedUpdate{}, fmt.Errorf("download checksums: %w", err)
	}
	expectedDigest, err := ParseChecksums(checksumBody, release.Platform.Archive)
	if err != nil {
		return VerifiedUpdate{}, err
	}
	archiveBody, err := c.get(ctx, release.Archive.URL, c.limit(c.ArchiveMax, defaultArchiveMax))
	if err != nil {
		return VerifiedUpdate{}, fmt.Errorf("download archive: %w", err)
	}
	actualDigest := sha256.Sum256(archiveBody)
	if !bytes.Equal(actualDigest[:], expectedDigest) {
		return VerifiedUpdate{}, fmt.Errorf("checksum mismatch for %s", release.Platform.Archive)
	}
	data, err := ValidateArchive(archiveBody, release.Platform.Executable, c.executableLimit())
	if err != nil {
		return VerifiedUpdate{}, err
	}
	return VerifiedUpdate{
		Version:        release.Version.String(),
		ExecutableName: release.Platform.Executable,
		Data:           data,
		Platform:       release.Platform,
	}, nil
}

func (c *Client) get(ctx context.Context, endpoint string, max int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	userAgent := c.UserAgent
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.Request != nil && resp.Request.URL != nil &&
		resp.Request.URL.Scheme == "http" && !isLoopbackHost(resp.Request.URL.Hostname()) {
		return nil, errors.New("insecure HTTP response URL")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	if resp.ContentLength > max {
		return nil, fmt.Errorf("response exceeds %d bytes", max)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > max {
		return nil, fmt.Errorf("response exceeds %d bytes", max)
	}
	if len(body) == 0 {
		return nil, errors.New("empty response")
	}
	return body, nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP == nil {
		return http.DefaultClient
	}
	return c.HTTP
}

func (c *Client) limit(value, fallback int64) int64 {
	if value <= 0 {
		return fallback
	}
	return value
}

func (c *Client) executableLimit() uint64 {
	if c.ExecutableMax == 0 {
		return defaultBinaryMax
	}
	return c.ExecutableMax
}

// ParseChecksums returns the expected digest for an exact basename.
func ParseChecksums(data []byte, expected string) ([]byte, error) {
	if !safeBasename(expected) {
		return nil, fmt.Errorf("unsafe expected checksum name %q", expected)
	}
	var matched []byte
	for lineNumber, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("malformed checksum line %d", lineNumber+1)
		}
		name := strings.TrimPrefix(fields[1], "*")
		if !safeBasename(name) {
			return nil, fmt.Errorf("unsafe checksum filename %q", name)
		}
		if len(fields[0]) != sha256.Size*2 {
			return nil, fmt.Errorf("invalid SHA-256 length for %s", name)
		}
		digest, err := hex.DecodeString(fields[0])
		if err != nil {
			return nil, fmt.Errorf("invalid SHA-256 for %s", name)
		}
		if name != expected {
			continue
		}
		if matched != nil {
			return nil, fmt.Errorf("duplicate checksum for %s", expected)
		}
		matched = digest
	}
	if matched == nil {
		return nil, fmt.Errorf("missing checksum for %s", expected)
	}
	return matched, nil
}

func safeBasename(name string) bool {
	if name == "" || name == "." || name == ".." || path.IsAbs(name) || filepath.IsAbs(name) {
		return false
	}
	if strings.Contains(name, "/") || strings.Contains(name, `\`) {
		return false
	}
	return path.Base(name) == name
}

// ValidateArchive validates the complete archive before returning executable
// bytes. It must only be called after checksum verification.
func ValidateArchive(data []byte, expectedExecutable string, maxSize uint64) ([]byte, error) {
	if !safeBasename(expectedExecutable) {
		return nil, fmt.Errorf("unsafe expected executable name %q", expectedExecutable)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open verified ZIP: %w", err)
	}
	if len(reader.File) != 1 {
		return nil, fmt.Errorf("ZIP must contain exactly one entry (found %d)", len(reader.File))
	}
	file := reader.File[0]
	if file.Name != expectedExecutable || !safeBasename(file.Name) {
		return nil, fmt.Errorf("unexpected ZIP entry %q", file.Name)
	}
	mode := file.Mode()
	if !mode.IsRegular() || mode&0111 == 0 {
		return nil, fmt.Errorf("ZIP entry %q is not a regular executable", file.Name)
	}
	if file.UncompressedSize64 == 0 {
		return nil, fmt.Errorf("ZIP executable %q is empty", file.Name)
	}
	if maxSize == 0 {
		maxSize = defaultBinaryMax
	}
	if file.UncompressedSize64 > maxSize {
		return nil, fmt.Errorf("ZIP executable exceeds %d bytes", maxSize)
	}
	rc, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open ZIP executable: %w", err)
	}
	defer rc.Close()
	payload, err := io.ReadAll(io.LimitReader(rc, int64(maxSize)+1))
	if err != nil {
		return nil, fmt.Errorf("read ZIP executable: %w", err)
	}
	if uint64(len(payload)) > maxSize {
		return nil, fmt.Errorf("ZIP executable exceeds %d bytes", maxSize)
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("ZIP executable %q is empty", file.Name)
	}
	return payload, nil
}
