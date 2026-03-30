package cmd

import (
	"fmt"
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

func TestParseOpenArgs(t *testing.T) {
	tests := []struct {
		input   string
		repo    string
		number  int
		wantErr bool
	}{
		{input: "org/repo#42", repo: "org/repo", number: 42},
		{input: "#42", repo: "", number: 42},
		{input: "org/repo", repo: "org/repo", number: 0},
		{input: "", repo: "", number: 0},
		{input: "myorg/myrepo#1", repo: "myorg/myrepo", number: 1},
		{input: "#", wantErr: true},
		{input: "#abc", wantErr: true},
		{input: "#0", wantErr: true},
		{input: "#-5", wantErr: true},
		{input: "badarg", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input=%q", tt.input), func(t *testing.T) {
			got, err := parseOpenArgs(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseOpenArgs(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseOpenArgs(%q) unexpected error: %v", tt.input, err)
			}
			if got.Repo != tt.repo {
				t.Errorf("parseOpenArgs(%q).Repo = %q, want %q", tt.input, got.Repo, tt.repo)
			}
			if got.Number != tt.number {
				t.Errorf("parseOpenArgs(%q).Number = %d, want %d", tt.input, got.Number, tt.number)
			}
		})
	}
}

func TestParseRepoFromURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://github.com/org/repo.git", "org/repo"},
		{"https://github.com/org/repo", "org/repo"},
		{"git@github.com:org/repo.git", "org/repo"},
		{"git@github.com:org/repo", "org/repo"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseRepoFromURL(tt.input)
			if err != nil {
				t.Fatalf("parseRepoFromURL(%q) error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("parseRepoFromURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRunOpenWithFullRef(t *testing.T) {
	// Stub repoFromRemote so we don't need a real git repo
	origFn := repoFromRemote
	defer func() { repoFromRemote = origFn }()
	repoFromRemote = func() (string, error) {
		return "fallback/repo", nil
	}

	tests := []struct {
		name     string
		arg      string
		wantRepo string
		wantNum  int
	}{
		{"full ref", "org/repo#99", "org/repo", 99},
		{"number only uses remote", "#7", "fallback/repo", 7},
		{"repo only", "org/repo", "org/repo", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseOpenArgs(tt.arg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			repo := parsed.Repo
			if repo == "" {
				repo, _ = repoFromRemote()
			}
			if repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
			}

			var url string
			if parsed.Number > 0 {
				url = fmt.Sprintf("https://github.com/%s/pull/%d", repo, parsed.Number)
			} else {
				url = fmt.Sprintf("https://github.com/%s/pulls", repo)
			}

			if tt.wantNum > 0 {
				expected := fmt.Sprintf("https://github.com/%s/pull/%d", tt.wantRepo, tt.wantNum)
				if url != expected {
					t.Errorf("url = %q, want %q", url, expected)
				}
			} else {
				expected := fmt.Sprintf("https://github.com/%s/pulls", tt.wantRepo)
				if url != expected {
					t.Errorf("url = %q, want %q", url, expected)
				}
			}
		})
	}
}
