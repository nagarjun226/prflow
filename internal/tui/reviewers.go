package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cheenu1092-oss/prflow/internal/gh"
)

// ReviewerStatus represents the state and wait time for a reviewer
type ReviewerStatus struct {
	Login       string
	State       string // APPROVED, CHANGES_REQUESTED, COMMENTED, PENDING
	LastUpdated time.Time
	WaitDays    int
	IsPending   bool // true if no review submitted yet
}

// CalculateReviewerStatus computes wait times and states for all reviewers
func CalculateReviewerStatus(pr *gh.PR) []ReviewerStatus {
	var statuses []ReviewerStatus
	seen := make(map[string]bool)

	// Track reviewers who have submitted reviews
	reviewedBy := make(map[string]ReviewerStatus)
	for _, rev := range pr.Reviews.Nodes {
		login := rev.Author.Login
		if login == "" || login == pr.Author.Login {
			continue // skip author
		}

		submittedAt, err := parseTime(rev.SubmittedAt)
		if err != nil {
			continue
		}

		// Keep the most recent review per reviewer
		if existing, ok := reviewedBy[login]; !ok || submittedAt.After(existing.LastUpdated) {
			reviewedBy[login] = ReviewerStatus{
				Login:       login,
				State:       rev.State,
				LastUpdated: submittedAt,
				WaitDays:    daysSince(submittedAt),
				IsPending:   false,
			}
		}
	}

	// Track pending reviewers (requested but no review yet)
	for _, req := range pr.ReviewRequests.Nodes {
		login := req.RequestedReviewer.Login
		if login == "" || login == pr.Author.Login {
			continue
		}

		// If they haven't reviewed yet, mark as pending
		if _, reviewed := reviewedBy[login]; !reviewed {
			// Use PR creation time as baseline for pending reviewers
			createdAt, err := parseTime(pr.CreatedAt)
			if err != nil {
				createdAt = time.Now()
			}
			reviewedBy[login] = ReviewerStatus{
				Login:       login,
				State:       "PENDING",
				LastUpdated: createdAt,
				WaitDays:    daysSince(createdAt),
				IsPending:   true,
			}
		}
	}

	// Convert map to slice
	for login, status := range reviewedBy {
		if !seen[login] {
			statuses = append(statuses, status)
			seen[login] = true
		}
	}

	// Sort by wait time descending (most stale first)
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].WaitDays > statuses[j].WaitDays
	})

	return statuses
}

// RenderReviewerWaitTimes creates a color-coded display of reviewer wait times
func RenderReviewerWaitTimes(statuses []ReviewerStatus) string {
	if len(statuses) == 0 {
		return wsMetaStyle.Render("No reviewers")
	}

	var lines []string
	for _, s := range statuses {
		icon := reviewerIcon(s.State)
		waitStr := formatWaitTime(s.WaitDays, s.IsPending)
		nameStr := threadAuthorStyle.Render("@" + s.Login)
		stateStr := formatReviewState(s.State)

		line := fmt.Sprintf("%s %s  %s  %s", icon, nameStr, stateStr, waitStr)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// reviewerIcon returns an icon based on review state
func reviewerIcon(state string) string {
	switch state {
	case "APPROVED":
		return wsCleanStyle.Render("✓")
	case "CHANGES_REQUESTED":
		return wsBehindStyle.Render("✗")
	case "COMMENTED":
		return wsDirtyStyle.Render("💬")
	case "PENDING":
		return wsMetaStyle.Render("⏳")
	default:
		return wsMetaStyle.Render("•")
	}
}

// formatReviewState formats the state label
func formatReviewState(state string) string {
	switch state {
	case "APPROVED":
		return wsCleanStyle.Render("APPROVED")
	case "CHANGES_REQUESTED":
		return wsBehindStyle.Render("CHANGES REQUESTED")
	case "COMMENTED":
		return wsDirtyStyle.Render("COMMENTED")
	case "PENDING":
		return wsMetaStyle.Render("PENDING")
	default:
		return wsMetaStyle.Render(state)
	}
}

// formatWaitTime creates a color-coded wait time string
func formatWaitTime(days int, isPending bool) string {
	var waitStr string
	if days == 0 {
		waitStr = "today"
	} else if days == 1 {
		waitStr = "1 day"
	} else {
		waitStr = fmt.Sprintf("%d days", days)
	}

	// Color-code based on severity
	style := wsCleanStyle // <1 day: green
	if days >= 1 && days < 3 {
		style = wsDirtyStyle // 1-2 days: yellow
	} else if days >= 3 {
		style = wsBehindStyle // 3+ days: red
	}

	prefix := ""
	if isPending {
		prefix = "waiting "
	} else {
		prefix = "responded "
	}

	return style.Render(prefix + waitStr + " ago")
}

// parseTime parses GitHub timestamp formats
func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		time.RFC3339Nano,
	}
	for _, layout := range formats {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp: %s", s)
}

// daysSince calculates days elapsed since a timestamp
func daysSince(t time.Time) int {
	if t.IsZero() {
		return 0
	}
	d := time.Since(t)
	if d < 0 {
		return 0
	}
	return int(d.Hours() / 24)
}
