package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/valeriybagrintsev/gloss/internal/model"
)

// Default relative paths under the user home directory.
const (
	relConfigDir  = ".config/gloss"
	relConfigFile = "config.yaml"
)

// Load reads or creates ~/.config/gloss/config.yaml and returns merged settings.
func Load() (*model.Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("user home: %w", err)
	}

	dir := filepath.Join(home, relConfigDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("config dir: %w", err)
	}

	path := filepath.Join(dir, relConfigFile)
	def := defaults(home)

	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := writeConfig(path, def); err != nil {
			return nil, err
		}
		return def, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		if err := writeConfig(path, def); err != nil {
			return nil, err
		}
		return def, nil
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	var cfg model.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.ShellFile == "" {
		cfg.ShellFile = def.ShellFile
	}
	if cfg.StoragePath == "" {
		cfg.StoragePath = def.StoragePath
	}
	if _, ok := raw["scan_paths"]; !ok {
		cfg.ScanPaths = def.ScanPaths
	}
	if _, ok := raw["use_color"]; !ok {
		cfg.UseColor = def.UseColor
	}

	return &cfg, nil
}

func defaults(home string) *model.Config {
	shellFile, scanPaths := defaultShellPaths(home)

	store := filepath.Join(home, relConfigDir)
	return &model.Config{
		ShellFile:   shellFile,
		StoragePath: store,
		ScanPaths:   scanPaths,
		UseColor:    true,
	}
}

func defaultShellPaths(home string) (string, []string) {
	shell := os.Getenv("SHELL")
	base := filepath.Base(shell)

	switch base {
	case "bash":
		bashrc := filepath.Join(home, ".bashrc")
		bashAliases := filepath.Join(home, ".bash_aliases")
		return bashrc, []string{bashrc, bashAliases}

	case "zsh":
		zshrc := filepath.Join(home, ".zshrc")
		return zshrc, []string{zshrc}

	default:
		// Safe fallback for unusual shells.
		// Gloss v1 officially targets zsh and bash-style alias files.
		zshrc := filepath.Join(home, ".zshrc")
		return zshrc, []string{zshrc}
	}
}

func writeConfig(path string, cfg *model.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
