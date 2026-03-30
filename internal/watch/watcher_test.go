package watch

import (
	"context"
	"testing"
	"time"

	"github.com/cheenu1092-oss/prflow/internal/config"
	"github.com/cheenu1092-oss/prflow/internal/gh"
)

// --- Change detection unit tests ---

func TestDetectChanges_NewComment(t *testing.T) {
	old := prSnapshot{State: "OPEN", CommentCount: 2, ReviewCount: 1, CIStatus: "SUCCESS"}
	new := prSnapshot{State: "OPEN", CommentCount: 4, ReviewCount: 1, CIStatus: "SUCCESS"}

	changes := DetectChanges("org/repo#42", "Fix bug", old, new)
	assertHasChangeKind(t, changes, "new_comment")
	if changes[0].PR != 42 {
		t.Errorf("expected PR 42, got %d", changes[0].PR)
	}
}

func TestDetectChanges_ReviewSubmitted(t *testing.T) {
	old := prSnapshot{State: "OPEN", CommentCount: 1, ReviewCount: 0, CIStatus: "SUCCESS"}
	new := prSnapshot{State: "OPEN", CommentCount: 1, ReviewCount: 1, CIStatus: "SUCCESS"}

	changes := DetectChanges("org/repo#10", "Add feature", old, new)
	assertHasChangeKind(t, changes, "review_submitted")
}

func TestDetectChanges_CIChanged(t *testing.T) {
	old := prSnapshot{State: "OPEN", CommentCount: 0, ReviewCount: 0, CIStatus: "PENDING"}
	new := prSnapshot{State: "OPEN", CommentCount: 0, ReviewCount: 0, CIStatus: "FAILURE"}

	changes := DetectChanges("org/repo#5", "Update deps", old, new)
	assertHasChangeKind(t, changes, "ci_changed")
}

func TestDetectChanges_CISucceeded(t *testing.T) {
	old := prSnapshot{State: "OPEN", CommentCount: 0, ReviewCount: 0, CIStatus: "PENDING"}
	new := prSnapshot{State: "OPEN", CommentCount: 0, ReviewCount: 0, CIStatus: "SUCCESS"}

	changes := DetectChanges("org/repo#5", "Update deps", old, new)
	assertHasChangeKind(t, changes, "ci_changed")
	if len(changes) != 1 {
		t.Errorf("expected 1 change, got %d", len(changes))
	}
}

func TestDetectChanges_Merged(t *testing.T) {
	old := prSnapshot{State: "OPEN", CommentCount: 3, ReviewCount: 2, CIStatus: "SUCCESS"}
	new := prSnapshot{State: "MERGED", CommentCount: 3, ReviewCount: 2, CIStatus: "SUCCESS"}

	changes := DetectChanges("org/repo#99", "Ship it", old, new)
	assertHasChangeKind(t, changes, "merged")
}

func TestDetectChanges_Closed(t *testing.T) {
	old := prSnapshot{State: "OPEN", CommentCount: 0, ReviewCount: 0, CIStatus: "UNKNOWN"}
	new := prSnapshot{State: "CLOSED", CommentCount: 0, ReviewCount: 0, CIStatus: "UNKNOWN"}

	changes := DetectChanges("org/repo#7", "WIP", old, new)
	assertHasChangeKind(t, changes, "closed")
}

func TestDetectChanges_ReviewDecisionChanged(t *testing.T) {
	old := prSnapshot{State: "OPEN", ReviewDecision: "", CommentCount: 0, ReviewCount: 1, CIStatus: "SUCCESS"}
	new := prSnapshot{State: "OPEN", ReviewDecision: "APPROVED", CommentCount: 0, ReviewCount: 1, CIStatus: "SUCCESS"}

	changes := DetectChanges("org/repo#20", "Ready", old, new)
	assertHasChangeKind(t, changes, "review_submitted")
}

func TestDetectChanges_NoChange(t *testing.T) {
	snap := prSnapshot{State: "OPEN", CommentCount: 2, ReviewCount: 1, CIStatus: "SUCCESS"}
	changes := DetectChanges("org/repo#1", "Stable", snap, snap)
	if len(changes) != 0 {
		t.Errorf("expected no changes, got %d: %v", len(changes), changes)
	}
}

func TestDetectChanges_MultipleChanges(t *testing.T) {
	old := prSnapshot{State: "OPEN", CommentCount: 1, ReviewCount: 0, CIStatus: "PENDING"}
	new := prSnapshot{State: "OPEN", CommentCount: 3, ReviewCount: 1, CIStatus: "SUCCESS"}

	changes := DetectChanges("org/repo#15", "Big update", old, new)
	if len(changes) < 3 {
		t.Errorf("expected at least 3 changes (comment, review, ci), got %d", len(changes))
	}
	kinds := make(map[string]bool)
	for _, c := range changes {
		kinds[c.Kind] = true
	}
	for _, k := range []string{"new_comment", "review_submitted", "ci_changed"} {
		if !kinds[k] {
			t.Errorf("expected change kind %q, not found in %v", k, changes)
		}
	}
}

// --- Watcher creation test ---

func TestWatcherCreation(t *testing.T) {
	cfg := config.DefaultConfig()
	w := New(cfg, nil, "testuser", 2*time.Minute)
	if w == nil {
		t.Fatal("New returned nil")
	}
	if w.username != "testuser" {
		t.Errorf("expected username testuser, got %s", w.username)
	}
	if w.interval != 2*time.Minute {
		t.Errorf("expected interval 2m, got %s", w.interval)
	}
}

// --- Mock fetcher for integration-style test ---

type mockFetcher struct {
	myPRs    []gh.PR
	requests []gh.PR
	reviewed []gh.PR
	details  map[string]*gh.PR // "repo#number" -> PR
}

func (m *mockFetcher) SearchMyPRs() ([]gh.PR, error)         { return m.myPRs, nil }
func (m *mockFetcher) SearchReviewRequests() ([]gh.PR, error) { return m.requests, nil }
func (m *mockFetcher) SearchReviewedPRs() ([]gh.PR, error)    { return m.reviewed, nil }
func (m *mockFetcher) GetPRDetail(repo string, number int) (*gh.PR, error) {
	key := snapshotKey(repo, number)
	if pr, ok := m.details[key]; ok {
		return pr, nil
	}
	// Return a minimal PR
	return &gh.PR{Number: number, State: "OPEN", Repository: gh.RepoRef{NameWithOwner: repo}}, nil
}

func TestWatcherPollDetectsNewReviewRequest(t *testing.T) {
	cfg := config.DefaultConfig()
	mf := &mockFetcher{
		myPRs:    nil,
		requests: nil,
		reviewed: nil,
		details:  make(map[string]*gh.PR),
	}
	w := newWithFetcher(cfg, nil, "testuser", time.Minute, mf)

	ctx := context.Background()

	// Seed initial state (no PRs)
	if err := w.poll(ctx, true); err != nil {
		t.Fatalf("seed poll: %v", err)
	}

	// Now add a review request
	mf.requests = []gh.PR{
		{Number: 50, Title: "Need review", State: "OPEN",
			Repository: gh.RepoRef{NameWithOwner: "org/repo"}},
	}
	mf.details["org/repo#50"] = &gh.PR{
		Number: 50, Title: "Need review", State: "OPEN",
		Repository: gh.RepoRef{NameWithOwner: "org/repo"},
	}

	// This poll should detect the new PR (not seed, so notifications fire)
	if err := w.poll(ctx, false); err != nil {
		t.Fatalf("poll: %v", err)
	}

	// Verify state was updated
	if _, ok := w.lastState["org/repo#50"]; !ok {
		t.Error("expected org/repo#50 in lastState after poll")
	}
}

func TestWatcherRunCancelledContext(t *testing.T) {
	cfg := config.DefaultConfig()
	mf := &mockFetcher{details: make(map[string]*gh.PR)}
	w := newWithFetcher(cfg, nil, "testuser", 50*time.Millisecond, mf)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := w.Run(ctx)
	if err != nil {
		t.Errorf("Run with cancelled context should return nil, got: %v", err)
	}
}

// --- helpers ---

func assertHasChangeKind(t *testing.T, changes []Change, kind string) {
	t.Helper()
	for _, c := range changes {
		if c.Kind == kind {
			return
		}
	}
	t.Errorf("expected change kind %q in %v", kind, changes)
}
