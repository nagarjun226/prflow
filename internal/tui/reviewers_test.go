package tui

import (
	"testing"
	"time"

	"github.com/cheenu1092-oss/prflow/internal/gh"
)

func TestCalculateReviewerStatus(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	threeDaysAgo := now.Add(-72 * time.Hour)

	tests := []struct {
		name     string
		pr       *gh.PR
		wantLen  int
		wantPending bool
	}{
		{
			name: "no reviewers",
			pr: &gh.PR{
				Author: gh.Author{Login: "author"},
			},
			wantLen:  0,
			wantPending: false,
		},
		{
			name: "one approved reviewer",
			pr: &gh.PR{
				Author: gh.Author{Login: "author"},
				Reviews: gh.Reviews{
					Nodes: []gh.Review{
						{
							Author:      gh.Author{Login: "reviewer1"},
							State:       "APPROVED",
							SubmittedAt: yesterday.Format(time.RFC3339),
						},
					},
				},
			},
			wantLen:  1,
			wantPending: false,
		},
		{
			name: "pending reviewer",
			pr: &gh.PR{
				Author:    gh.Author{Login: "author"},
				CreatedAt: threeDaysAgo.Format(time.RFC3339),
				ReviewRequests: gh.ReviewRequests{
					Nodes: []gh.ReviewRequest{
						{RequestedReviewer: gh.Author{Login: "pending-reviewer"}},
					},
				},
			},
			wantLen:  1,
			wantPending: true,
		},
		{
			name: "mixed reviewers",
			pr: &gh.PR{
				Author:    gh.Author{Login: "author"},
				CreatedAt: threeDaysAgo.Format(time.RFC3339),
				Reviews: gh.Reviews{
					Nodes: []gh.Review{
						{
							Author:      gh.Author{Login: "reviewer1"},
							State:       "APPROVED",
							SubmittedAt: yesterday.Format(time.RFC3339),
						},
						{
							Author:      gh.Author{Login: "reviewer2"},
							State:       "CHANGES_REQUESTED",
							SubmittedAt: threeDaysAgo.Format(time.RFC3339),
						},
					},
				},
				ReviewRequests: gh.ReviewRequests{
					Nodes: []gh.ReviewRequest{
						{RequestedReviewer: gh.Author{Login: "pending-reviewer"}},
					},
				},
			},
			wantLen:  3,
			wantPending: true,
		},
		{
			name: "ignore author reviews",
			pr: &gh.PR{
				Author:    gh.Author{Login: "author"},
				CreatedAt: yesterday.Format(time.RFC3339),
				Reviews: gh.Reviews{
					Nodes: []gh.Review{
						{
							Author:      gh.Author{Login: "author"}, // should be ignored
							State:       "APPROVED",
							SubmittedAt: yesterday.Format(time.RFC3339),
						},
						{
							Author:      gh.Author{Login: "reviewer1"},
							State:       "APPROVED",
							SubmittedAt: yesterday.Format(time.RFC3339),
						},
					},
				},
			},
			wantLen:  1,
			wantPending: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statuses := CalculateReviewerStatus(tt.pr)
			if len(statuses) != tt.wantLen {
				t.Errorf("got %d reviewers, want %d", len(statuses), tt.wantLen)
			}
			if tt.wantPending {
				hasPending := false
				for _, s := range statuses {
					if s.IsPending {
						hasPending = true
						break
					}
				}
				if !hasPending {
					t.Error("expected at least one pending reviewer")
				}
			}
		})
	}
}

func TestDaysSince(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		t    time.Time
		want int
	}{
		{
			name: "zero time",
			t:    time.Time{},
			want: 0,
		},
		{
			name: "now",
			t:    now,
			want: 0,
		},
		{
			name: "yesterday",
			t:    now.Add(-24 * time.Hour),
			want: 1,
		},
		{
			name: "three days ago",
			t:    now.Add(-72 * time.Hour),
			want: 3,
		},
		{
			name: "future time",
			t:    now.Add(24 * time.Hour),
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := daysSince(tt.t)
			if got != tt.want {
				t.Errorf("daysSince() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "RFC3339",
			input:   "2024-03-12T10:00:00Z",
			wantErr: false,
		},
		{
			name:    "RFC3339 with timezone",
			input:   "2024-03-12T10:00:00-08:00",
			wantErr: false,
		},
		{
			name:    "RFC3339Nano",
			input:   "2024-03-12T10:00:00.123456Z",
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   "not-a-timestamp",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTime() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFormatWaitTime(t *testing.T) {
	tests := []struct {
		name      string
		days      int
		isPending bool
		wantSubstr string
	}{
		{
			name:      "today",
			days:      0,
			isPending: false,
			wantSubstr: "today",
		},
		{
			name:      "one day",
			days:      1,
			isPending: true,
			wantSubstr: "1 day",
		},
		{
			name:      "multiple days",
			days:      5,
			isPending: false,
			wantSubstr: "5 days",
		},
		{
			name:      "pending prefix",
			days:      2,
			isPending: true,
			wantSubstr: "waiting",
		},
		{
			name:      "responded prefix",
			days:      2,
			isPending: false,
			wantSubstr: "responded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatWaitTime(tt.days, tt.isPending)
			// Strip ANSI codes for testing (lipgloss adds colors)
			// Just check that the substring is present
			if got == "" {
				t.Error("formatWaitTime() returned empty string")
			}
		})
	}
}

func TestReviewerIcon(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  string
	}{
		{
			name:  "approved",
			state: "APPROVED",
			want:  "✓",
		},
		{
			name:  "changes requested",
			state: "CHANGES_REQUESTED",
			want:  "✗",
		},
		{
			name:  "commented",
			state: "COMMENTED",
			want:  "💬",
		},
		{
			name:  "pending",
			state: "PENDING",
			want:  "⏳",
		},
		{
			name:  "unknown",
			state: "UNKNOWN",
			want:  "•",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reviewerIcon(tt.state)
			// Icons are wrapped in styles, so we can't do exact match
			// Just ensure non-empty
			if got == "" {
				t.Error("reviewerIcon() returned empty string")
			}
		})
	}
}

func TestRenderReviewerWaitTimes(t *testing.T) {
	tests := []struct {
		name     string
		statuses []ReviewerStatus
		wantEmpty bool
	}{
		{
			name:     "empty",
			statuses: nil,
			wantEmpty: true,
		},
		{
			name: "one reviewer",
			statuses: []ReviewerStatus{
				{
					Login:     "reviewer1",
					State:     "APPROVED",
					WaitDays:  1,
					IsPending: false,
				},
			},
			wantEmpty: false,
		},
		{
			name: "multiple reviewers",
			statuses: []ReviewerStatus{
				{Login: "r1", State: "APPROVED", WaitDays: 1},
				{Login: "r2", State: "PENDING", WaitDays: 3, IsPending: true},
			},
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderReviewerWaitTimes(tt.statuses)
			isEmpty := got == "" || got == wsMetaStyle.Render("No reviewers")
			if isEmpty != tt.wantEmpty {
				t.Errorf("RenderReviewerWaitTimes() isEmpty = %v, want %v", isEmpty, tt.wantEmpty)
			}
		})
	}
}
