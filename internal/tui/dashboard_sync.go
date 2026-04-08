package tui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nagarjun226/prflow/internal/cache"
	"github.com/nagarjun226/prflow/internal/config"
	"github.com/nagarjun226/prflow/internal/gh"
)

func syncPRs(db *cache.DB, cfg *config.Config, username string) tea.Cmd {
	return func() tea.Msg {
		var doNow, waiting, review []cache.CachedPR
		seenPRs := make(map[string]bool)

		// Re-verify username if it's the default fallback
		if username == "" || username == "user" {
			if u, err := gh.CheckAuth(); err == nil && u != "user" {
				username = u
			}
		}

		// Step 1: Search for my authored PRs (fast, cross-repo)
		myPRs, _ := gh.SearchMyPRs()

		// Build set of repos + my PR numbers from search results
		repoSet := make(map[string]bool)
		myPRKeys := make(map[string]bool) // "repo#number" keys for PRs I authored
		for _, pr := range myPRs {
			if pr.Repository.NameWithOwner != "" {
				repoSet[pr.Repository.NameWithOwner] = true
				myPRKeys[fmt.Sprintf("%s#%d", pr.Repository.NameWithOwner, pr.Number)] = true
			}
		}
		for _, repo := range cfg.Repos {
			repoSet[repo] = true
		}
		// Fetch review-requested and reviewed PRs up-front so their repos get
		// included in Step 2's parallel ListPRsForRepo refresh.
		reviewReqs, _ := gh.SearchReviewRequests()
		reviewedPRs, _ := gh.SearchReviewedPRs()
		for _, pr := range reviewReqs {
			if pr.Repository.NameWithOwner != "" {
				repoSet[pr.Repository.NameWithOwner] = true
			}
		}
		for _, pr := range reviewedPRs {
			if pr.Repository.NameWithOwner != "" {
				repoSet[pr.Repository.NameWithOwner] = true
			}
		}

		// Step 2: For each repo, get rich PR data (parallelized)
		type repoResult struct {
			repo string
			prs  []gh.PR
		}
		resultsCh := make(chan repoResult, len(repoSet))
		sem := make(chan struct{}, 5) // limit to 5 concurrent gh calls
		var wg sync.WaitGroup
		for repo := range repoSet {
			wg.Add(1)
			go func(r string) {
				defer wg.Done()
				sem <- struct{}{}        // acquire
				defer func() { <-sem }() // release
				repoPRs, err := gh.ListPRsForRepo(r)
				if err != nil {
					resultsCh <- repoResult{repo: r}
					return
				}
				resultsCh <- repoResult{repo: r, prs: repoPRs}
			}(repo)
		}
		go func() {
			wg.Wait()
			close(resultsCh)
		}()

		// allRepoPRs collects the rich per-repo results for use in Steps 3/4
		allRepoPRs := make(map[string]gh.PR) // key -> richest PR data we have

		for res := range resultsCh {
			for i := range res.prs {
				pr := &res.prs[i]
				key := fmt.Sprintf("%s#%d", res.repo, pr.Number)
				allRepoPRs[key] = *pr
				if seenPRs[key] {
					continue
				}
				seenPRs[key] = true

				// Check if this is my PR: either from search results or by username match
				isMyPR := myPRKeys[key] || strings.EqualFold(pr.Author.Login, username)

				cached := cache.CachedPR{
					PR:   *pr,
					Repo: res.repo,
				}

				if isMyPR {
					switch {
					case pr.ReviewDecision == "CHANGES_REQUESTED":
						cached.Section = "do_now"
						doNow = append(doNow, cached)
					case pr.ReviewDecision == "APPROVED":
						cached.Section = "do_now"
						doNow = append(doNow, cached)
					case pr.Mergeable == "CONFLICTING":
						cached.Section = "do_now"
						doNow = append(doNow, cached)
					default:
						cached.Section = "waiting"
						waiting = append(waiting, cached)
					}
				}
				// Non-authored PRs will be caught by review requests below

				db.UpsertPR(pr, res.repo, cached.Section)
			}
		}

		// Fallback: if per-repo didn't work, use search results directly
		if len(doNow) == 0 && len(waiting) == 0 && len(myPRs) > 0 {
			for i := range myPRs {
				pr := &myPRs[i]
				key := fmt.Sprintf("%s#%d", pr.Repository.NameWithOwner, pr.Number)
				if seenPRs[key] {
					continue
				}
				seenPRs[key] = true
				cached := cache.CachedPR{
					PR:      *pr,
					Repo:    pr.Repository.NameWithOwner,
					Section: "waiting",
				}
				waiting = append(waiting, cached)
				db.UpsertPR(pr, cached.Repo, "waiting")
			}
		}

		// Step 3: PRs where review is requested from me (use rich data from Step 2 when available)
		for i := range reviewReqs {
			pr := &reviewReqs[i]
			key := fmt.Sprintf("%s#%d", pr.Repository.NameWithOwner, pr.Number)
			if seenPRs[key] {
				continue
			}
			seenPRs[key] = true
			richPR := *pr
			if rich, ok := allRepoPRs[key]; ok {
				richPR = rich
			}
			cached := cache.CachedPR{
				PR:      richPR,
				Repo:    pr.Repository.NameWithOwner,
				Section: "review",
			}
			review = append(review, cached)
			db.UpsertPR(&richPR, cached.Repo, "review")
		}

		// Step 4: PRs needing re-attention (reviewed by me, updated after my review)
		// Use rich data from Step 2 where available; fall back to GetPRDetail only if needed.
		var needsAttention []cache.CachedPR
		for i := range reviewedPRs {
			pr := &reviewedPRs[i]
			key := fmt.Sprintf("%s#%d", pr.Repository.NameWithOwner, pr.Number)

			// Skip if already in another section (e.g., my own PRs or review requests)
			if seenPRs[key] {
				continue
			}

			// Use rich data from Step 2 if available (avoids extra API call)
			var detail *gh.PR
			if rich, ok := allRepoPRs[key]; ok {
				detail = &rich
			} else {
				var err error
				detail, err = gh.GetPRDetail(pr.Repository.NameWithOwner, pr.Number)
				if err != nil {
					continue
				}
			}

			// Check if there's been activity after my last review
			if needsReReview(detail, username) {
				seenPRs[key] = true
				cached := cache.CachedPR{
					PR:      *detail,
					Repo:    pr.Repository.NameWithOwner,
					Section: "needs_attention",
				}
				needsAttention = append(needsAttention, cached)
				db.UpsertPR(detail, cached.Repo, "needs_attention")
			}
		}

		// Step 5: Sort each section by urgency (not just updated_at)
		staleDays := config.ParseStaleThresholdDays(cfg.Settings.StaleThreshold)
		SortByUrgency(doNow, staleDays)
		SortByUrgency(waiting, staleDays)
		SortByUrgency(review, staleDays)
		SortByUrgency(needsAttention, staleDays)

		return syncDoneMsg{
			doNow:          doNow,
			waiting:        waiting,
			review:         review,
			needsAttention: needsAttention,
		}
	}
}

// needsReReview checks if a PR needs re-attention from the reviewer
// Returns true if the PR was updated after the user's last review
func needsReReview(pr *gh.PR, username string) bool {
	if pr == nil || username == "" {
		return false
	}

	// Skip if this is my own PR
	if strings.EqualFold(pr.Author.Login, username) {
		return false
	}

	// Find my last review timestamp
	var myLastReview time.Time
	for _, rev := range pr.Reviews.Nodes {
		if !strings.EqualFold(rev.Author.Login, username) {
			continue
		}

		reviewTime, err := time.Parse(time.RFC3339, rev.SubmittedAt)
		if err != nil {
			continue
		}

		if reviewTime.After(myLastReview) {
			myLastReview = reviewTime
		}
	}

	// If I never reviewed, no need for re-attention
	if myLastReview.IsZero() {
		return false
	}

	// Check if PR was updated after my last review
	prUpdated, err := time.Parse(time.RFC3339, pr.UpdatedAt)
	if err != nil {
		return false
	}

	// If PR updated after my review (with 1-minute buffer to avoid false positives)
	return prUpdated.After(myLastReview.Add(1 * time.Minute))
}

func scanWorkspace(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		var repos []RepoStatus
		for _, dir := range cfg.Workspace.ScanDirs {
			// Scan for git repos in directory
			entries, err := scanDir(dir)
			if err != nil {
				continue
			}
			for _, path := range entries {
				rs, err := ScanWorkspaceRepo(path)
				if err != nil {
					continue
				}
				repos = append(repos, *rs)
			}
		}
		// Also scan explicitly configured repos
		for _, path := range cfg.Workspace.Repos {
			rs, err := ScanWorkspaceRepo(path)
			if err != nil {
				continue
			}
			repos = append(repos, *rs)
		}
		return workspaceScanMsg{repos: repos}
	}
}

func scanDir(dir string) ([]string, error) {
	entries, err := readDirNames(dir)
	if err != nil {
		return nil, err
	}
	var repos []string
	for _, name := range entries {
		path := dir + "/" + name
		if isGitRepo(path) {
			repos = append(repos, path)
		} else {
			// Scan 2 levels deep (e.g., ~/repos/org/repo)
			subEntries, err := readDirNames(path)
			if err != nil {
				continue
			}
			for _, subName := range subEntries {
				subPath := path + "/" + subName
				if isGitRepo(subPath) {
					repos = append(repos, subPath)
				}
			}
		}
	}
	return repos, nil
}

func readDirNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func isGitRepo(path string) bool {
	_, err := gitCmd(path, "rev-parse", "--is-inside-work-tree")
	return err == nil
}
