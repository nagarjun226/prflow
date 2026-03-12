package gh

import (
	"encoding/json"
	"testing"
)

func TestPRUnmarshal(t *testing.T) {
	// Test basic PR JSON from gh search
	data := `{
		"number": 42,
		"title": "Fix auth bug",
		"state": "OPEN",
		"url": "https://github.com/org/repo/pull/42",
		"headRefName": "fix/auth",
		"baseRefName": "main",
		"author": {"login": "nagaconda"},
		"createdAt": "2026-03-01T10:00:00Z",
		"updatedAt": "2026-03-10T15:30:00Z",
		"reviewDecision": "CHANGES_REQUESTED",
		"isDraft": false
	}`

	var pr PR
	err := json.Unmarshal([]byte(data), &pr)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if pr.Number != 42 {
		t.Errorf("expected number 42, got %d", pr.Number)
	}
	if pr.Title != "Fix auth bug" {
		t.Errorf("expected title 'Fix auth bug', got '%s'", pr.Title)
	}
	if pr.Author.Login != "nagaconda" {
		t.Errorf("expected author 'nagaconda', got '%s'", pr.Author.Login)
	}
	if pr.ReviewDecision != "CHANGES_REQUESTED" {
		t.Errorf("expected CHANGES_REQUESTED, got '%s'", pr.ReviewDecision)
	}
	if pr.IsDraft {
		t.Error("expected isDraft false")
	}
}

func TestCommentsUnmarshalArray(t *testing.T) {
	// gh pr view returns comments as a flat array
	data := `[
		{"author": {"login": "alice"}, "body": "LGTM", "createdAt": "2026-03-10T10:00:00Z"},
		{"author": {"login": "bob"}, "body": "Needs fix", "createdAt": "2026-03-10T11:00:00Z"}
	]`

	var comments Comments
	err := json.Unmarshal([]byte(data), &comments)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(comments.Items) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments.Items))
	}
	if comments.Items[0].Author.Login != "alice" {
		t.Errorf("expected alice, got '%s'", comments.Items[0].Author.Login)
	}
	if comments.Items[1].Body != "Needs fix" {
		t.Errorf("expected 'Needs fix', got '%s'", comments.Items[1].Body)
	}
}

func TestCommentsUnmarshalNodes(t *testing.T) {
	// GraphQL returns {nodes: [...]}
	data := `{
		"nodes": [
			{"author": {"login": "charlie"}, "body": "Great work", "createdAt": "2026-03-10T12:00:00Z"}
		]
	}`

	var comments Comments
	err := json.Unmarshal([]byte(data), &comments)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(comments.Items) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments.Items))
	}
	if comments.Items[0].Author.Login != "charlie" {
		t.Errorf("expected charlie, got '%s'", comments.Items[0].Author.Login)
	}
}

func TestCommentsUnmarshalEmpty(t *testing.T) {
	data := `[]`
	var comments Comments
	err := json.Unmarshal([]byte(data), &comments)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(comments.Items) != 0 {
		t.Errorf("expected 0 comments, got %d", len(comments.Items))
	}
}

func TestCommentsUnmarshalNull(t *testing.T) {
	data := `null`
	var comments Comments
	err := json.Unmarshal([]byte(data), &comments)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
}

func TestReviewsUnmarshalArray(t *testing.T) {
	data := `[
		{"author": {"login": "alice"}, "state": "APPROVED"},
		{"author": {"login": "bob"}, "state": "CHANGES_REQUESTED"}
	]`

	var reviews Reviews
	err := json.Unmarshal([]byte(data), &reviews)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(reviews.Nodes) != 2 {
		t.Fatalf("expected 2 reviews, got %d", len(reviews.Nodes))
	}
	if reviews.Nodes[0].State != "APPROVED" {
		t.Errorf("expected APPROVED, got '%s'", reviews.Nodes[0].State)
	}
	if reviews.Nodes[1].State != "CHANGES_REQUESTED" {
		t.Errorf("expected CHANGES_REQUESTED, got '%s'", reviews.Nodes[1].State)
	}
}

func TestReviewsUnmarshalNodes(t *testing.T) {
	data := `{"nodes": [{"author": {"login": "dave"}, "state": "COMMENTED"}]}`

	var reviews Reviews
	err := json.Unmarshal([]byte(data), &reviews)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(reviews.Nodes) != 1 {
		t.Fatalf("expected 1 review, got %d", len(reviews.Nodes))
	}
}

func TestReviewRequestsUnmarshalArray(t *testing.T) {
	data := `[
		{"requestedReviewer": {"login": "reviewer1"}},
		{"requestedReviewer": {"login": "reviewer2"}}
	]`

	var rr ReviewRequests
	err := json.Unmarshal([]byte(data), &rr)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(rr.Nodes) != 2 {
		t.Fatalf("expected 2, got %d", len(rr.Nodes))
	}
}

func TestReviewRequestsUnmarshalNodes(t *testing.T) {
	data := `{"nodes": [{"requestedReviewer": {"login": "reviewer3"}}]}`

	var rr ReviewRequests
	err := json.Unmarshal([]byte(data), &rr)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(rr.Nodes) != 1 {
		t.Fatalf("expected 1, got %d", len(rr.Nodes))
	}
}

func TestFullPRWithComments(t *testing.T) {
	// Full PR JSON as returned by gh pr view
	data := `{
		"number": 87,
		"title": "Refactor auth module",
		"state": "OPEN",
		"url": "https://github.com/org/repo/pull/87",
		"headRefName": "feature/auth",
		"baseRefName": "main",
		"author": {"login": "nagaconda"},
		"createdAt": "2026-03-01T10:00:00Z",
		"updatedAt": "2026-03-10T15:30:00Z",
		"reviewDecision": "APPROVED",
		"mergeable": "MERGEABLE",
		"isDraft": false,
		"reviews": [
			{"author": {"login": "alice"}, "state": "APPROVED"},
			{"author": {"login": "bob"}, "state": "APPROVED"}
		],
		"reviewRequests": [],
		"statusCheckRollup": [
			{"name": "tests", "status": "COMPLETED", "conclusion": "SUCCESS"},
			{"name": "lint", "status": "COMPLETED", "conclusion": "SUCCESS"}
		],
		"comments": [
			{"author": {"login": "alice"}, "body": "LGTM!", "createdAt": "2026-03-10T12:00:00Z", "url": "https://github.com/org/repo/pull/87#comment-1"}
		]
	}`

	var pr PR
	err := json.Unmarshal([]byte(data), &pr)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if pr.Number != 87 {
		t.Errorf("expected 87, got %d", pr.Number)
	}
	if pr.ReviewDecision != "APPROVED" {
		t.Errorf("expected APPROVED, got '%s'", pr.ReviewDecision)
	}
	if pr.Mergeable != "MERGEABLE" {
		t.Errorf("expected MERGEABLE, got '%s'", pr.Mergeable)
	}
	if len(pr.Reviews.Nodes) != 2 {
		t.Errorf("expected 2 reviews, got %d", len(pr.Reviews.Nodes))
	}
	if len(pr.Comments.Items) != 1 {
		t.Errorf("expected 1 comment, got %d", len(pr.Comments.Items))
	}
	if len(pr.StatusCheckRollup) != 2 {
		t.Errorf("expected 2 checks, got %d", len(pr.StatusCheckRollup))
	}
}

func TestRepoRefParsing(t *testing.T) {
	data := `{"nameWithOwner": "juniper/mist-api"}`

	var ref RepoRef
	err := json.Unmarshal([]byte(data), &ref)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if ref.NameWithOwner != "juniper/mist-api" {
		t.Errorf("expected 'juniper/mist-api', got '%s'", ref.NameWithOwner)
	}
}

func TestStatusCheckParsing(t *testing.T) {
	data := `[
		{"name": "CI", "status": "COMPLETED", "conclusion": "SUCCESS"},
		{"name": "deploy", "status": "IN_PROGRESS", "conclusion": ""},
		{"name": "lint", "status": "COMPLETED", "conclusion": "FAILURE"}
	]`

	var checks []StatusCheck
	err := json.Unmarshal([]byte(data), &checks)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(checks) != 3 {
		t.Fatalf("expected 3 checks, got %d", len(checks))
	}
	if checks[0].Conclusion != "SUCCESS" {
		t.Errorf("expected SUCCESS, got '%s'", checks[0].Conclusion)
	}
	if checks[2].Conclusion != "FAILURE" {
		t.Errorf("expected FAILURE, got '%s'", checks[2].Conclusion)
	}
}

func TestSearchResultsParsing(t *testing.T) {
	// gh search prs returns this format
	data := `[
		{
			"number": 1,
			"title": "PR One",
			"state": "open",
			"url": "https://github.com/org/repo/pull/1",
			"createdAt": "2026-03-01T00:00:00Z",
			"updatedAt": "2026-03-10T00:00:00Z",
			"repository": {"nameWithOwner": "org/repo"}
		},
		{
			"number": 2,
			"title": "PR Two",
			"state": "open",
			"url": "https://github.com/org/repo2/pull/2",
			"createdAt": "2026-03-02T00:00:00Z",
			"updatedAt": "2026-03-11T00:00:00Z",
			"repository": {"nameWithOwner": "org/repo2"}
		}
	]`

	var results []struct {
		Number     int     `json:"number"`
		Title      string  `json:"title"`
		State      string  `json:"state"`
		URL        string  `json:"url"`
		CreatedAt  string  `json:"createdAt"`
		UpdatedAt  string  `json:"updatedAt"`
		Repository RepoRef `json:"repository"`
	}
	err := json.Unmarshal([]byte(data), &results)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Repository.NameWithOwner != "org/repo" {
		t.Errorf("expected 'org/repo', got '%s'", results[0].Repository.NameWithOwner)
	}
}

func TestThreadCommentParsing(t *testing.T) {
	tc := ThreadComment{
		Author:    "alice",
		Body:      "Please fix this",
		CreatedAt: "2026-03-10T10:00:00Z",
		URL:       "https://github.com/org/repo/pull/1#discussion_r123",
	}
	if tc.Author != "alice" {
		t.Errorf("expected alice, got '%s'", tc.Author)
	}
}

func TestReviewThreadStruct(t *testing.T) {
	rt := ReviewThread{
		ID:         "thread-1",
		Path:       "src/auth.go",
		Line:       42,
		IsResolved: false,
		Comments: []ThreadComment{
			{Author: "bob", Body: "Needs refactor", CreatedAt: "2026-03-10T10:00:00Z"},
		},
	}
	if rt.Path != "src/auth.go" {
		t.Errorf("expected 'src/auth.go', got '%s'", rt.Path)
	}
	if rt.IsResolved {
		t.Error("expected unresolved")
	}
	if len(rt.Comments) != 1 {
		t.Errorf("expected 1 comment, got %d", len(rt.Comments))
	}
}
