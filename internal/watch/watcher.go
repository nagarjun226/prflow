package watch

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cheenu1092-oss/prflow/internal/cache"
	"github.com/cheenu1092-oss/prflow/internal/config"
	"github.com/cheenu1092-oss/prflow/internal/gh"
	"github.com/cheenu1092-oss/prflow/internal/notify"
)

// Change represents a detected change on a PR.
type Change struct {
	Kind    string // new_comment, review_submitted, ci_changed, merged, closed, new_review_request
	Repo    string
	PR      int
	Title   string
	Message string
}

// prSnapshot captures the state of a PR at a point in time for diffing.
type prSnapshot struct {
	State          string
	CommentCount   int
	ReviewCount    int
	CIStatus       string
	ReviewDecision string
}

// fetcher abstracts the GitHub API so tests can inject mocks.
type fetcher interface {
	SearchMyPRs() ([]gh.PR, error)
	SearchReviewRequests() ([]gh.PR, error)
	SearchReviewedPRs() ([]gh.PR, error)
	GetPRDetail(repo string, number int) (*gh.PR, error)
}

// defaultFetcher delegates to the real gh package functions.
type defaultFetcher struct{}

func (defaultFetcher) SearchMyPRs() ([]gh.PR, error)                       { return gh.SearchMyPRs() }
func (defaultFetcher) SearchReviewRequests() ([]gh.PR, error)               { return gh.SearchReviewRequests() }
func (defaultFetcher) SearchReviewedPRs() ([]gh.PR, error)                  { return gh.SearchReviewedPRs() }
func (defaultFetcher) GetPRDetail(repo string, number int) (*gh.PR, error) { return gh.GetPRDetail(repo, number) }

// Watcher polls GitHub and sends OS notifications when PR state changes.
type Watcher struct {
	cfg       *config.Config
	db        *cache.DB
	username  string
	interval  time.Duration
	lastState map[string]prSnapshot // key: "repo#number"
	fetch     fetcher
}

// New creates a Watcher using the real GitHub CLI.
func New(cfg *config.Config, db *cache.DB, username string, interval time.Duration) *Watcher {
	return newWithFetcher(cfg, db, username, interval, defaultFetcher{})
}

// newWithFetcher is used internally and by tests to inject a mock fetcher.
func newWithFetcher(cfg *config.Config, db *cache.DB, username string, interval time.Duration, f fetcher) *Watcher {
	return &Watcher{
		cfg:       cfg,
		db:        db,
		username:  username,
		interval:  interval,
		lastState: make(map[string]prSnapshot),
		fetch:     f,
	}
}

// Run starts the polling loop. It blocks until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) error {
	// Seed initial state (no notifications on first poll).
	if err := w.poll(ctx, true); err != nil {
		fmt.Printf("watch: initial poll failed: %v\n", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(w.interval):
			if err := w.poll(ctx, false); err != nil {
				fmt.Printf("watch: poll error: %v\n", err)
			}
		}
	}
}

// poll fetches current PR state, detects changes, and sends notifications.
// When seed is true, state is recorded but no notifications are sent.
func (w *Watcher) poll(ctx context.Context, seed bool) error {
	// Collect all PRs we care about.
	prs := make(map[string]gh.PR) // key -> PR

	myPRs, err := w.fetch.SearchMyPRs()
	if err != nil {
		return fmt.Errorf("search my PRs: %w", err)
	}
	for _, p := range myPRs {
		prs[snapshotKey(p.Repository.NameWithOwner, p.Number)] = p
	}

	requests, err := w.fetch.SearchReviewRequests()
	if err != nil {
		return fmt.Errorf("search review requests: %w", err)
	}
	for _, p := range requests {
		prs[snapshotKey(p.Repository.NameWithOwner, p.Number)] = p
	}

	reviewed, err := w.fetch.SearchReviewedPRs()
	if err != nil {
		return fmt.Errorf("search reviewed PRs: %w", err)
	}
	for _, p := range reviewed {
		prs[snapshotKey(p.Repository.NameWithOwner, p.Number)] = p
	}

	// Check for new review requests (PRs in requests that weren't previously tracked).
	if !seed {
		for _, p := range requests {
			key := snapshotKey(p.Repository.NameWithOwner, p.Number)
			if _, existed := w.lastState[key]; !existed {
				change := Change{
					Kind:    "new_review_request",
					Repo:    p.Repository.NameWithOwner,
					PR:      p.Number,
					Title:   p.Title,
					Message: fmt.Sprintf("Review requested on #%d: %s", p.Number, p.Title),
				}
				notify.Send("prflow: Review Requested", change.Message)
			}
		}
	}

	// Fetch details and build new state, detecting changes along the way.
	newState := make(map[string]prSnapshot)
	for key, p := range prs {
		detail, err := w.fetch.GetPRDetail(p.Repository.NameWithOwner, p.Number)
		if err != nil {
			// Use what we have from the search result.
			detail = &p
		}

		snap := buildSnapshot(detail)
		newState[key] = snap

		if !seed {
			if old, ok := w.lastState[key]; ok {
				changes := DetectChanges(key, p.Title, old, snap)
				for _, c := range changes {
					notify.Send("prflow: "+c.Kind, c.Message)
				}
			}
		}
	}

	w.lastState = newState
	return nil
}

// buildSnapshot creates a prSnapshot from a PR detail.
func buildSnapshot(pr *gh.PR) prSnapshot {
	ciStatus := "UNKNOWN"
	for _, check := range pr.StatusCheckRollup {
		if check.Conclusion == "FAILURE" {
			ciStatus = "FAILURE"
			break
		}
		if check.Status == "IN_PROGRESS" {
			ciStatus = "PENDING"
		}
		if check.Conclusion == "SUCCESS" && ciStatus != "PENDING" {
			ciStatus = "SUCCESS"
		}
	}
	return prSnapshot{
		State:          pr.State,
		CommentCount:   len(pr.Comments.Items),
		ReviewCount:    len(pr.Reviews.Nodes),
		CIStatus:       ciStatus,
		ReviewDecision: pr.ReviewDecision,
	}
}

// snapshotKey returns a unique key for a PR: "repo#number".
func snapshotKey(repo string, number int) string {
	return fmt.Sprintf("%s#%d", repo, number)
}

// DetectChanges compares two snapshots and returns a list of changes.
// key is expected to be "repo#number".
func DetectChanges(key, title string, old, new prSnapshot) []Change {
	var changes []Change

	repo, pr := parseKey(key)

	if new.State == "MERGED" && old.State != "MERGED" {
		changes = append(changes, Change{
			Kind:    "merged",
			Repo:    repo,
			PR:      pr,
			Title:   title,
			Message: fmt.Sprintf("#%d merged: %s", pr, title),
		})
		return changes // no point checking other fields
	}

	if new.State == "CLOSED" && old.State != "CLOSED" {
		changes = append(changes, Change{
			Kind:    "closed",
			Repo:    repo,
			PR:      pr,
			Title:   title,
			Message: fmt.Sprintf("#%d closed: %s", pr, title),
		})
		return changes
	}

	if new.CommentCount > old.CommentCount {
		changes = append(changes, Change{
			Kind:    "new_comment",
			Repo:    repo,
			PR:      pr,
			Title:   title,
			Message: fmt.Sprintf("%d new comment(s) on #%d: %s", new.CommentCount-old.CommentCount, pr, title),
		})
	}

	if new.ReviewCount > old.ReviewCount || (new.ReviewDecision != old.ReviewDecision && new.ReviewDecision != "") {
		changes = append(changes, Change{
			Kind:    "review_submitted",
			Repo:    repo,
			PR:      pr,
			Title:   title,
			Message: fmt.Sprintf("Review update on #%d: %s", pr, title),
		})
	}

	if new.CIStatus != old.CIStatus {
		changes = append(changes, Change{
			Kind:    "ci_changed",
			Repo:    repo,
			PR:      pr,
			Title:   title,
			Message: fmt.Sprintf("CI %s -> %s on #%d: %s", old.CIStatus, new.CIStatus, pr, title),
		})
	}

	return changes
}

// parseKey extracts repo and PR number from "repo#number".
func parseKey(key string) (string, int) {
	idx := strings.LastIndex(key, "#")
	if idx < 0 {
		return key, 0
	}
	n, _ := strconv.Atoi(key[idx+1:])
	return key[:idx], n
}
