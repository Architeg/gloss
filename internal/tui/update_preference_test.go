package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Architeg/gloss/internal/model"
	"github.com/Architeg/gloss/internal/update"
)

func TestUpdateOnboardingPresenceSemantics(t *testing.T) {
	tests := []struct {
		name    string
		config  *model.Config
		want    bool
		enabled bool
	}{
		{
			name: "absent",
			config: &model.Config{
				UpdateCheckInterval: model.UpdateInterval(24 * time.Hour),
			},
			want: true,
		},
		{
			name: "explicit false",
			config: &model.Config{
				CheckForUpdatesSet:  true,
				UpdateCheckInterval: model.UpdateInterval(24 * time.Hour),
			},
		},
		{
			name: "explicit true",
			config: &model.Config{
				CheckForUpdates:     true,
				CheckForUpdatesSet:  true,
				UpdateCheckInterval: model.UpdateInterval(24 * time.Hour),
			},
			enabled: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &fakeAutomaticChecker{}
			m := New(Options{
				Config:               tt.config,
				UpdateChecker:        checker,
				SaveUpdatePreference: func(bool) error { return nil },
			}).(*Model)
			if m.updatePromptVisible != tt.want {
				t.Fatalf("prompt visible = %v, want %v", m.updatePromptVisible, tt.want)
			}
			if checker.callCount() != 0 {
				t.Fatal("model construction performed a network check")
			}
			if tt.want {
				view := stripANSI(m.View())
				if !strings.Contains(view, "Check for Gloss updates automatically?") ||
					!strings.Contains(view, "Updates are never installed automatically.") {
					t.Fatalf("onboarding content missing:\n%s", view)
				}
			}
		})
	}
}

func TestUpdateOnboardingEnablePersistsThenSchedulesOnce(t *testing.T) {
	checker := &fakeAutomaticChecker{result: update.CheckResult{LatestVersion: "0.2.0"}}
	var saved []bool
	state := update.StateStore{Path: filepath.Join(t.TempDir(), "update-state.json")}
	m := New(Options{
		Config: &model.Config{
			UpdateCheckInterval: model.UpdateInterval(24 * time.Hour),
		},
		UpdateChecker: checker,
		UpdateState:   state,
		Version:       "0.1.0",
		SaveUpdatePreference: func(enabled bool) error {
			saved = append(saved, enabled)
			return nil
		},
	}).(*Model)

	if cmd := m.automaticUpdateCommand(); cmd != nil {
		t.Fatal("automatic check scheduled before consent")
	}
	_, saveCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if saveCmd == nil || checker.callCount() != 0 {
		t.Fatalf("save command = %v, checker calls = %d", saveCmd, checker.callCount())
	}
	_, checkCmd := m.Update(saveCmd())
	if len(saved) != 1 || !saved[0] {
		t.Fatalf("saved choices = %v", saved)
	}
	if !m.config.CheckForUpdates || !m.config.CheckForUpdatesSet || m.updatePromptVisible {
		t.Fatalf("saved model state = enabled %v set %v prompt %v",
			m.config.CheckForUpdates, m.config.CheckForUpdatesSet, m.updatePromptVisible)
	}
	if checkCmd == nil || checker.callCount() != 0 {
		t.Fatalf("deferred check = %v, calls = %d", checkCmd, checker.callCount())
	}
	if duplicate := m.automaticUpdateCommand(); duplicate != nil {
		t.Fatal("second automatic check was scheduled")
	}
	_ = checkCmd()
	if checker.callCount() != 1 {
		t.Fatalf("checker calls = %d, want 1", checker.callCount())
	}
}

func TestUpdateOnboardingNotNowAndEscape(t *testing.T) {
	t.Run("not now persists false", func(t *testing.T) {
		var saved []bool
		checker := &fakeAutomaticChecker{}
		m := New(Options{
			Config:        &model.Config{UpdateCheckInterval: model.UpdateInterval(24 * time.Hour)},
			UpdateChecker: checker,
			SaveUpdatePreference: func(enabled bool) error {
				saved = append(saved, enabled)
				return nil
			},
		}).(*Model)
		_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
		_, saveCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		_, checkCmd := m.Update(saveCmd())
		if len(saved) != 1 || saved[0] || checkCmd != nil {
			t.Fatalf("saved = %v, check command = %v", saved, checkCmd)
		}
		if m.config.CheckForUpdates || !m.config.CheckForUpdatesSet || m.updatePromptVisible {
			t.Fatalf("not-now model state = enabled %v set %v prompt %v",
				m.config.CheckForUpdates, m.config.CheckForUpdatesSet, m.updatePromptVisible)
		}
		if checker.callCount() != 0 {
			t.Fatal("Not now performed a network check")
		}
		next := New(Options{
			Config:               m.config,
			SaveUpdatePreference: func(bool) error { return nil },
		}).(*Model)
		if next.updatePromptVisible {
			t.Fatal("explicit false triggered repeated onboarding")
		}
	})

	t.Run("escape makes no choice", func(t *testing.T) {
		calls := 0
		cfg := &model.Config{UpdateCheckInterval: model.UpdateInterval(24 * time.Hour)}
		m := New(Options{
			Config: cfg,
			SaveUpdatePreference: func(bool) error {
				calls++
				return nil
			},
		}).(*Model)
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		if cmd != nil || calls != 0 || cfg.CheckForUpdatesSet || m.updatePromptVisible {
			t.Fatalf("escape state: cmd=%v calls=%d set=%v prompt=%v", cmd, calls, cfg.CheckForUpdatesSet, m.updatePromptVisible)
		}
		next := New(Options{Config: cfg, SaveUpdatePreference: func(bool) error { return nil }}).(*Model)
		if !next.updatePromptVisible {
			t.Fatal("undecided preference did not reappear on the next model launch")
		}
	})
}

func TestUpdateOnboardingSaveFailureRemainsVisible(t *testing.T) {
	cfg := &model.Config{UpdateCheckInterval: model.UpdateInterval(24 * time.Hour)}
	m := New(Options{
		Config: cfg,
		SaveUpdatePreference: func(bool) error {
			return errors.New("read-only config")
		},
	}).(*Model)
	_, saveCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_, next := m.Update(saveCmd())
	if next != nil || !m.updatePromptVisible || cfg.CheckForUpdatesSet || cfg.CheckForUpdates {
		t.Fatalf("failed save state: cmd=%v prompt=%v set=%v enabled=%v",
			next, m.updatePromptVisible, cfg.CheckForUpdatesSet, cfg.CheckForUpdates)
	}
	if !strings.Contains(stripANSI(m.View()), "Could not save update preference: read-only config") {
		t.Fatalf("save error is not visible:\n%s", stripANSI(m.View()))
	}
}

func TestSettingsUpdatePreferenceToggleAndDueLogic(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	state := update.StateStore{
		Path: filepath.Join(t.TempDir(), "update-state.json"),
		Now:  func() time.Time { return now },
	}
	if err := state.MarkCompleted("0.1.0"); err != nil {
		t.Fatal(err)
	}
	checker := &fakeAutomaticChecker{}
	var saved []bool
	cfg := &model.Config{
		CheckForUpdatesSet:  true,
		UpdateCheckInterval: model.UpdateInterval(24 * time.Hour),
	}
	m := New(Options{
		Config:        cfg,
		UpdateChecker: checker,
		UpdateState:   state,
		Version:       "0.1.0",
		SaveUpdatePreference: func(enabled bool) error {
			saved = append(saved, enabled)
			return nil
		},
	}).(*Model)
	m.screen = ScreenSettings
	if view := stripANSI(m.settingsView(76)); !strings.Contains(view, "Automatic update checks") || !strings.Contains(view, "Off") {
		t.Fatalf("disabled Settings row missing:\n%s", view)
	}
	_, saveCmd := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if saveCmd == nil {
		t.Fatal("Settings toggle did not schedule a save")
	}
	if _, repeated := m.Update(tea.KeyMsg{Type: tea.KeyEnter}); repeated != nil {
		t.Fatal("repeated Settings key scheduled a parallel save")
	}
	_, checkCmd := m.Update(saveCmd())
	if len(saved) != 1 || !saved[0] || !cfg.CheckForUpdates || !cfg.CheckForUpdatesSet {
		t.Fatalf("Settings enable = saved %v enabled %v set %v", saved, cfg.CheckForUpdates, cfg.CheckForUpdatesSet)
	}
	if view := stripANSI(m.settingsView(76)); !strings.Contains(view, "On") ||
		!strings.Contains(view, "Automatic update checks enabled.") {
		t.Fatalf("enabled Settings row missing:\n%s", view)
	}
	if checkCmd == nil {
		t.Fatal("Settings enable did not use automatic scheduler")
	}
	msg := checkCmd().(automaticUpdateMsg)
	if !msg.skipped || checker.callCount() != 0 {
		t.Fatalf("inside-interval check = %#v, checker calls=%d", msg, checker.callCount())
	}
}

func TestSettingsDisableSuppressesLateAutomaticNotice(t *testing.T) {
	cfg := &model.Config{
		CheckForUpdates:     true,
		CheckForUpdatesSet:  true,
		UpdateCheckInterval: model.UpdateInterval(24 * time.Hour),
	}
	var saved []bool
	m := New(Options{
		Config: cfg,
		SaveUpdatePreference: func(enabled bool) error {
			saved = append(saved, enabled)
			return nil
		},
	}).(*Model)
	m.screen = ScreenSettings
	_, saveCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if saveCmd == nil {
		t.Fatal("Settings disable did not schedule a save")
	}
	_, _ = m.Update(automaticUpdateMsg{
		result: update.CheckResult{LatestVersion: "0.2.0", UpdateAvailable: true},
	})
	if m.updateNotice != "" {
		t.Fatalf("pending disable allowed a late notice: %q", m.updateNotice)
	}
	_, checkCmd := m.Update(saveCmd())
	if checkCmd != nil || len(saved) != 1 || saved[0] || cfg.CheckForUpdates || !cfg.CheckForUpdatesSet {
		t.Fatalf("disabled state: cmd=%v saved=%v enabled=%v set=%v", checkCmd, saved, cfg.CheckForUpdates, cfg.CheckForUpdatesSet)
	}
	if later := m.automaticUpdateCommand(); later != nil {
		t.Fatal("disabled setting allowed a later automatic check")
	}
	_, _ = m.Update(automaticUpdateMsg{
		result: update.CheckResult{LatestVersion: "0.2.0", UpdateAvailable: true},
	})
	if m.updateNotice != "" {
		t.Fatalf("disabled preference allowed a notice: %q", m.updateNotice)
	}
}

func TestSettingsSaveFailureIsVisibleAndDoesNotToggle(t *testing.T) {
	cfg := &model.Config{
		CheckForUpdatesSet:  true,
		UpdateCheckInterval: model.UpdateInterval(24 * time.Hour),
	}
	m := New(Options{
		Config: cfg,
		SaveUpdatePreference: func(bool) error {
			return errors.New("permission denied")
		},
	}).(*Model)
	m.screen = ScreenSettings
	_, saveCmd := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	_, checkCmd := m.Update(saveCmd())
	if checkCmd != nil || cfg.CheckForUpdates || !cfg.CheckForUpdatesSet {
		t.Fatalf("failed Settings save changed preference: cmd=%v enabled=%v set=%v",
			checkCmd, cfg.CheckForUpdates, cfg.CheckForUpdatesSet)
	}
	view := stripANSI(m.settingsView(76))
	if !strings.Contains(view, "Could not save update preference: permission denied") {
		t.Fatalf("Settings save error missing:\n%s", view)
	}
}

func TestUpdatePreferenceViewsAreSafeAtNarrowWidths(t *testing.T) {
	cfg := &model.Config{
		UpdateCheckInterval: model.UpdateInterval(24 * time.Hour),
	}
	m := New(Options{
		Config:               cfg,
		SaveUpdatePreference: func(bool) error { return nil },
	}).(*Model)
	for _, width := range []int{0, 1, 8, 20, 76} {
		t.Run(fmt.Sprintf("width-%d", width), func(t *testing.T) {
			_ = m.updatePreferenceView(width)
			_ = m.settingsView(width)
		})
	}
}
