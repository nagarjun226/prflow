package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cheenu1092-oss/prflow/internal/config"
	"github.com/cheenu1092-oss/prflow/internal/gh"
)

func TestPrintUsage(t *testing.T) {
	// Should not panic
	printUsage()
}

func TestRunConfig(t *testing.T) {
	err := runConfig()
	if err != nil {
		t.Errorf("runConfig should not error: %v", err)
	}
}

func TestRunListWithConfig(t *testing.T) {
	// Create a temp config so runList works
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "config.yaml")
	cfg := config.DefaultConfig()
	cfg.Repos = []string{"org/repo"}
	os.MkdirAll(filepath.Dir(testPath), 0755)
	config.SetPathOverride(testPath)
	defer config.SetPathOverride("")

	err := config.Save(cfg)
	if err != nil {
		t.Fatalf("failed to save test config: %v", err)
	}

	// runList should not panic or error (cache will be empty, that's ok)
	err = runList()
	if err != nil {
		t.Errorf("runList should not error with valid config: %v", err)
	}
}

func TestRunSyncNoConfig(t *testing.T) {
	// Without config, sync should return an error
	config.SetPathOverride("/nonexistent/config.yaml")
	defer config.SetPathOverride("")

	err := runSync()
	if err == nil {
		t.Error("runSync should error without config")
	}
}

func TestClassifyPR(t *testing.T) {
	tests := []struct {
		name     string
		pr       *gh.PR
		username string
		want     string
	}{
		{
			name: "changes requested on my PR",
			pr: &gh.PR{
				Author:         gh.Author{Login: "me"},
				ReviewDecision: "CHANGES_REQUESTED",
			},
			username: "me",
			want:     "do_now",
		},
		{
			name: "approved my PR",
			pr: &gh.PR{
				Author:         gh.Author{Login: "me"},
				ReviewDecision: "APPROVED",
			},
			username: "me",
			want:     "do_now",
		},
		{
			name: "my PR with conflicts",
			pr: &gh.PR{
				Author:    gh.Author{Login: "me"},
				Mergeable: "CONFLICTING",
			},
			username: "me",
			want:     "do_now",
		},
		{
			name: "my PR waiting for review",
			pr: &gh.PR{
				Author: gh.Author{Login: "me"},
			},
			username: "me",
			want:     "waiting",
		},
		{
			name: "someone else's PR",
			pr: &gh.PR{
				Author: gh.Author{Login: "other"},
			},
			username: "me",
			want:     "review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyPR(tt.pr, tt.username)
			if got != tt.want {
				t.Errorf("classifyPR() = %q, want %q", got, tt.want)
			}
		})
	}
}
