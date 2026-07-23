package model

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// UpdateInterval is a YAML duration used for automatic update checks.
type UpdateInterval time.Duration

// Duration returns the standard-library duration value.
func (d UpdateInterval) Duration() time.Duration {
	return time.Duration(d)
}

// MarshalYAML writes the human-readable duration used by config.yaml.
func (d UpdateInterval) MarshalYAML() (any, error) {
	duration := time.Duration(d)
	if duration%time.Hour == 0 {
		return fmt.Sprintf("%dh", duration/time.Hour), nil
	}
	return duration.String(), nil
}

// UnmarshalYAML validates a positive human-readable duration.
func (d *UpdateInterval) UnmarshalYAML(value *yaml.Node) error {
	parsed, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("invalid update_check_interval %q: %w", value.Value, err)
	}
	if parsed <= 0 {
		return fmt.Errorf("update_check_interval must be positive")
	}
	*d = UpdateInterval(parsed)
	return nil
}

// Config holds user preferences and storage locations.
type Config struct {
	ShellFile           string         `yaml:"shell_file"`
	StoragePath         string         `yaml:"storage_path"`
	ScanPaths           []string       `yaml:"scan_paths"`
	UseColor            bool           `yaml:"use_color"`
	CheckForUpdates     bool           `yaml:"check_for_updates,omitempty"`
	CheckForUpdatesSet  bool           `yaml:"-"`
	UpdateCheckInterval UpdateInterval `yaml:"update_check_interval"`
}
