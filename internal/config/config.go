package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Architeg/gloss/internal/model"
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
	if err := ensurePrivateConfigDir(dir); err != nil {
		return nil, fmt.Errorf("config dir: %w", err)
	}

	path := filepath.Join(dir, relConfigFile)
	def := defaults(home)

	exists, err := ensurePrivateConfigFile(path)
	if err != nil {
		return nil, fmt.Errorf("config file: %w", err)
	}
	if !exists {
		if err := writeConfig(path, def, true); err != nil {
			return nil, err
		}
		return def, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		if err := writeConfig(path, def, false); err != nil {
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
	if value, ok := raw["check_for_updates"]; !ok {
		cfg.CheckForUpdates = def.CheckForUpdates
	} else {
		enabled, valid := value.(bool)
		if !valid {
			return nil, fmt.Errorf("parse config: check_for_updates must be true or false")
		}
		cfg.CheckForUpdates = enabled
		cfg.CheckForUpdatesSet = true
	}
	if _, ok := raw["update_check_interval"]; !ok {
		cfg.UpdateCheckInterval = def.UpdateCheckInterval
	}

	return &cfg, nil
}

// SaveCheckForUpdates atomically persists an explicit automatic-update choice
// while preserving unrelated YAML nodes and comments.
func SaveCheckForUpdates(enabled bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("user home: %w", err)
	}
	dir := filepath.Join(home, relConfigDir)
	if err := ensurePrivateConfigDir(dir); err != nil {
		return fmt.Errorf("config dir: %w", err)
	}
	path := filepath.Join(dir, relConfigFile)
	exists, err := ensurePrivateConfigFile(path)
	if err != nil {
		return fmt.Errorf("config file: %w", err)
	}
	if !exists {
		return fmt.Errorf("config file does not exist: %s", path)
	}
	return updateBooleanKey(path, "check_for_updates", enabled)
}

func updateBooleanKey(path, key string, value bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect config: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("config file is not a regular non-symlink file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("config root must be a mapping")
	}
	mapping := document.Content[0]
	var valueNode *yaml.Node
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			valueNode = mapping.Content[i+1]
			break
		}
	}
	if valueNode == nil {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
			&yaml.Node{},
		)
		valueNode = mapping.Content[len(mapping.Content)-1]
	}
	*valueNode = yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!bool",
		Value: strconv.FormatBool(value),
	}
	encoded, err := yaml.Marshal(&document)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return replaceConfigAtomically(path, info, encoded)
}

func replaceConfigAtomically(path string, original os.FileInfo, data []byte) (retErr error) {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".config.yaml-*")
	if err != nil {
		return fmt.Errorf("create staged config: %w", err)
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
		return fmt.Errorf("chmod staged config: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("write staged config: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync staged config: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close staged config: %w", err)
	}
	closed = true
	current, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("revalidate config: %w", err)
	}
	if !current.Mode().IsRegular() || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(original, current) {
		return fmt.Errorf("config file changed while saving")
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

func ensurePrivateConfigDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symlink", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	return os.Chmod(path, 0o700)
}

func ensurePrivateConfigFile(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		return false, nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("%s is a symlink", path)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("%s is not a regular file", path)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return false, err
	}
	return true, nil
}

func defaults(home string) *model.Config {
	shellFile, scanPaths := defaultShellPaths(home)

	store := filepath.Join(home, relConfigDir)
	return &model.Config{
		ShellFile:           shellFile,
		StoragePath:         store,
		ScanPaths:           scanPaths,
		UseColor:            true,
		CheckForUpdates:     false,
		UpdateCheckInterval: model.UpdateInterval(24 * time.Hour),
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

func writeConfig(path string, cfg *model.Config, create bool) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	flags := os.O_WRONLY | os.O_TRUNC
	if create {
		flags |= os.O_CREATE | os.O_EXCL
	}
	f, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return fmt.Errorf("open config: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
