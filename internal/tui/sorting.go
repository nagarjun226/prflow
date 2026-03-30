package tui

import (
	"sort"

	"github.com/cheenu1092-oss/prflow/internal/cache"
)

// DefaultStaleThresholdDays is used when no config is available
const DefaultStaleThresholdDays = 3

// UrgencyScore calculates a priority score for a PR (higher = more urgent).
// staleThresholdDays controls when a reviewer wait becomes "stale" (default: 3).
func UrgencyScore(pr cache.CachedPR, staleThresholdDays ...int) int {
	threshold := DefaultStaleThresholdDays
	if len(staleThresholdDays) > 0 && staleThresholdDays[0] > 0 {
		threshold = staleThresholdDays[0]
	}

	score := 0

	// Critical: Merge conflicts (highest urgency)
	if pr.Mergeable == "CONFLICTING" {
		score += 1000
	}

	// High: Changes requested (blocks progress)
	if pr.ReviewDecision == "CHANGES_REQUESTED" {
		score += 800
	}

	// Medium-High: Approved and ready to merge (quick win)
	if pr.ReviewDecision == "APPROVED" && pr.Mergeable != "CONFLICTING" {
		score += 600

		// Bonus: CI passing (truly ready)
		if HasPassingCI(pr) {
			score += 100
		}
	}

	// Medium: Waiting on reviewers (time-based urgency relative to threshold)
	reviewerStatuses := CalculateReviewerStatus(&pr.PR)
	if len(reviewerStatuses) > 0 {
		stalest := reviewerStatuses[0] // already sorted by wait time descending

		// Scale urgency relative to configured threshold
		if stalest.WaitDays >= threshold*2+1 {
			score += 400 // 2x+ threshold = very urgent
		} else if stalest.WaitDays >= threshold+2 {
			score += 300
		} else if stalest.WaitDays >= threshold {
			score += 200
		} else if stalest.WaitDays >= 1 {
			score += 100
		}
	}

	// Low: Age of PR (newer = slightly higher priority for review section)
	daysSinceUpdate := DaysSinceUpdate(pr.UpdatedAt)
	if daysSinceUpdate > 0 {
		// Older PRs get a small boost (within same category)
		score += min(daysSinceUpdate*5, 50)
	}

	// Penalty: Draft PRs are lower priority
	if pr.IsDraft {
		score -= 500
	}

	return score
}

// HasPassingCI checks if all status checks are passing
func HasPassingCI(pr cache.CachedPR) bool {
	if len(pr.StatusCheckRollup) == 0 {
		return true // no CI = assume passing (or doesn't matter)
	}
	
	for _, check := range pr.StatusCheckRollup {
		if check.Conclusion != "SUCCESS" && check.Conclusion != "NEUTRAL" && check.Conclusion != "SKIPPED" {
			return false
		}
	}
	return true
}

// DaysSinceUpdate calculates days since last PR update
func DaysSinceUpdate(updatedAt string) int {
	t, err := parseTime(updatedAt)
	if err != nil {
		return 0
	}
	return daysSince(t)
}

// SortByUrgency sorts a slice of PRs by urgency score (highest first).
// staleThresholdDays controls when a reviewer wait becomes "stale" (default: 3).
func SortByUrgency(prs []cache.CachedPR, staleThresholdDays ...int) {
	threshold := DefaultStaleThresholdDays
	if len(staleThresholdDays) > 0 && staleThresholdDays[0] > 0 {
		threshold = staleThresholdDays[0]
	}
	sort.SliceStable(prs, func(i, j int) bool {
		scoreI := UrgencyScore(prs[i], threshold)
		scoreJ := UrgencyScore(prs[j], threshold)

		// Higher score = more urgent = comes first
		if scoreI != scoreJ {
			return scoreI > scoreJ
		}

		// Tie-breaker: more recent updates first
		return prs[i].UpdatedAt > prs[j].UpdatedAt
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
