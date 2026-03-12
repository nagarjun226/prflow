package gh

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// CheckAuth verifies gh CLI is authenticated
func CheckAuth() (string, error) {
	out, err := run("auth", "status", "--hostname", "github.com")
	if err != nil {
		return "", fmt.Errorf("gh auth failed: %w\n%s", err, out)
	}
	// Extract username from output
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Logged in to") {
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "as" && i+1 < len(parts) {
					return strings.TrimSpace(parts[i+1]), nil
				}
			}
		}
		if strings.Contains(line, "account") {
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "account" && i+1 < len(parts) {
					return strings.TrimSpace(parts[i+1]), nil
				}
			}
		}
	}
	return "unknown", nil
}

// PR represents a pull request
type PR struct {
	Number         int      `json:"number"`
	Title          string   `json:"title"`
	State          string   `json:"state"`
	URL            string   `json:"url"`
	HeadRefName    string   `json:"headRefName"`
	BaseRefName    string   `json:"baseRefName"`
	Author         Author   `json:"author"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
	ReviewDecision string   `json:"reviewDecision"`
	Mergeable      string   `json:"mergeable"`
	IsDraft        bool     `json:"isDraft"`
	Repository     RepoRef  `json:"repository"`
	Reviews        Reviews  `json:"reviews"`
	ReviewRequests ReviewRequests `json:"reviewRequests"`
	StatusCheckRollup []StatusCheck `json:"statusCheckRollup"`
	Comments       Comments `json:"comments"`
}

type Author struct {
	Login string `json:"login"`
}

type RepoRef struct {
	NameWithOwner string `json:"nameWithOwner"`
}

type Reviews struct {
	Nodes []Review `json:"nodes"`
}

type Review struct {
	Author Author `json:"author"`
	State  string `json:"state"`
}

type ReviewRequests struct {
	Nodes []ReviewRequest `json:"nodes"`
}

type ReviewRequest struct {
	RequestedReviewer Author `json:"requestedReviewer"`
}

type StatusCheck struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

type Comments struct {
	Nodes []Comment `json:"nodes"`
}

type Comment struct {
	Author    Author `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
	URL       string `json:"url"`
}

// ReviewThread from GraphQL
type ReviewThread struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Line       int    `json:"line"`
	IsResolved bool   `json:"isResolved"`
	Comments   []ThreadComment `json:"comments"`
}

type ThreadComment struct {
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
	URL       string `json:"url"`
}

// SearchMyPRs returns all open PRs authored by the current user
func SearchMyPRs() ([]PR, error) {
	out, err := run("search", "prs",
		"--author=@me",
		"--state=open",
		"--json", "number,title,state,url,headRefName,baseRefName,author,createdAt,updatedAt,reviewDecision,isDraft,repository",
		"--limit", "100",
	)
	if err != nil {
		return nil, fmt.Errorf("search PRs failed: %w", err)
	}
	var prs []PR
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return nil, fmt.Errorf("parse PRs failed: %w", err)
	}
	return prs, nil
}

// SearchReviewRequests returns PRs where review is requested from current user
func SearchReviewRequests() ([]PR, error) {
	out, err := run("search", "prs",
		"--review-requested=@me",
		"--state=open",
		"--json", "number,title,state,url,headRefName,baseRefName,author,createdAt,updatedAt,repository",
		"--limit", "100",
	)
	if err != nil {
		return nil, fmt.Errorf("search review requests failed: %w", err)
	}
	var prs []PR
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return nil, fmt.Errorf("parse review requests failed: %w", err)
	}
	return prs, nil
}

// GetPRDetail gets full PR details for a specific PR
func GetPRDetail(repo string, number int) (*PR, error) {
	out, err := run("pr", "view",
		fmt.Sprintf("%d", number),
		"-R", repo,
		"--json", "number,title,state,url,headRefName,baseRefName,author,createdAt,updatedAt,reviewDecision,mergeable,isDraft,reviews,reviewRequests,statusCheckRollup,comments",
	)
	if err != nil {
		return nil, fmt.Errorf("get PR detail failed: %w", err)
	}
	var pr PR
	if err := json.Unmarshal([]byte(out), &pr); err != nil {
		return nil, fmt.Errorf("parse PR detail failed: %w", err)
	}
	pr.Repository.NameWithOwner = repo
	return &pr, nil
}

// GetReviewThreads fetches review threads via GraphQL
func GetReviewThreads(repo string, number int) ([]ReviewThread, error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo format: %s", repo)
	}
	query := fmt.Sprintf(`query {
		repository(owner: "%s", name: "%s") {
			pullRequest(number: %d) {
				reviewThreads(first: 50) {
					nodes {
						id
						path
						line
						isResolved
						comments(first: 20) {
							nodes {
								author { login }
								body
								createdAt
								url
							}
						}
					}
				}
			}
		}
	}`, parts[0], parts[1], number)

	out, err := run("api", "graphql", "-f", fmt.Sprintf("query=%s", query))
	if err != nil {
		return nil, fmt.Errorf("graphql query failed: %w", err)
	}

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

	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("parse graphql result failed: %w", err)
	}

	var threads []ReviewThread
	for _, t := range result.Data.Repository.PullRequest.ReviewThreads.Nodes {
		thread := ReviewThread{
			ID:         t.ID,
			Path:       t.Path,
			Line:       t.Line,
			IsResolved: t.IsResolved,
		}
		for _, c := range t.Comments.Nodes {
			thread.Comments = append(thread.Comments, ThreadComment{
				Author:    c.Author.Login,
				Body:      c.Body,
				CreatedAt: c.CreatedAt,
				URL:       c.URL,
			})
		}
		threads = append(threads, thread)
	}
	return threads, nil
}

// ListUserRepos lists repos the user has access to
func ListUserRepos() ([]string, error) {
	out, err := run("repo", "list", "--json", "nameWithOwner", "--limit", "100")
	if err != nil {
		return nil, fmt.Errorf("list repos failed: %w", err)
	}
	var repos []struct {
		NameWithOwner string `json:"nameWithOwner"`
	}
	if err := json.Unmarshal([]byte(out), &repos); err != nil {
		return nil, fmt.Errorf("parse repos failed: %w", err)
	}
	result := make([]string, len(repos))
	for i, r := range repos {
		result[i] = r.NameWithOwner
	}
	return result, nil
}

// OpenInBrowser opens a URL in the default browser
func OpenInBrowser(url string) error {
	return exec.Command("open", url).Start()
}

func run(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
