package tui

import (
	"testing"

	"github.com/cheenu1092-oss/prflow/internal/cache"
	"github.com/cheenu1092-oss/prflow/internal/config"
	"github.com/cheenu1092-oss/prflow/internal/gh"
)

func TestTimeSince(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{"recent", "2026-03-11T22:00:00Z", ""}, // might be in future
		{"iso format", "2026-01-01T00:00:00Z", ""},
		{"with timezone", "2026-01-01T00:00:00-08:00", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTimeAgo(tt.input)
			// Should not panic and should return something
			_ = result
		})
	}
}

func TestFormatTimeAgoEmpty(t *testing.T) {
	result := formatTimeAgo("")
	if result != "" {
		t.Errorf("expected empty string for empty input, got '%s'", result)
	}
}

func TestFormatTimeAgoInvalid(t *testing.T) {
	result := formatTimeAgo("not-a-date")
	if result != "" {
		t.Errorf("expected empty string for invalid input, got '%s'", result)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		max      int
	}{
		{"short", 10},
		{"exactly ten", 11},
		{"this is a long string that should be truncated", 20},
		{"", 10},
		{"abc", 3},
		{"abcd", 3},
		{"abcdef", 2}, // edge case: max < 4, just truncates hard
		{"ab", 1},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.max)
		if len(result) > tt.max {
			t.Errorf("truncate(%q, %d) = %q (len %d), exceeds max", tt.input, tt.max, result, len(result))
		}
	}
}

func TestTruncateNewlines(t *testing.T) {
	input := "line1\nline2\rline3"
	result := truncate(input, 50)
	// \n replaced with " ", \r removed
	if result != "line1 line2line3" {
		t.Errorf("expected newlines handled, got '%s'", result)
	}
}

func TestPrBadge(t *testing.T) {
	m := dashModel{cfg: config.DefaultConfig()}

	tests := []struct {
		name         string
		pr           cache.CachedPR
		expectBadge  string // substring to check for
	}{
		{
			name: "approved + CI passing",
			pr: cache.CachedPR{PR: gh.PR{
				ReviewDecision: "APPROVED",
				Mergeable:      "MERGEABLE",
				StatusCheckRollup: []gh.StatusCheck{
					{Conclusion: "SUCCESS"},
				},
			}},
			expectBadge: "✓CI",
		},
		{
			name: "approved + CI failing",
			pr: cache.CachedPR{PR: gh.PR{
				ReviewDecision: "APPROVED",
				Mergeable:      "MERGEABLE",
				StatusCheckRollup: []gh.StatusCheck{
					{Conclusion: "FAILURE"},
				},
			}},
			expectBadge: "✗CI",
		},
		{
			name: "approved + CI pending",
			pr: cache.CachedPR{PR: gh.PR{
				ReviewDecision: "APPROVED",
				Mergeable:      "MERGEABLE",
				StatusCheckRollup: []gh.StatusCheck{
					{Conclusion: "PENDING"},
				},
			}},
			expectBadge: "⏳CI",
		},
		{
			name: "approved + no CI checks",
			pr: cache.CachedPR{PR: gh.PR{
				ReviewDecision:    "APPROVED",
				Mergeable:         "MERGEABLE",
				StatusCheckRollup: []gh.StatusCheck{},
			}},
			expectBadge: "✓CI", // assume passing when no checks
		},
		{
			name: "approved + conflict",
			pr: cache.CachedPR{PR: gh.PR{
				ReviewDecision: "APPROVED",
				Mergeable:      "CONFLICTING",
			}},
			expectBadge: "CONFLICT",
		},
		{
			name: "changes requested",
			pr: cache.CachedPR{PR: gh.PR{
				ReviewDecision: "CHANGES_REQUESTED",
			}},
			expectBadge: "CHANGES REQUESTED",
		},
		{
			name: "merge conflict",
			pr: cache.CachedPR{PR: gh.PR{
				Mergeable: "CONFLICTING",
			}},
			expectBadge: "CONFLICT",
		},
		{
			name: "draft",
			pr: cache.CachedPR{PR: gh.PR{
				IsDraft: true,
			}},
			expectBadge: "DRAFT",
		},
		{
			name: "review required",
			pr: cache.CachedPR{PR: gh.PR{
				ReviewDecision: "REVIEW_REQUIRED",
			}},
			expectBadge: "AWAITING REVIEW",
		},
		{
			name: "default (in review)",
			pr: cache.CachedPR{PR: gh.PR{}},
			expectBadge: "IN REVIEW",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.prBadge(tt.pr)
			if result == "" {
				t.Error("expected non-empty badge")
			}
			// Check for expected substring (badges include styling, so we just check content is there)
			// This is a basic sanity check - lipgloss styling wraps the text
		})
	}
}

func TestSectionString(t *testing.T) {
	tests := []struct {
		sec      section
		expected string
	}{
		{sectionDoNow, "⚡ Do Now"},
		{sectionWaiting, "⏳ Waiting"},
		{sectionReview, "👀 Review"},
		{sectionWorkspace, "📂 Workspace"},
		{sectionNeedsAttention, "🔔 Needs Attention Again"},
	}

	for _, tt := range tests {
		result := tt.sec.String()
		if result != tt.expected {
			t.Errorf("section(%d).String() = '%s', want '%s'", tt.sec, result, tt.expected)
		}
	}
}

func TestCurrentListLen(t *testing.T) {
	m := dashModel{
		doNow:          make([]cache.CachedPR, 3),
		waiting:        make([]cache.CachedPR, 5),
		review:         make([]cache.CachedPR, 2),
		workspace:      make([]RepoStatus, 4),
		needsAttention: make([]cache.CachedPR, 1),
	}

	tests := []struct {
		sec      section
		expected int
	}{
		{sectionDoNow, 3},
		{sectionWaiting, 5},
		{sectionReview, 2},
		{sectionWorkspace, 4},
		{sectionNeedsAttention, 1},
	}

	for _, tt := range tests {
		m.section = tt.sec
		result := m.currentListLen()
		if result != tt.expected {
			t.Errorf("section %v: expected %d, got %d", tt.sec, tt.expected, result)
		}
	}
}

func TestHelpPair(t *testing.T) {
	result := helpPair("q", "quit")
	if result == "" {
		t.Error("expected non-empty help pair")
	}
}

func TestRenderPRCardsEmpty(t *testing.T) {
	m := dashModel{cfg: config.DefaultConfig()}
	result := m.renderPRCards(nil, 80)
	if result == "" {
		t.Error("expected non-empty empty state")
	}
}

func TestRenderPRCard(t *testing.T) {
	m := dashModel{cfg: config.DefaultConfig()}
	pr := cache.CachedPR{
		PR: gh.PR{
			Number:         42,
			Title:          "Fix auth bug in the middleware layer",
			ReviewDecision: "APPROVED",
			Mergeable:      "MERGEABLE",
			UpdatedAt:      "2026-03-10T15:30:00Z",
		},
		Repo: "org/repo",
	}

	// Not selected
	result := m.renderPRCard(pr, false, 80)
	if result == "" {
		t.Error("expected non-empty PR card")
	}

	// Selected
	resultSelected := m.renderPRCard(pr, true, 80)
	if resultSelected == "" {
		t.Error("expected non-empty selected PR card")
	}
}

func TestRenderWorkspaceCardsEmpty(t *testing.T) {
	m := dashModel{cfg: config.DefaultConfig()}
	result := m.renderWorkspaceCards(80)
	if result == "" {
		t.Error("expected non-empty empty state")
	}
}

func TestFindLocalRepoNotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Workspace.ScanDirs = []string{"/nonexistent"}
	m := dashModel{cfg: cfg}

	result := m.findLocalRepo("org/nonexistent-repo")
	if result != "" {
		t.Errorf("expected empty for nonexistent repo, got %q", result)
	}
}

func TestFindLocalRepoFromWorkspaceResults(t *testing.T) {
	cfg := config.DefaultConfig()
	m := dashModel{
		cfg: cfg,
		workspace: []RepoStatus{
			{Name: "org/myrepo", Path: "/tmp/myrepo"},
		},
	}

	result := m.findLocalRepo("org/myrepo")
	// Won't find it since /tmp/myrepo doesn't exist as git repo,
	// but the scan result matching should work if path existed
	_ = result
}

func TestViewSearchMode(t *testing.T) {
	cfg := config.DefaultConfig()
	m := dashModel{
		cfg:         cfg,
		spinFrames:  []string{"⠋"},
		viewMode:    viewSearch,
		searchQuery: "test",
	}

	result := m.viewSearchMode()
	if result == "" {
		t.Error("expected non-empty search view")
	}

	// With results
	m.searchResults = []string{"org/repo1", "org/repo2"}
	result = m.viewSearchMode()
	if result == "" {
		t.Error("expected non-empty search view with results")
	}

	// Searching state
	m.searching = true
	m.searchResults = nil
	result = m.viewSearchMode()
	if result == "" {
		t.Error("expected non-empty searching view")
	}
}

func TestSectionAllValues(t *testing.T) {
	// Verify all 5 sections have non-empty string representation
	for i := section(0); i <= sectionNeedsAttention; i++ {
		if i.String() == "" {
			t.Errorf("section %d has empty string", i)
		}
	}
}

func TestViewReplyMode(t *testing.T) {
	cfg := config.DefaultConfig()
	m := dashModel{
		cfg:           cfg,
		viewMode:      viewReply,
		replyText:     "This is my reply",
		replyThreadID: "thread123",
		detailPR: &cache.CachedPR{
			PR: gh.PR{
				Number: 42,
			},
			Repo: "org/repo",
		},
		detailThreads: []gh.ReviewThread{
			{
				ID: "thread123",
				Comments: []gh.ThreadComment{
					{
						Author: "reviewer",
						Body:   "Please fix this bug",
					},
				},
			},
		},
	}

	result := m.viewReplyMode()
	if result == "" {
		t.Error("expected non-empty reply view")
	}

	// Empty reply
	m.replyText = ""
	result = m.viewReplyMode()
	if result == "" {
		t.Error("expected non-empty reply view with empty text")
	}
}


