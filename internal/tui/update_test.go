package tui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Architeg/gloss/internal/buildinfo"
	"github.com/Architeg/gloss/internal/model"
	"github.com/Architeg/gloss/internal/update"
)

type fakeAutomaticChecker struct {
	mu      sync.Mutex
	calls   int
	result  update.CheckResult
	err     error
	current string
}

func (f *fakeAutomaticChecker) Check(_ context.Context, current string) (update.CheckResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.current = current
	return f.result, f.err
}

func (f *fakeAutomaticChecker) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeAutomaticChecker) currentVersion() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.current
}

func TestAutomaticUpdateDisabledSchedulesNothing(t *testing.T) {
	checker := &fakeAutomaticChecker{}
	m := New(Options{
		Config:        &model.Config{CheckForUpdates: false, UpdateCheckInterval: model.UpdateInterval(24 * time.Hour)},
		UpdateChecker: checker,
	}).(*Model)
	if cmd := m.automaticUpdateCommand(); cmd != nil {
		t.Fatal("disabled automatic checks returned a command")
	}
	if checker.callCount() != 0 || m.updateCheckStarted {
		t.Fatalf("disabled check state: calls=%d started=%v", checker.callCount(), m.updateCheckStarted)
	}
}

func TestAutomaticUpdateDueCheckIsDeferredAndRecorded(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	checker := &fakeAutomaticChecker{result: update.CheckResult{LatestVersion: "0.2.0"}}
	state := update.StateStore{
		Path: filepath.Join(t.TempDir(), "update-state.json"),
		Now:  func() time.Time { return now },
	}
	m := New(Options{
		Config:              &model.Config{CheckForUpdates: true, UpdateCheckInterval: model.UpdateInterval(24 * time.Hour)},
		UpdateChecker:       checker,
		Version:             "0.1.0",
		UpdateState:         state,
		InspectUpdateLayout: func() (update.Layout, error) { return update.Layout{}, nil },
		UpdateTimeout:       time.Second,
	}).(*Model)
	cmd := m.automaticUpdateCommand()
	if cmd == nil || checker.callCount() != 0 {
		t.Fatalf("automatic command was not deferred: cmd=%v calls=%d", cmd, checker.callCount())
	}
	if duplicate := m.automaticUpdateCommand(); duplicate != nil {
		t.Fatal("duplicate automatic check was scheduled")
	}
	msg := cmd().(automaticUpdateMsg)
	if msg.err != nil || msg.skipped || checker.callCount() != 1 {
		t.Fatalf("automatic message = %#v calls=%d", msg, checker.callCount())
	}
	loaded, err := state.Load()
	if err != nil || loaded.LatestVersion != "0.2.0" || !loaded.LastCompleted.Equal(now) {
		t.Fatalf("recorded state = %#v, %v", loaded, err)
	}
}

func TestAutomaticUpdateInsideIntervalIsSkipped(t *testing.T) {
	now := time.Now().UTC()
	state := update.StateStore{Path: filepath.Join(t.TempDir(), "update-state.json"), Now: func() time.Time { return now }}
	if err := state.MarkCompleted("0.1.0"); err != nil {
		t.Fatal(err)
	}
	checker := &fakeAutomaticChecker{}
	cmd := automaticUpdateCheckCmd(checker, state, "0.1.0", 24*time.Hour, time.Second, nil)
	msg := cmd().(automaticUpdateMsg)
	if !msg.skipped || checker.callCount() != 0 {
		t.Fatalf("inside-interval message = %#v calls=%d", msg, checker.callCount())
	}
}

func TestAutomaticUpdateFailureIsQuietAndNotRecorded(t *testing.T) {
	state := update.StateStore{Path: filepath.Join(t.TempDir(), "update-state.json")}
	checker := &fakeAutomaticChecker{err: errors.New("offline")}
	cmd := automaticUpdateCheckCmd(checker, state, "0.1.0", 24*time.Hour, 20*time.Millisecond, nil)
	msg := cmd().(automaticUpdateMsg)
	m := New(Options{}).(*Model)
	m.errBanner = "useful existing error"
	m.commandStatus.text = "Copied"
	_, next := m.Update(msg)
	if next != nil || m.errBanner != "useful existing error" || m.commandStatus.text != "Copied" || m.updateNotice != "" {
		t.Fatalf("quiet failure changed UI: error=%q status=%q notice=%q", m.errBanner, m.commandStatus.text, m.updateNotice)
	}
	if _, err := state.Load(); err == nil {
		t.Fatal("failed check was recorded")
	}
}

func TestAutomaticUpdateAvailableNoticeAndHomebrewGuidance(t *testing.T) {
	for _, tt := range []struct {
		name     string
		homebrew bool
		want     string
	}{
		{name: "manual", want: "run gloss update --install"},
		{name: "homebrew", homebrew: true, want: update.HomebrewUpgradeCommand},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := New(Options{}).(*Model)
			m.allEntries = []model.Entry{{ID: 1, Command: "one", Description: "description"}}
			m.rebuildBrowse()
			beforeRows := append([]cmdRow(nil), m.cmdRows...)
			beforeCursor, beforeOffset, beforeID := m.browseCursor, m.browseOffset, m.selectedID
			msg := automaticUpdateMsg{
				result:   update.CheckResult{LatestVersion: "0.2.0", UpdateAvailable: true},
				homebrew: tt.homebrew,
			}
			_, cmd := m.Update(msg)
			if cmd != nil || !strings.Contains(m.updateNotice, tt.want) {
				t.Fatalf("notice = %q, cmd=%v", m.updateNotice, cmd)
			}
			if len(m.cmdRows) != len(beforeRows) || m.browseCursor != beforeCursor || m.browseOffset != beforeOffset || m.selectedID != beforeID {
				t.Fatalf("update notice changed command-list state")
			}
			if !strings.Contains(stripANSI(m.homeView(76)), m.updateNotice) {
				t.Fatalf("home view omitted notice: %q", stripANSI(m.homeView(76)))
			}
		})
	}
}

func TestAutomaticUpdateNoUpdateIsQuiet(t *testing.T) {
	m := New(Options{}).(*Model)
	m.updateNotice = "existing notice"
	_, cmd := m.Update(automaticUpdateMsg{result: update.CheckResult{LatestVersion: "0.1.0"}})
	if cmd != nil || m.updateNotice != "existing notice" {
		t.Fatalf("no-update result changed notice: %q", m.updateNotice)
	}
}

func TestBannerAndUpdaterUseSameResolvedVersion(t *testing.T) {
	version := buildinfo.Resolve("v3.2.1", nil)
	checker := &fakeAutomaticChecker{result: update.CheckResult{LatestVersion: version}}
	m := New(Options{
		Config:        &model.Config{CheckForUpdates: true, UpdateCheckInterval: model.UpdateInterval(time.Hour)},
		Version:       version,
		UpdateChecker: checker,
		UpdateState:   update.StateStore{Path: filepath.Join(t.TempDir(), "state.json")},
	}).(*Model)
	if !strings.Contains(stripANSI(m.renderHomeBanner(100)), "v3.2.1") {
		t.Fatalf("banner does not contain resolved version: %q", stripANSI(m.renderHomeBanner(100)))
	}
	msg := m.automaticUpdateCommand()().(automaticUpdateMsg)
	if msg.err != nil || checker.callCount() != 1 {
		t.Fatalf("automatic update check = %#v, calls=%d", msg, checker.callCount())
	}
	if checker.currentVersion() != version {
		t.Fatalf("updater received %q, want banner version %q", checker.currentVersion(), version)
	}
}
