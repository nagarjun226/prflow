package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Repos     []string          `yaml:"repos"`
	Favorites []string          `yaml:"favorites"`
	Workspace WorkspaceConfig   `yaml:"workspace"`
	Settings  SettingsConfig    `yaml:"settings"`
}

type WorkspaceConfig struct {
	ScanDirs []string          `yaml:"scan_dirs"`
	Repos    map[string]string `yaml:"repos"` // "org/repo" -> local path
}

type SettingsConfig struct {
	RefreshInterval string `yaml:"refresh_interval"`
	StaleThreshold  string `yaml:"stale_threshold"`
	Editor          string `yaml:"editor"`
	ReposDir        string `yaml:"repos_dir"`
	MergeMethod     string `yaml:"merge_method"`
	PageSize        int    `yaml:"page_size"`
	Theme           string `yaml:"theme"`
	WatchInterval   string `yaml:"watch_interval"`
}

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Repos:     []string{},
		Favorites: []string{},
		Workspace: WorkspaceConfig{
			ScanDirs: []string{
				filepath.Join(home, "repos"),
				filepath.Join(home, "Projects"),
				filepath.Join(home, "work"),
				filepath.Join(home, "src"),
			},
			Repos: make(map[string]string),
		},
		Settings: SettingsConfig{
			RefreshInterval: "2m",
			StaleThreshold:  "3d",
			Editor:          "vim",
			ReposDir:        filepath.Join(home, "repos"),
			MergeMethod:     "squash",
			PageSize:        50,
			Theme:           "auto",
			WatchInterval:   "2m",
		},
	}
}

func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "prflow", "config.yaml")
}

// pathOverride allows tests to override the config path
var pathOverride string

// SetPathOverride sets the config path override for testing
func SetPathOverride(p string) {
	pathOverride = p
}

func Load() (*Config, error) {
	p := Path()
	if pathOverride != "" {
		p = pathOverride
	}
	return loadFromPath(p)
}

func loadFromPath(p string) (*Config, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func Save(cfg *Config) error {
	p := Path()
	if pathOverride != "" {
		p = pathOverride
	}
	return saveToPath(cfg, p)
}

// ParseStaleThresholdDays parses the stale_threshold config string (e.g. "3d", "5d")
// and returns the number of days. Returns the default of 3 if parsing fails.
func ParseStaleThresholdDays(s string) int {
	if s == "" {
		return 3
	}
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimSuffix(s, "d")
	days, err := strconv.Atoi(s)
	if err != nil || days <= 0 {
		return 3
	}
	return days
}

// Validate checks for common config issues and applies defaults where needed.
func (c *Config) Validate() {
	if c.Settings.MergeMethod == "" {
		c.Settings.MergeMethod = "squash"
	}
	if c.Settings.PageSize <= 0 {
		c.Settings.PageSize = 50
	}
	if c.Settings.RefreshInterval == "" {
		c.Settings.RefreshInterval = "2m"
	}
	if c.Settings.StaleThreshold == "" {
		c.Settings.StaleThreshold = "3d"
	}
	if c.Workspace.Repos == nil {
		c.Workspace.Repos = make(map[string]string)
	}
}

func saveToPath(cfg *Config, p string) error {
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}
