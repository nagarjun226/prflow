package tui

import (
	"testing"
	"time"

	"github.com/cheenu1092-oss/prflow/internal/cache"
	"github.com/cheenu1092-oss/prflow/internal/config"
	"github.com/cheenu1092-oss/prflow/internal/gh"
)

func TestTimeSinceFunction(t *testing.T) {
	tests := []struct {
		name     string
		dur      time.Duration
		expected string
	}{
		{"just now", 10 * time.Second, "just now"},
		{"minutes", 5 * time.Minute, "5m ago"},
		{"hours", 3 * time.Hour, "3h ago"},
		{"yesterday", 36 * time.Hour, "yesterday"},
		{"days", 5 * 24 * time.Hour, "5d ago"},
		{"month", 35 * 24 * time.Hour, "1 month ago"},
		{"months", 90 * 24 * time.Hour, "3 months ago"},
		{"year", 400 * 24 * time.Hour, "1 year ago"},
		{"years", 800 * 24 * time.Hour, "2 years ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := time.Now().Add(-tt.dur)
			result := timeSince(ts)
			if result != tt.expected {
				t.Errorf("timeSince(%v ago) = %q, want %q", tt.dur, result, tt.expected)
			}
		})
	}
}

func TestTimeSinceFuture(t *testing.T) {
	ts := time.Now().Add(1 * time.Hour)
	result := timeSince(ts)
	if result != "just now" {
		t.Errorf("future time should be 'just now', got %q", result)
	}
}

func TestFormatTimeAgoFormats(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"2026-03-10T15:30:00Z", true},
		{"2026-03-10T15:30:00-08:00", true},
		{"2026-03-10T15:30:00.123456789Z", true},
		{"not-a-date", false},
		{"", false},
		{"2026", false},
	}

	for _, tt := range tests {
		result := formatTimeAgo(tt.input)
		if tt.valid && result == "" {
			t.Errorf("expected non-empty for %q", tt.input)
		}
		if !tt.valid && result != "" {
			t.Errorf("expected empty for %q, got %q", tt.input, result)
		}
	}
}

func TestTruncateEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
	}{
		{"empty", "", 10},
		{"exact", "hello", 5},
		{"shorter", "hi", 10},
		{"longer", "this is very long", 10},
		{"unicode", "héllo wörld", 8},
		{"very short max", "abc", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.max)
			if tt.max >= 3 && len(result) > tt.max {
				t.Errorf("result %q exceeds max %d", result, tt.max)
			}
		})
	}
}

func TestDashModelInitialState(t *testing.T) {
	cfg := config.DefaultConfig()
	m := dashModel{
		cfg:        cfg,
		spinFrames: []string{"⠋", "⠙"},
		loading:    true,
	}

	if !m.loading {
		t.Error("expected loading=true")
	}
	if m.section != sectionDoNow {
		t.Errorf("expected section 0 (DoNow), got %d", m.section)
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}
	if m.viewMode != viewList {
		t.Errorf("expected viewMode list, got %d", m.viewMode)
	}
}

func TestRenderStatusBar(t *testing.T) {
	m := dashModel{spinFrames: []string{"⠋"}}

	// No messages
	result := m.renderStatusBar()
	if result != "" {
		t.Errorf("expected empty status bar, got %q", result)
	}

	// With status
	m.statusMsg = "fetched 5 repos"
	result = m.renderStatusBar()
	if result == "" {
		t.Error("expected non-empty status bar")
	}

	// With error
	m.err = "something failed"
	result = m.renderStatusBar()
	if result == "" {
		t.Error("expected non-empty status bar with error")
	}
}

func TestRenderHelp(t *testing.T) {
	m := dashModel{spinFrames: []string{"⠋"}}

	// Normal section
	m.section = sectionDoNow
	result := m.renderHelp()
	if result == "" {
		t.Error("expected non-empty help")
	}

	// Workspace section should include pull/push/fetch
	m.section = sectionWorkspace
	result = m.renderHelp()
	if result == "" {
		t.Error("expected non-empty workspace help")
	}
}

func TestRenderDetailHelp(t *testing.T) {
	m := dashModel{spinFrames: []string{"⠋"}}
	result := m.renderDetailHelp()
	if result == "" {
		t.Error("expected non-empty detail help")
	}
}

func TestRenderSidebar(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Favorites = []string{"org/repo1", "org/repo2"}
	m := dashModel{
		cfg:        cfg,
		spinFrames: []string{"⠋"},
		doNow:     make([]cache.CachedPR, 3),
		waiting:   make([]cache.CachedPR, 2),
	}

	result := m.renderSidebar()
	if result == "" {
		t.Error("expected non-empty sidebar")
	}
}

func TestRenderMainPanel(t *testing.T) {
	cfg := config.DefaultConfig()
	m := dashModel{
		cfg:        cfg,
		spinFrames: []string{"⠋"},
		loading:    true,
	}

	// Loading state
	result := m.renderMainPanel(80)
	if result == "" {
		t.Error("expected non-empty loading panel")
	}

	// Each section
	m.loading = false
	for _, sec := range []section{sectionDoNow, sectionWaiting, sectionReview, sectionWorkspace, sectionDone} {
		m.section = sec
		result = m.renderMainPanel(80)
		if result == "" {
			t.Errorf("expected non-empty panel for section %v", sec)
		}
	}
}

func TestRenderWorkspaceCard(t *testing.T) {
	m := dashModel{spinFrames: []string{"⠋"}}

	ws := &RepoStatus{
		Name:      "org/repo",
		Branch:    "feature/xyz",
		Behind:    3,
		Ahead:     1,
		Modified:  2,
		Unpushed:  1,
		Clean:     false,
		HasRemote: true,
		LastCommit: "abc123 fix stuff (2h ago)",
	}

	result := m.renderWorkspaceCard(ws, false, 80)
	if result == "" {
		t.Error("expected non-empty workspace card")
	}

	// Selected
	result = m.renderWorkspaceCard(ws, true, 80)
	if result == "" {
		t.Error("expected non-empty selected workspace card")
	}

	// Clean repo
	ws.Clean = true
	ws.Modified = 0
	ws.Behind = 0
	ws.Ahead = 0
	ws.Unpushed = 0
	result = m.renderWorkspaceCard(ws, false, 80)
	if result == "" {
		t.Error("expected non-empty clean workspace card")
	}

	// Way behind
	ws.Behind = 50
	result = m.renderWorkspaceCard(ws, false, 80)
	if result == "" {
		t.Error("expected non-empty behind workspace card")
	}
}

func TestPrBadgeAllCases(t *testing.T) {
	m := dashModel{}

	cases := []cache.CachedPR{
		{PR: gh.PR{ReviewDecision: "APPROVED", Mergeable: "MERGEABLE"}},
		{PR: gh.PR{ReviewDecision: "APPROVED", Mergeable: "CONFLICTING"}},
		{PR: gh.PR{ReviewDecision: "CHANGES_REQUESTED"}},
		{PR: gh.PR{Mergeable: "CONFLICTING"}},
		{PR: gh.PR{IsDraft: true}},
		{PR: gh.PR{ReviewDecision: "REVIEW_REQUIRED"}},
		{PR: gh.PR{}},
	}

	for i, pr := range cases {
		result := m.prBadge(pr)
		if result == "" {
			t.Errorf("case %d: expected non-empty badge", i)
		}
	}
}
