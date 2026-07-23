package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CheckState is the minimal persisted automatic-check state.
type CheckState struct {
	LastCompleted time.Time `json:"last_completed"`
	LatestVersion string    `json:"latest_version,omitempty"`
}

// StateStore persists automatic-check state beside other Gloss-owned data.
type StateStore struct {
	Path string
	Now  func() time.Time
}

// Due reports whether an automatic check should run. Missing or malformed
// state is treated as due so it can never prevent application startup.
func (s StateStore) Due(interval time.Duration) bool {
	if interval <= 0 {
		return false
	}
	state, err := s.Load()
	if err != nil || state.LastCompleted.IsZero() {
		return true
	}
	return !s.now().Before(state.LastCompleted.Add(interval))
}

// Load reads state without changing it.
func (s StateStore) Load() (CheckState, error) {
	if s.Path == "" {
		return CheckState{}, errors.New("update state path is empty")
	}
	info, err := os.Lstat(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return CheckState{}, err
		}
		return CheckState{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return CheckState{}, fmt.Errorf("unsafe update state file %s", s.Path)
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return CheckState{}, err
	}
	var state CheckState
	if err := json.Unmarshal(data, &state); err != nil {
		return CheckState{}, err
	}
	return state, nil
}

// MarkCompleted atomically records a successful release check.
func (s StateStore) MarkCompleted(latest string) (retErr error) {
	if s.Path == "" {
		return errors.New("update state path is empty")
	}
	dir := filepath.Dir(s.Path)
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("update state directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsafe update state directory %s", dir)
	}
	if existing, err := os.Lstat(s.Path); err == nil {
		if existing.Mode()&os.ModeSymlink != 0 || !existing.Mode().IsRegular() {
			return fmt.Errorf("unsafe update state file %s", s.Path)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	state := CheckState{LastCompleted: s.now().UTC(), LatestVersion: latest}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(dir, ".update-state-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temp.Close()
		}
		if retErr != nil {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(0o600); err != nil {
		return err
	}
	if err := writeComplete(temp, data); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	closed = true
	if err := os.Rename(tempPath, s.Path); err != nil {
		return err
	}
	return os.Chmod(s.Path, 0o600)
}

func (s StateStore) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
