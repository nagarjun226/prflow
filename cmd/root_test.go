package cmd

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/cheenu1092-oss/prflow/internal/config"
)

func TestPrintUsage(t *testing.T) {
	// Should not panic
	printUsage()
}

func TestPrintUsageContainsWatch(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printUsage()

	w.Close()
	os.Stdout = old

	out, _ := io.ReadAll(r)
	if !strings.Contains(string(out), "watch") {
		t.Error("expected 'watch' in usage output")
	}
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

	// Cache DB — point HOME so cache.Open() creates DB there
	os.Setenv("HOME", tmpDir)
	t.Cleanup(func() { os.Unsetenv("HOME") })
}

// seedDB inserts PRs directly into the cache DB at the temp HOME.
// It creates the directory structure and schema first.
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

	// Create schema so inserts work even if cache.Open hasn't been called yet.
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

	// Verify all required JSON keys are present by checking the raw JSON.
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

	// All sections should be empty arrays, not null.
	raw := buf.String()
	for _, key := range []string{"do_now", "waiting", "review", "needs_attention"} {
		// The value should be [] (empty array), not null.
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

	// Should contain section headers.
	if !strings.Contains(output, "Do Now") {
		t.Error("plaintext output missing 'Do Now' section")
	}
	if !strings.Contains(output, "Waiting") {
		t.Error("plaintext output missing 'Waiting' section")
	}
	if !strings.Contains(output, "Review") {
		t.Error("plaintext output missing 'Review' section")
	}

	// Should contain PR numbers and titles.
	if !strings.Contains(output, "#42") {
		t.Error("plaintext output missing PR #42")
	}
	if !strings.Contains(output, "Fix login bug") {
		t.Error("plaintext output missing PR title 'Fix login bug'")
	}
	if !strings.Contains(output, "Add dark mode") {
		t.Error("plaintext output missing PR title 'Add dark mode'")
	}

	// Should NOT be valid JSON.
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
