package tui

import (
	"testing"
	"time"

	"github.com/cheenu1092-oss/prflow/internal/cache"
	"github.com/cheenu1092-oss/prflow/internal/gh"
)

func TestUrgencyScore(t *testing.T) {
	tests := []struct {
		name          string
		pr            cache.CachedPR
		expectedRange [2]int // [min, max] score range
	}{
		{
			name: "merge conflict - highest urgency",
			pr: cache.CachedPR{
				PR: gh.PR{
					Mergeable: "CONFLICTING",
				},
			},
			expectedRange: [2]int{1000, 1200},
		},
		{
			name: "changes requested - high urgency",
			pr: cache.CachedPR{
				PR: gh.PR{
					ReviewDecision: "CHANGES_REQUESTED",
					Mergeable:      "MERGEABLE",
				},
			},
			expectedRange: [2]int{800, 900},
		},
		{
			name: "approved + ready - medium-high urgency",
			pr: cache.CachedPR{
				PR: gh.PR{
					ReviewDecision: "APPROVED",
					Mergeable:      "MERGEABLE",
				},
			},
			expectedRange: [2]int{600, 750},
		},
		{
			name: "draft PR - lower priority",
			pr: cache.CachedPR{
				PR: gh.PR{
					IsDraft:   true,
					Mergeable: "MERGEABLE",
				},
			},
			expectedRange: [2]int{-500, 0},
		},
		{
			name: "stale reviewers (7+ days) - urgent",
			pr: cache.CachedPR{
				PR: gh.PR{
					CreatedAt: time.Now().Add(-8 * 24 * time.Hour).Format(time.RFC3339),
					UpdatedAt: time.Now().Add(-8 * 24 * time.Hour).Format(time.RFC3339),
					ReviewRequests: gh.ReviewRequests{
						Nodes: []gh.ReviewRequest{
							{RequestedReviewer: gh.Author{Login: "reviewer1"}},
						},
					},
				},
			},
			expectedRange: [2]int{400, 500},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := UrgencyScore(tt.pr)
			if score < tt.expectedRange[0] || score > tt.expectedRange[1] {
				t.Errorf("UrgencyScore() = %d, want range [%d, %d]",
					score, tt.expectedRange[0], tt.expectedRange[1])
			}
		})
	}
}

func TestSortByUrgency(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	lastWeek := now.Add(-7 * 24 * time.Hour)

	prs := []cache.CachedPR{
		{
			PR: gh.PR{
				Number:         1,
				Title:          "Normal PR",
				UpdatedAt:      now.Format(time.RFC3339),
				ReviewDecision: "",
				Mergeable:      "MERGEABLE",
			},
		},
		{
			PR: gh.PR{
				Number:         2,
				Title:          "Merge conflict",
				UpdatedAt:      yesterday.Format(time.RFC3339),
				Mergeable:      "CONFLICTING",
			},
		},
		{
			PR: gh.PR{
				Number:         3,
				Title:          "Changes requested",
				UpdatedAt:      yesterday.Format(time.RFC3339),
				ReviewDecision: "CHANGES_REQUESTED",
				Mergeable:      "MERGEABLE",
			},
		},
		{
			PR: gh.PR{
				Number:         4,
				Title:          "Ready to merge",
				UpdatedAt:      yesterday.Format(time.RFC3339),
				ReviewDecision: "APPROVED",
				Mergeable:      "MERGEABLE",
			},
		},
		{
			PR: gh.PR{
				Number:    5,
				Title:     "Stale reviewer",
				CreatedAt: lastWeek.Format(time.RFC3339),
				UpdatedAt: lastWeek.Format(time.RFC3339),
				ReviewRequests: gh.ReviewRequests{
					Nodes: []gh.ReviewRequest{
						{RequestedReviewer: gh.Author{Login: "reviewer1"}},
					},
				},
			},
		},
	}

	SortByUrgency(prs)

	// Expected order (by urgency):
	// 1. #2 Merge conflict (score ~1000)
	// 2. #3 Changes requested (score ~800)
	// 3. #4 Ready to merge (score ~600)
	// 4. #5 Stale reviewer (score ~400)
	// 5. #1 Normal PR (score ~0-50)

	expectedOrder := []int{2, 3, 4, 5, 1}
	for i, expectedNum := range expectedOrder {
		if prs[i].Number != expectedNum {
			t.Errorf("After sorting, position %d = PR #%d, want #%d",
				i, prs[i].Number, expectedNum)
		}
	}
}

func TestHasPassingCI(t *testing.T) {
	tests := []struct {
		name     string
		pr       cache.CachedPR
		expected bool
	}{
		{
			name: "no CI checks - assume passing",
			pr: cache.CachedPR{
				PR: gh.PR{
					StatusCheckRollup: []gh.StatusCheck{},
				},
			},
			expected: true,
		},
		{
			name: "all checks passing",
			pr: cache.CachedPR{
				PR: gh.PR{
					StatusCheckRollup: []gh.StatusCheck{
						{Conclusion: "SUCCESS"},
						{Conclusion: "SUCCESS"},
					},
				},
			},
			expected: true,
		},
		{
			name: "one check failing",
			pr: cache.CachedPR{
				PR: gh.PR{
					StatusCheckRollup: []gh.StatusCheck{
						{Conclusion: "SUCCESS"},
						{Conclusion: "FAILURE"},
					},
				},
			},
			expected: false,
		},
		{
			name: "neutral checks count as passing",
			pr: cache.CachedPR{
				PR: gh.PR{
					StatusCheckRollup: []gh.StatusCheck{
						{Conclusion: "SUCCESS"},
						{Conclusion: "NEUTRAL"},
						{Conclusion: "SKIPPED"},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasPassingCI(tt.pr)
			if result != tt.expected {
				t.Errorf("HasPassingCI() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDaysSinceUpdate(t *testing.T) {
	tests := []struct {
		name      string
		updatedAt string
		wantDays  int
	}{
		{
			name:      "today",
			updatedAt: time.Now().Format(time.RFC3339),
			wantDays:  0,
		},
		{
			name:      "yesterday",
			updatedAt: time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			wantDays:  1,
		},
		{
			name:      "one week ago",
			updatedAt: time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339),
			wantDays:  7,
		},
		{
			name:      "invalid timestamp",
			updatedAt: "invalid",
			wantDays:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			days := DaysSinceUpdate(tt.updatedAt)
			// Allow ±1 day for timezone/timing differences
			if days < tt.wantDays-1 || days > tt.wantDays+1 {
				t.Errorf("DaysSinceUpdate() = %d, want ~%d", days, tt.wantDays)
			}
		})
	}
}
