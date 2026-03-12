package gh

import (
	"encoding/json"
	"testing"
)

// Test the full PR detail JSON structure that gh pr view returns
func TestFullPRViewJSON(t *testing.T) {
	data := `{
		"number": 412,
		"title": "Refactor auth token handling",
		"state": "OPEN",
		"url": "https://github.com/juniper/mist-api/pull/412",
		"headRefName": "feature/auth-refactor",
		"baseRefName": "main",
		"author": {"login": "nagaconda"},
		"createdAt": "2026-03-01T10:00:00Z",
		"updatedAt": "2026-03-10T15:30:00Z",
		"reviewDecision": "CHANGES_REQUESTED",
		"mergeable": "MERGEABLE",
		"isDraft": false,
		"reviews": [
			{"author": {"login": "bob"}, "state": "CHANGES_REQUESTED"},
			{"author": {"login": "alice"}, "state": "APPROVED"}
		],
		"reviewRequests": [
			{"requestedReviewer": {"login": "charlie"}}
		],
		"statusCheckRollup": [
			{"name": "tests", "status": "COMPLETED", "conclusion": "SUCCESS"},
			{"name": "lint", "status": "COMPLETED", "conclusion": "SUCCESS"},
			{"name": "deploy-preview", "status": "IN_PROGRESS", "conclusion": ""}
		],
		"comments": [
			{
				"author": {"login": "bob"},
				"body": "Can we use the existing TokenValidator here?",
				"createdAt": "2026-03-08T14:00:00Z",
				"url": "https://github.com/juniper/mist-api/pull/412#comment-1"
			},
			{
				"author": {"login": "alice"},
				"body": "Missing error handling for expired tokens",
				"createdAt": "2026-03-09T09:00:00Z",
				"url": "https://github.com/juniper/mist-api/pull/412#comment-2"
			}
		]
	}`

	var pr PR
	err := json.Unmarshal([]byte(data), &pr)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Verify all fields
	if pr.Number != 412 {
		t.Errorf("number: got %d, want 412", pr.Number)
	}
	if pr.Title != "Refactor auth token handling" {
		t.Errorf("title: got %q", pr.Title)
	}
	if pr.HeadRefName != "feature/auth-refactor" {
		t.Errorf("headRefName: got %q", pr.HeadRefName)
	}
	if pr.ReviewDecision != "CHANGES_REQUESTED" {
		t.Errorf("reviewDecision: got %q", pr.ReviewDecision)
	}
	if pr.Mergeable != "MERGEABLE" {
		t.Errorf("mergeable: got %q", pr.Mergeable)
	}
	if len(pr.Reviews.Nodes) != 2 {
		t.Errorf("reviews: got %d, want 2", len(pr.Reviews.Nodes))
	}
	if len(pr.ReviewRequests.Nodes) != 1 {
		t.Errorf("reviewRequests: got %d, want 1", len(pr.ReviewRequests.Nodes))
	}
	if pr.ReviewRequests.Nodes[0].RequestedReviewer.Login != "charlie" {
		t.Errorf("reviewer: got %q", pr.ReviewRequests.Nodes[0].RequestedReviewer.Login)
	}
	if len(pr.StatusCheckRollup) != 3 {
		t.Errorf("checks: got %d, want 3", len(pr.StatusCheckRollup))
	}
	if len(pr.Comments.Items) != 2 {
		t.Errorf("comments: got %d, want 2", len(pr.Comments.Items))
	}
}

func TestPRWithEmptyCollections(t *testing.T) {
	data := `{
		"number": 1,
		"title": "Empty PR",
		"state": "OPEN",
		"reviews": [],
		"reviewRequests": [],
		"statusCheckRollup": [],
		"comments": []
	}`

	var pr PR
	err := json.Unmarshal([]byte(data), &pr)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(pr.Reviews.Nodes) != 0 {
		t.Errorf("expected 0 reviews, got %d", len(pr.Reviews.Nodes))
	}
	if len(pr.Comments.Items) != 0 {
		t.Errorf("expected 0 comments, got %d", len(pr.Comments.Items))
	}
}

func TestPRWithMissingFields(t *testing.T) {
	// Minimal JSON (search results have fewer fields)
	data := `{
		"number": 1,
		"title": "Minimal PR",
		"state": "open",
		"url": "https://github.com/org/repo/pull/1"
	}`

	var pr PR
	err := json.Unmarshal([]byte(data), &pr)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if pr.Number != 1 {
		t.Errorf("expected 1, got %d", pr.Number)
	}
	if pr.ReviewDecision != "" {
		t.Errorf("expected empty reviewDecision, got %q", pr.ReviewDecision)
	}
}

func TestDraftPR(t *testing.T) {
	data := `{
		"number": 5,
		"title": "WIP: new feature",
		"state": "OPEN",
		"isDraft": true,
		"reviews": [],
		"comments": []
	}`

	var pr PR
	err := json.Unmarshal([]byte(data), &pr)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !pr.IsDraft {
		t.Error("expected isDraft true")
	}
}

func TestConflictingPR(t *testing.T) {
	data := `{
		"number": 10,
		"title": "Has conflicts",
		"state": "OPEN",
		"mergeable": "CONFLICTING",
		"reviewDecision": "APPROVED",
		"reviews": [{"author": {"login": "alice"}, "state": "APPROVED"}],
		"comments": []
	}`

	var pr PR
	err := json.Unmarshal([]byte(data), &pr)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if pr.Mergeable != "CONFLICTING" {
		t.Errorf("expected CONFLICTING, got %q", pr.Mergeable)
	}
}

func TestGraphQLReviewThreadsParsing(t *testing.T) {
	// Simulate the GraphQL response structure
	data := `{
		"data": {
			"repository": {
				"pullRequest": {
					"reviewThreads": {
						"nodes": [
							{
								"id": "thread-1",
								"path": "src/auth.go",
								"line": 42,
								"isResolved": false,
								"comments": {
									"nodes": [
										{
											"author": {"login": "bob"},
											"body": "Use TokenValidator",
											"createdAt": "2026-03-08T14:00:00Z",
											"url": "https://github.com/org/repo/pull/1#discussion_r1"
										}
									]
								}
							},
							{
								"id": "thread-2",
								"path": "src/middleware.go",
								"line": 15,
								"isResolved": true,
								"comments": {
									"nodes": [
										{
											"author": {"login": "alice"},
											"body": "Unused import",
											"createdAt": "2026-03-08T15:00:00Z",
											"url": "https://github.com/org/repo/pull/1#discussion_r2"
										}
									]
								}
							}
						]
					}
				}
			}
		}
	}`

	var result struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					ReviewThreads struct {
						Nodes []struct {
							ID         string `json:"id"`
							Path       string `json:"path"`
							Line       int    `json:"line"`
							IsResolved bool   `json:"isResolved"`
							Comments   struct {
								Nodes []struct {
									Author    Author `json:"author"`
									Body      string `json:"body"`
									CreatedAt string `json:"createdAt"`
									URL       string `json:"url"`
								} `json:"nodes"`
							} `json:"comments"`
						} `json:"nodes"`
					} `json:"reviewThreads"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}

	err := json.Unmarshal([]byte(data), &result)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	threads := result.Data.Repository.PullRequest.ReviewThreads.Nodes
	if len(threads) != 2 {
		t.Fatalf("expected 2 threads, got %d", len(threads))
	}
	if threads[0].Path != "src/auth.go" {
		t.Errorf("expected 'src/auth.go', got %q", threads[0].Path)
	}
	if threads[0].IsResolved {
		t.Error("expected thread 1 unresolved")
	}
	if !threads[1].IsResolved {
		t.Error("expected thread 2 resolved")
	}
	if len(threads[0].Comments.Nodes) != 1 {
		t.Errorf("expected 1 comment, got %d", len(threads[0].Comments.Nodes))
	}
}

func TestAPISearchResultParsing(t *testing.T) {
	// Simulate gh api search/issues response
	line := `{"number":42,"title":"Fix bug","state":"open","html_url":"https://github.com/org/repo/pull/42","created_at":"2026-03-01T00:00:00Z","updated_at":"2026-03-10T00:00:00Z","repository_url":"https://api.github.com/repos/org/repo"}`

	var item struct {
		Number        int    `json:"number"`
		Title         string `json:"title"`
		State         string `json:"state"`
		HTMLURL       string `json:"html_url"`
		CreatedAt     string `json:"created_at"`
		UpdatedAt     string `json:"updated_at"`
		RepositoryURL string `json:"repository_url"`
	}

	err := json.Unmarshal([]byte(line), &item)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if item.Number != 42 {
		t.Errorf("expected 42, got %d", item.Number)
	}

	// Extract repo name
	repoName := ""
	parts := splitAfter(item.RepositoryURL, "/repos/")
	if len(parts) == 2 {
		repoName = parts[1]
	}
	if repoName != "org/repo" {
		t.Errorf("expected 'org/repo', got %q", repoName)
	}
}

func splitAfter(s, sep string) []string {
	idx := len(sep)
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+idx] == sep {
			return []string{s[:i], s[i+idx:]}
		}
	}
	return []string{s}
}
