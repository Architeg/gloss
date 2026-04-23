package model

// Config holds user preferences and storage locations.
type Config struct {
	ShellFile   string   `yaml:"shell_file"`
	StoragePath string   `yaml:"storage_path"`
	ScanPaths   []string `yaml:"scan_paths"`
	UseColor    bool     `yaml:"use_color"`
}
