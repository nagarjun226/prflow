package cmd

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

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

// setupTestEnv creates a temp config and empty cache DB so runListTo works.
func setupTestEnv(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()

	// Config
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfg := config.DefaultConfig()
	cfg.Repos = []string{"org/repo"}
	config.SetPathOverride(cfgPath)
	t.Cleanup(func() { config.SetPathOverride("") })
	if err := config.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	// Cache DB -- point HOME so cache.Open() creates DB there
	os.Setenv("HOME", tmpDir)
	t.Cleanup(func() { os.Unsetenv("HOME") })
}

// seedDB inserts PRs directly into the cache DB at the temp HOME.
func seedDB(t *testing.T) {
	t.Helper()
	home := os.Getenv("HOME")
	dir := filepath.Join(home, ".config", "prflow")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dbPath := filepath.Join(dir, "prflow.db")
	db, err := sql.Open("sqlite3", dbPath+"?_journal=WAL")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS prs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		repo TEXT NOT NULL,
		number INTEGER NOT NULL,
		title TEXT,
		state TEXT,
		author TEXT,
		branch TEXT,
		base_branch TEXT,
		url TEXT,
		created_at TEXT,
		updated_at TEXT,
		mergeable TEXT,
		review_decision TEXT,
		ci_status TEXT,
		is_draft INTEGER DEFAULT 0,
		section TEXT,
		raw_json TEXT,
		fetched_at TEXT,
		UNIQUE(repo, number)
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	prs := []struct {
		repo, title, section, reviewDecision, mergeable, updatedAt string
		number                                                     int
	}{
		{"org/repo", "Fix login bug", "do_now", "CHANGES_REQUESTED", "MERGEABLE", "2026-03-10T10:00:00Z", 42},
		{"org/repo", "Add dark mode", "waiting", "REVIEW_REQUIRED", "MERGEABLE", "2026-03-09T08:00:00Z", 43},
		{"other/lib", "Bump deps", "review", "", "MERGEABLE", "2026-03-08T12:00:00Z", 7},
		{"org/repo", "Resolve merge conflict", "needs_attention", "", "CONFLICTING", "2026-03-11T09:00:00Z", 50},
	}

	for _, p := range prs {
		_, err := db.Exec(`
			INSERT INTO prs (repo, number, title, state, author, branch, base_branch, url,
				created_at, updated_at, mergeable, review_decision, ci_status, is_draft, section, raw_json, fetched_at)
			VALUES (?, ?, ?, 'OPEN', 'user', 'branch', 'main', '', '', ?, ?, ?, 'UNKNOWN', 0, ?, '{}', datetime('now'))`,
			p.repo, p.number, p.title, p.updatedAt, p.mergeable, p.reviewDecision, p.section)
		if err != nil {
			t.Fatalf("insert PR: %v", err)
		}
	}
}

func TestRunListJSON(t *testing.T) {
	setupTestEnv(t)
	seedDB(t)

	var buf bytes.Buffer
	err := runListTo(&buf, true)
	if err != nil {
		t.Fatalf("runListTo(json) error: %v", err)
	}

	var out jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, buf.String())
	}

	if len(out.DoNow) != 1 {
		t.Errorf("expected 1 do_now PR, got %d", len(out.DoNow))
	} else {
		if out.DoNow[0].Number != 42 {
			t.Errorf("expected do_now PR #42, got #%d", out.DoNow[0].Number)
		}
		if out.DoNow[0].Repo != "org/repo" {
			t.Errorf("expected repo org/repo, got %s", out.DoNow[0].Repo)
		}
		if out.DoNow[0].ReviewDecision != "CHANGES_REQUESTED" {
			t.Errorf("expected review_decision CHANGES_REQUESTED, got %s", out.DoNow[0].ReviewDecision)
		}
	}

	if len(out.Waiting) != 1 {
		t.Errorf("expected 1 waiting PR, got %d", len(out.Waiting))
	}
	if len(out.Review) != 1 {
		t.Errorf("expected 1 review PR, got %d", len(out.Review))
	}
	if len(out.NeedsAttention) != 1 {
		t.Errorf("expected 1 needs_attention PR, got %d", len(out.NeedsAttention))
	} else if out.NeedsAttention[0].Number != 50 {
		t.Errorf("expected needs_attention PR #50, got #%d", out.NeedsAttention[0].Number)
	}

	raw := buf.String()
	for _, key := range []string{"do_now", "waiting", "review", "needs_attention"} {
		if !strings.Contains(raw, `"`+key+`"`) {
			t.Errorf("JSON output missing key %q", key)
		}
	}
	for _, key := range []string{"repo", "number", "title", "review_decision", "mergeable", "updated_at"} {
		if !strings.Contains(raw, `"`+key+`"`) {
			t.Errorf("JSON PR object missing key %q", key)
		}
	}
}

func TestRunListJSONEmpty(t *testing.T) {
	setupTestEnv(t)

	var buf bytes.Buffer
	err := runListTo(&buf, true)
	if err != nil {
		t.Fatalf("runListTo(json, empty) error: %v", err)
	}

	var out jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON for empty cache: %v\n%s", err, buf.String())
	}

	raw := buf.String()
	for _, key := range []string{"do_now", "waiting", "review", "needs_attention"} {
		if strings.Contains(raw, `"`+key+`": null`) {
			t.Errorf("section %q is null, expected empty array", key)
		}
	}

	if len(out.DoNow) != 0 || len(out.Waiting) != 0 || len(out.Review) != 0 || len(out.NeedsAttention) != 0 {
		t.Errorf("expected all empty sections, got do_now=%d waiting=%d review=%d needs_attention=%d",
			len(out.DoNow), len(out.Waiting), len(out.Review), len(out.NeedsAttention))
	}
}

func TestRunListPlaintext(t *testing.T) {
	setupTestEnv(t)
	seedDB(t)

	var buf bytes.Buffer
	err := runListTo(&buf, false)
	if err != nil {
		t.Fatalf("runListTo(plaintext) error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "Do Now") {
		t.Error("plaintext output missing 'Do Now' section")
	}
	if !strings.Contains(output, "Waiting") {
		t.Error("plaintext output missing 'Waiting' section")
	}
	if !strings.Contains(output, "Review") {
		t.Error("plaintext output missing 'Review' section")
	}

	if !strings.Contains(output, "#42") {
		t.Error("plaintext output missing PR #42")
	}
	if !strings.Contains(output, "Fix login bug") {
		t.Error("plaintext output missing PR title 'Fix login bug'")
	}
	if !strings.Contains(output, "Add dark mode") {
		t.Error("plaintext output missing PR title 'Add dark mode'")
	}

	var js json.RawMessage
	if json.Unmarshal([]byte(output), &js) == nil {
		t.Error("plaintext output should not be valid JSON")
	}
}

func TestRunListPlaintextEmpty(t *testing.T) {
	setupTestEnv(t)

	var buf bytes.Buffer
	err := runListTo(&buf, false)
	if err != nil {
		t.Fatalf("runListTo(plaintext, empty) error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No cached PRs") {
		t.Errorf("expected 'No cached PRs' message, got: %s", output)
	}
}

func TestHasFlag(t *testing.T) {
	tests := []struct {
		args []string
		flag string
		want bool
	}{
		{[]string{"--json"}, "--json", true},
		{[]string{"--verbose", "--json"}, "--json", true},
		{[]string{"--verbose"}, "--json", false},
		{nil, "--json", false},
	}
	for _, tt := range tests {
		got := hasFlag(tt.args, tt.flag)
		if got != tt.want {
			t.Errorf("hasFlag(%v, %q) = %v, want %v", tt.args, tt.flag, got, tt.want)
		}
	}
}

func TestRunSyncNoConfig(t *testing.T) {
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
