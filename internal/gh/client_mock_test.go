package gh

import (
	"fmt"
	"strings"
	"testing"
)

// MockRunner returns predefined responses
type MockRunner struct {
	responses map[string]struct {
		output string
		err    error
	}
}

func NewMockRunner() *MockRunner {
	return &MockRunner{
		responses: make(map[string]struct {
			output string
			err    error
		}),
	}
}

func (m *MockRunner) When(key string, output string, err error) {
	m.responses[key] = struct {
		output string
		err    error
	}{output, err}
}

func (m *MockRunner) Run(args ...string) (string, error) {
	key := strings.Join(args, " ")
	for pattern, resp := range m.responses {
		if strings.Contains(key, pattern) {
			return resp.output, resp.err
		}
	}
	return "", fmt.Errorf("no mock for: %s", key)
}

func TestCheckAuthSuccess(t *testing.T) {
	mock := NewMockRunner()
	mock.When("auth status", `github.com
  ✓ Logged in to github.com account nagaconda (keyring)
  - Active account: true`, nil)

	old := defaultRunner
	SetRunner(mock)
	defer SetRunner(old)

	username, err := CheckAuth()
	if err != nil {
		t.Fatalf("CheckAuth failed: %v", err)
	}
	if username != "nagaconda" {
		t.Errorf("expected nagaconda, got %q", username)
	}
}

func TestCheckAuthAsFormat(t *testing.T) {
	mock := NewMockRunner()
	mock.When("auth status", `Logged in to github.com as testuser (token)`, nil)
	mock.When("api user", `testuser`, nil)

	old := defaultRunner
	SetRunner(mock)
	defer SetRunner(old)

	username, err := CheckAuth()
	if err != nil {
		t.Fatalf("CheckAuth failed: %v", err)
	}
	if username != "testuser" {
		t.Errorf("expected testuser, got %q", username)
	}
}

func TestCheckAuthNotLoggedIn(t *testing.T) {
	mock := NewMockRunner()
	mock.When("auth", "not logged in", fmt.Errorf("exit 1"))

	old := defaultRunner
	SetRunner(mock)
	defer SetRunner(old)

	_, err := CheckAuth()
	if err == nil {
		t.Error("expected error for unauthenticated user")
	}
}

func TestSearchMyPRsSuccess(t *testing.T) {
	mock := NewMockRunner()
	mock.When("search prs", `[
		{"number": 1, "title": "PR One", "state": "open", "url": "https://github.com/org/repo/pull/1", "createdAt": "2026-03-01T00:00:00Z", "updatedAt": "2026-03-10T00:00:00Z", "repository": {"nameWithOwner": "org/repo"}},
		{"number": 2, "title": "PR Two", "state": "open", "url": "https://github.com/org/repo2/pull/2", "createdAt": "2026-03-02T00:00:00Z", "updatedAt": "2026-03-11T00:00:00Z", "repository": {"nameWithOwner": "org/repo2"}}
	]`, nil)

	old := defaultRunner
	SetRunner(mock)
	defer SetRunner(old)

	prs, err := SearchMyPRs()
	if err != nil {
		t.Fatalf("SearchMyPRs failed: %v", err)
	}
	if len(prs) != 2 {
		t.Fatalf("expected 2 PRs, got %d", len(prs))
	}
	if prs[0].Number != 1 {
		t.Errorf("expected PR #1, got #%d", prs[0].Number)
	}
	if prs[1].Repository.NameWithOwner != "org/repo2" {
		t.Errorf("expected org/repo2, got %q", prs[1].Repository.NameWithOwner)
	}
}

func TestSearchMyPRsEmpty(t *testing.T) {
	mock := NewMockRunner()
	mock.When("search prs", `[]`, nil)

	old := defaultRunner
	SetRunner(mock)
	defer SetRunner(old)

	prs, err := SearchMyPRs()
	if err != nil {
		t.Fatalf("SearchMyPRs failed: %v", err)
	}
	if len(prs) != 0 {
		t.Errorf("expected 0 PRs, got %d", len(prs))
	}
}

func TestSearchReviewRequestsSuccess(t *testing.T) {
	mock := NewMockRunner()
	mock.When("search prs", `[
		{"number": 5, "title": "Need review", "state": "open", "url": "https://github.com/org/repo/pull/5", "createdAt": "2026-03-01T00:00:00Z", "updatedAt": "2026-03-10T00:00:00Z", "repository": {"nameWithOwner": "org/repo"}}
	]`, nil)

	old := defaultRunner
	SetRunner(mock)
	defer SetRunner(old)

	prs, err := SearchReviewRequests()
	if err != nil {
		t.Fatalf("SearchReviewRequests failed: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
}

func TestListPRsForRepoSuccess(t *testing.T) {
	mock := NewMockRunner()
	mock.When("pr list", `[
		{"number": 42, "title": "Fix bug", "state": "OPEN", "url": "https://github.com/org/repo/pull/42", "headRefName": "fix/bug", "baseRefName": "main", "author": {"login": "user"}, "reviewDecision": "APPROVED", "isDraft": false}
	]`, nil)

	old := defaultRunner
	SetRunner(mock)
	defer SetRunner(old)

	prs, err := ListPRsForRepo("org/repo")
	if err != nil {
		t.Fatalf("ListPRsForRepo failed: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	if prs[0].Repository.NameWithOwner != "org/repo" {
		t.Errorf("expected repo set to org/repo, got %q", prs[0].Repository.NameWithOwner)
	}
}

func TestGetPRDetailSuccess(t *testing.T) {
	mock := NewMockRunner()
	mock.When("pr view", `{
		"number": 42,
		"title": "Fix bug",
		"state": "OPEN",
		"url": "https://github.com/org/repo/pull/42",
		"headRefName": "fix/bug",
		"baseRefName": "main",
		"author": {"login": "user"},
		"reviewDecision": "APPROVED",
		"mergeable": "MERGEABLE",
		"isDraft": false,
		"reviews": [{"author": {"login": "alice"}, "state": "APPROVED"}],
		"reviewRequests": [],
		"statusCheckRollup": [{"name": "CI", "status": "COMPLETED", "conclusion": "SUCCESS"}],
		"comments": [{"author": {"login": "alice"}, "body": "LGTM", "createdAt": "2026-03-10T00:00:00Z"}]
	}`, nil)

	old := defaultRunner
	SetRunner(mock)
	defer SetRunner(old)

	pr, err := GetPRDetail("org/repo", 42)
	if err != nil {
		t.Fatalf("GetPRDetail failed: %v", err)
	}
	if pr.Number != 42 {
		t.Errorf("expected 42, got %d", pr.Number)
	}
	if pr.Repository.NameWithOwner != "org/repo" {
		t.Errorf("expected org/repo, got %q", pr.Repository.NameWithOwner)
	}
	if len(pr.Reviews.Nodes) != 1 {
		t.Errorf("expected 1 review, got %d", len(pr.Reviews.Nodes))
	}
}

func TestListUserReposSuccess(t *testing.T) {
	mock := NewMockRunner()
	mock.When("repo list", `[
		{"nameWithOwner": "org/repo1"},
		{"nameWithOwner": "org/repo2"},
		{"nameWithOwner": "user/dotfiles"}
	]`, nil)

	old := defaultRunner
	SetRunner(mock)
	defer SetRunner(old)

	repos, err := ListUserRepos()
	if err != nil {
		t.Fatalf("ListUserRepos failed: %v", err)
	}
	if len(repos) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(repos))
	}
	if repos[0] != "org/repo1" {
		t.Errorf("expected org/repo1, got %q", repos[0])
	}
}

func TestGetReviewThreadsInvalidRepo(t *testing.T) {
	_, err := GetReviewThreads("invalidrepo", 1)
	if err == nil {
		t.Error("expected error for invalid repo format")
	}
}

func TestSearchOrgReposSuccess(t *testing.T) {
	mock := NewMockRunner()
	mock.When("search repos", `[
		{"nameWithOwner": "org/api"},
		{"nameWithOwner": "org/web"}
	]`, nil)

	old := defaultRunner
	SetRunner(mock)
	defer SetRunner(old)

	repos, err := SearchOrgRepos("org")
	if err != nil {
		t.Fatalf("SearchOrgRepos failed: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	if repos[0] != "org/api" {
		t.Errorf("expected org/api, got %q", repos[0])
	}
}

// CapturingRunner records the arguments passed to Run for verification.
type CapturingRunner struct {
	Args []string
}

func (c *CapturingRunner) Run(args ...string) (string, error) {
	c.Args = args
	return "", nil
}

func TestNudgeReviewerArgs(t *testing.T) {
	capture := &CapturingRunner{}
	old := defaultRunner
	SetRunner(capture)
	defer SetRunner(old)

	err := NudgeReviewer("org/repo", 42, "alice", 3)
	if err != nil {
		t.Fatalf("NudgeReviewer failed: %v", err)
	}

	// Verify correct args
	expected := []string{
		"pr", "comment", "42", "-R", "org/repo", "-b",
		"@alice friendly nudge — this PR has been waiting for your review for 3 days",
	}
	if len(capture.Args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(capture.Args), capture.Args)
	}
	for i, arg := range expected {
		if capture.Args[i] != arg {
			t.Errorf("arg[%d]: expected %q, got %q", i, arg, capture.Args[i])
		}
	}
}

func TestSearchReviewedPRsSuccess(t *testing.T) {
	mock := NewMockRunner()
	mock.When("search prs", `[
		{"number": 100, "title": "Reviewed PR", "state": "open", "url": "https://github.com/org/repo/pull/100", "repository": {"nameWithOwner": "org/repo"}, "createdAt": "2024-01-01T00:00:00Z", "updatedAt": "2024-01-02T00:00:00Z"}
	]`, nil)

	old := defaultRunner
	SetRunner(mock)
	defer SetRunner(old)

	prs, err := SearchReviewedPRs()
	if err != nil {
		t.Fatalf("SearchReviewedPRs failed: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	if prs[0].Number != 100 {
		t.Errorf("expected #100, got #%d", prs[0].Number)
	}
}
