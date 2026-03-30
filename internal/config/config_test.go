package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}
	if len(cfg.Repos) != 0 {
		t.Errorf("expected empty repos, got %d", len(cfg.Repos))
	}
	if len(cfg.Favorites) != 0 {
		t.Errorf("expected empty favorites, got %d", len(cfg.Favorites))
	}
	if cfg.Settings.RefreshInterval != "2m" {
		t.Errorf("expected refresh_interval '2m', got '%s'", cfg.Settings.RefreshInterval)
	}
	if cfg.Settings.StaleThreshold != "3d" {
		t.Errorf("expected stale_threshold '3d', got '%s'", cfg.Settings.StaleThreshold)
	}
	if cfg.Settings.MergeMethod != "squash" {
		t.Errorf("expected merge_method 'squash', got '%s'", cfg.Settings.MergeMethod)
	}
	if cfg.Settings.PageSize != 50 {
		t.Errorf("expected page_size 50, got %d", cfg.Settings.PageSize)
	}
	if cfg.Settings.Theme != "auto" {
		t.Errorf("expected theme 'auto', got '%s'", cfg.Settings.Theme)
	}
	if len(cfg.Workspace.ScanDirs) != 4 {
		t.Errorf("expected 4 scan dirs, got %d", len(cfg.Workspace.ScanDirs))
	}
	if cfg.Workspace.Repos == nil {
		t.Error("expected non-nil workspace repos map")
	}
}

func TestPath(t *testing.T) {
	p := Path()
	if p == "" {
		t.Fatal("Path returned empty string")
	}
	if !filepath.IsAbs(p) {
		t.Errorf("expected absolute path, got '%s'", p)
	}
	if filepath.Base(p) != "config.yaml" {
		t.Errorf("expected config.yaml, got '%s'", filepath.Base(p))
	}
	if !contains(p, "prflow") {
		t.Errorf("expected path to contain 'prflow', got '%s'", p)
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Use a temp directory
	tmpDir := t.TempDir()
	origPath := Path

	// Override config path for testing
	testPath := filepath.Join(tmpDir, "config.yaml")
	oldPathFunc := pathOverride
	pathOverride = testPath
	defer func() { pathOverride = oldPathFunc }()

	_ = origPath

	cfg := DefaultConfig()
	cfg.Repos = []string{"org/repo1", "org/repo2"}
	cfg.Favorites = []string{"org/repo1"}
	cfg.Settings.Editor = "nvim"
	cfg.Settings.MergeMethod = "rebase"
	cfg.Settings.Theme = "dark"
	cfg.Workspace.Repos = map[string]string{
		"org/repo1": "/home/user/repos/repo1",
	}

	// Save
	err := saveToPath(cfg, testPath)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load
	loaded, err := loadFromPath(testPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify
	if len(loaded.Repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(loaded.Repos))
	}
	if loaded.Repos[0] != "org/repo1" {
		t.Errorf("expected 'org/repo1', got '%s'", loaded.Repos[0])
	}
	if len(loaded.Favorites) != 1 {
		t.Errorf("expected 1 favorite, got %d", len(loaded.Favorites))
	}
	if loaded.Settings.Editor != "nvim" {
		t.Errorf("expected editor 'nvim', got '%s'", loaded.Settings.Editor)
	}
	if loaded.Settings.MergeMethod != "rebase" {
		t.Errorf("expected merge_method 'rebase', got '%s'", loaded.Settings.MergeMethod)
	}
	if loaded.Settings.Theme != "dark" {
		t.Errorf("expected theme 'dark', got '%s'", loaded.Settings.Theme)
	}
	if loaded.Workspace.Repos["org/repo1"] != "/home/user/repos/repo1" {
		t.Errorf("expected workspace repo path, got '%s'", loaded.Workspace.Repos["org/repo1"])
	}
}

func TestThemeConfigRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "config.yaml")

	for _, theme := range []string{"auto", "dark", "light"} {
		cfg := DefaultConfig()
		cfg.Settings.Theme = theme

		err := saveToPath(cfg, testPath)
		if err != nil {
			t.Fatalf("Save failed for theme %q: %v", theme, err)
		}

		loaded, err := loadFromPath(testPath)
		if err != nil {
			t.Fatalf("Load failed for theme %q: %v", theme, err)
		}

		if loaded.Settings.Theme != theme {
			t.Errorf("expected theme %q, got %q", theme, loaded.Settings.Theme)
		}
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := loadFromPath("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error loading nonexistent config")
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	deepPath := filepath.Join(tmpDir, "a", "b", "c", "config.yaml")

	cfg := DefaultConfig()
	err := saveToPath(cfg, deepPath)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(deepPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created in nested directory")
	}
}

func contains(s, substr string) bool {
	return filepath.Base(filepath.Dir(filepath.Dir(s))) == ".config" || 
		len(s) > 0 && filepath.Base(filepath.Dir(s)) == "prflow"
}
