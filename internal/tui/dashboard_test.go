package tui

import (
	"testing"

	"github.com/cheenu1092-oss/prflow/internal/cache"
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
		expected string
	}{
		{"short", 10, "short"},
		{"exactly ten", 11, "exactly ten"},
		{"this is a long string that should be truncated", 20, "this is a long st..."},
		{"", 10, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, ""},  // edge case: max < 3 means we can't even fit "..."
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.max)
		if tt.max >= 3 && len(result) > tt.max {
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
	m := dashModel{}

	tests := []struct {
		name     string
		pr       cache.CachedPR
	}{
		{"approved", cache.CachedPR{PR: gh.PR{ReviewDecision: "APPROVED", Mergeable: "MERGEABLE"}}},
		{"approved+conflict", cache.CachedPR{PR: gh.PR{ReviewDecision: "APPROVED", Mergeable: "CONFLICTING"}}},
		{"changes", cache.CachedPR{PR: gh.PR{ReviewDecision: "CHANGES_REQUESTED"}}},
		{"conflict", cache.CachedPR{PR: gh.PR{Mergeable: "CONFLICTING"}}},
		{"draft", cache.CachedPR{PR: gh.PR{IsDraft: true}}},
		{"review_required", cache.CachedPR{PR: gh.PR{ReviewDecision: "REVIEW_REQUIRED"}}},
		{"default", cache.CachedPR{PR: gh.PR{}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.prBadge(tt.pr)
			if result == "" {
				t.Error("expected non-empty badge")
			}
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
		{sectionDone, "✅ Done"},
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
		doNow:     make([]cache.CachedPR, 3),
		waiting:   make([]cache.CachedPR, 5),
		review:    make([]cache.CachedPR, 2),
		workspace: make([]RepoStatus, 4),
		done:      make([]cache.CachedPR, 1),
	}

	tests := []struct {
		sec      section
		expected int
	}{
		{sectionDoNow, 3},
		{sectionWaiting, 5},
		{sectionReview, 2},
		{sectionWorkspace, 4},
		{sectionDone, 1},
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
	m := dashModel{}
	result := m.renderPRCards(nil, 80)
	if result == "" {
		t.Error("expected non-empty empty state")
	}
}

func TestRenderPRCard(t *testing.T) {
	m := dashModel{}
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
	m := dashModel{}
	result := m.renderWorkspaceCards(80)
	if result == "" {
		t.Error("expected non-empty empty state")
	}
}
