package config

import (
	"os"
	"path/filepath"

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
		},
	}
}

func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "prflow", "config.yaml")
}

func Load() (*Config, error) {
	data, err := os.ReadFile(Path())
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
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}
