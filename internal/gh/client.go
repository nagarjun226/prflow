package gh

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// CheckAuth verifies gh CLI is authenticated and returns the username
func CheckAuth() (string, error) {
	// First try: gh auth status (works on most versions)
	out, err := run("auth", "status")
	if err != nil {
		return "", fmt.Errorf("not authenticated: %s", out)
	}

	// Try to extract username from various output formats
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		// Format: "Logged in to github.com account username ..."
		if strings.Contains(line, "account") {
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "account" && i+1 < len(parts) {
					username := strings.Trim(parts[i+1], "().,")
					if username != "" {
						return username, nil
					}
				}
			}
		}
		// Format: "Logged in to github.com as username ..."
		if strings.Contains(line, " as ") {
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "as" && i+1 < len(parts) {
					username := strings.Trim(parts[i+1], "().,")
					if username != "" {
						return username, nil
					}
				}
			}
		}
	}

	// Fallback: use gh api to get current user
	apiOut, apiErr := run("api", "user", "--jq", ".login")
	if apiErr == nil && apiOut != "" {
		return strings.TrimSpace(apiOut), nil
	}

	return "user", nil // authenticated but couldn't parse username
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
	// gh search prs has limited --json fields. Use only valid ones.
	out, err := run("search", "prs",
		"--author=@me",
		"--state=open",
		"--limit", "100",
		"--json", "number,title,state,url,repository,createdAt,updatedAt",
	)
	if err != nil {
		// Fallback: try using gh api directly
		return searchPRsViaAPI("author:@me")
	}
	var results []struct {
		Number     int    `json:"number"`
		Title      string `json:"title"`
		State      string `json:"state"`
		URL        string `json:"url"`
		CreatedAt  string `json:"createdAt"`
		UpdatedAt  string `json:"updatedAt"`
		Repository RepoRef `json:"repository"`
	}
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		return nil, fmt.Errorf("parse PRs failed: %w", err)
	}

	var prs []PR
	for _, r := range results {
		prs = append(prs, PR{
			Number:    r.Number,
			Title:     r.Title,
			State:     r.State,
			URL:       r.URL,
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
			Repository: r.Repository,
		})
	}
	return prs, nil
}

// SearchReviewRequests returns PRs where review is requested from current user
func SearchReviewRequests() ([]PR, error) {
	out, err := run("search", "prs",
		"--review-requested=@me",
		"--state=open",
		"--limit", "100",
		"--json", "number,title,state,url,repository,createdAt,updatedAt",
	)
	if err != nil {
		return searchPRsViaAPI("review-requested:@me")
	}
	var results []struct {
		Number     int    `json:"number"`
		Title      string `json:"title"`
		State      string `json:"state"`
		URL        string `json:"url"`
		CreatedAt  string `json:"createdAt"`
		UpdatedAt  string `json:"updatedAt"`
		Repository RepoRef `json:"repository"`
	}
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		return nil, fmt.Errorf("parse review requests failed: %w", err)
	}

	var prs []PR
	for _, r := range results {
		prs = append(prs, PR{
			Number:    r.Number,
			Title:     r.Title,
			State:     r.State,
			URL:       r.URL,
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
			Repository: r.Repository,
		})
	}
	return prs, nil
}

// searchPRsViaAPI is a fallback that uses GitHub search API directly
func searchPRsViaAPI(qualifier string) ([]PR, error) {
	out, err := run("api", "search/issues",
		"-X", "GET",
		"-f", fmt.Sprintf("q=is:pr is:open %s", qualifier),
		"-f", "per_page=100",
		"--jq", ".items[] | {number, title, state, html_url, created_at, updated_at, repository_url}",
	)
	if err != nil {
		return nil, fmt.Errorf("API search failed: %w", err)
	}
	if out == "" {
		return []PR{}, nil
	}

	// Parse line-by-line JSON objects
	var prs []PR
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var item struct {
			Number        int    `json:"number"`
			Title         string `json:"title"`
			State         string `json:"state"`
			HTMLURL       string `json:"html_url"`
			CreatedAt     string `json:"created_at"`
			UpdatedAt     string `json:"updated_at"`
			RepositoryURL string `json:"repository_url"`
		}
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue
		}
		// Extract repo name from URL: https://api.github.com/repos/org/repo
		repoName := ""
		parts := strings.Split(item.RepositoryURL, "/repos/")
		if len(parts) == 2 {
			repoName = parts[1]
		}
		prs = append(prs, PR{
			Number:    item.Number,
			Title:     item.Title,
			State:     item.State,
			URL:       item.HTMLURL,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
			Repository: RepoRef{NameWithOwner: repoName},
		})
	}
	return prs, nil
}

// ListPRsForRepo lists open PRs for a specific repo (more reliable than search)
func ListPRsForRepo(repo string) ([]PR, error) {
	out, err := run("pr", "list",
		"-R", repo,
		"--state", "open",
		"--json", "number,title,state,url,headRefName,baseRefName,author,createdAt,updatedAt,reviewDecision,isDraft",
		"--limit", "50",
	)
	if err != nil {
		return nil, fmt.Errorf("list PRs for %s failed: %w", repo, err)
	}
	var prs []PR
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return nil, fmt.Errorf("parse PRs failed: %w", err)
	}
	for i := range prs {
		prs[i].Repository.NameWithOwner = repo
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

// OpenInBrowser opens a URL in the default browser (cross-platform)
func OpenInBrowser(url string) error {
	// Use gh browse which is cross-platform
	// Fallback to OS-specific commands
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("open", url)
	}
	return cmd.Start()
}

func run(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
