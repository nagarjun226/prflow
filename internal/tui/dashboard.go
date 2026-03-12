package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cheenu1092-oss/prflow/internal/ai"
	"github.com/cheenu1092-oss/prflow/internal/cache"
	"github.com/cheenu1092-oss/prflow/internal/config"
	"github.com/cheenu1092-oss/prflow/internal/gh"
)

type section int

const (
	sectionDoNow section = iota
	sectionWaiting
	sectionReview
	sectionWorkspace
	sectionDone
)

func (s section) String() string {
	switch s {
	case sectionDoNow:
		return "⚡ Do Now"
	case sectionWaiting:
		return "⏳ Waiting"
	case sectionReview:
		return "👀 Review"
	case sectionWorkspace:
		return "📂 Workspace"
	case sectionDone:
		return "✅ Done"
	}
	return ""
}

type viewMode int

const (
	viewList viewMode = iota
	viewDetail
	viewSearch
)

type dashModel struct {
	cfg       *config.Config
	db        *cache.DB
	username  string

	// Navigation
	section   section
	cursor    int
	viewMode  viewMode

	// Data
	doNow     []cache.CachedPR
	waiting   []cache.CachedPR
	review    []cache.CachedPR
	done      []cache.CachedPR
	workspace []RepoStatus

	// Detail view
	detailPR      *cache.CachedPR
	detailThreads []gh.ReviewThread
	threadCursor  int

	// Search mode
	searchQuery   string
	searchResults []string
	searchCursor  int
	searching     bool

	// AI analysis
	aiAnalysis    *ai.PRAnalysis
	aiThread      *ai.ThreadAnalysis
	aiLoading     bool
	aiAvailable   bool

	// State
	loading    bool
	lastSync   time.Time
	width      int
	height     int
	err        string
	statusMsg  string // temporary status message (git ops feedback)
	spinner    int
	spinFrames []string
}

type syncDoneMsg struct {
	doNow   []cache.CachedPR
	waiting []cache.CachedPR
	review  []cache.CachedPR
	done    []cache.CachedPR
	err     error
}

type workspaceScanMsg struct {
	repos []RepoStatus
}

type detailLoadedMsg struct {
	pr      *cache.CachedPR
	threads []gh.ReviewThread
	err     error
}

type gitOpDoneMsg struct {
	msg string
	err error
}

type searchResultsMsg struct {
	repos []string
	err   error
}

type cloneDoneMsg struct {
	repo string
	path string
	err  error
}

type checkoutDoneMsg struct {
	repo   string
	branch string
	err    error
}

type aiAnalysisDoneMsg struct {
	analysis *ai.PRAnalysis
	err      error
}

type aiThreadDoneMsg struct {
	analysis *ai.ThreadAnalysis
	err      error
}

type prActionDoneMsg struct {
	action string
	err    error
}

type dashTickMsg time.Time

func RunDashboard(cfg *config.Config) error {
	db, err := cache.Open()
	if err != nil {
		return fmt.Errorf("failed to open cache: %w", err)
	}
	defer db.Close()

	username, _ := gh.CheckAuth()

	m := dashModel{
		cfg:         cfg,
		db:          db,
		username:    username,
		loading:     true,
		aiAvailable: ai.Available(),
		spinFrames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func dashTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return dashTickMsg(t)
	})
}

func (m dashModel) Init() tea.Cmd {
	return tea.Batch(syncPRs(m.db, m.cfg, m.username), scanWorkspace(m.cfg), dashTickCmd())
}

func syncPRs(db *cache.DB, cfg *config.Config, username string) tea.Cmd {
	return func() tea.Msg {
		var doNow, waiting, review, done []cache.CachedPR
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

		// Step 2: For each repo, get rich PR data
		for repo := range repoSet {
			repoPRs, err := gh.ListPRsForRepo(repo)
			if err != nil {
				continue
			}
			for i := range repoPRs {
				pr := &repoPRs[i]
				key := fmt.Sprintf("%s#%d", repo, pr.Number)
				if seenPRs[key] {
					continue
				}
				seenPRs[key] = true

				// Check if this is my PR: either from search results or by username match
				isMyPR := myPRKeys[key] || strings.EqualFold(pr.Author.Login, username)

				cached := cache.CachedPR{
					PR:   *pr,
					Repo: repo,
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

				db.UpsertPR(pr, repo, cached.Section)
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

		// Step 3: Get PRs where review is requested from me
		reviewPRs, _ := gh.SearchReviewRequests()
		for i := range reviewPRs {
			pr := &reviewPRs[i]
			key := fmt.Sprintf("%s#%d", pr.Repository.NameWithOwner, pr.Number)
			if seenPRs[key] {
				continue
			}
			seenPRs[key] = true
			cached := cache.CachedPR{
				PR:      *pr,
				Repo:    pr.Repository.NameWithOwner,
				Section: "review",
			}
			review = append(review, cached)
			db.UpsertPR(pr, cached.Repo, "review")
		}

		// Step 4: Get recently merged PRs for Done section
		mergedPRs, _ := gh.SearchMergedPRs()
		for _, pr := range mergedPRs {
			pr := pr
			cached := cache.CachedPR{
				PR:      pr,
				Repo:    pr.Repository.NameWithOwner,
				Section: "done",
			}
			done = append(done, cached)
		}

		return syncDoneMsg{
			doNow:   doNow,
			waiting: waiting,
			review:  review,
			done:    done,
		}
	}
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

func (m dashModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case dashTickMsg:
		m.spinner = (m.spinner + 1) % len(m.spinFrames)
		return m, dashTickCmd()

	case tea.KeyMsg:
		// Search mode handles its own keys
		if m.viewMode == viewSearch {
			return m.updateSearch(msg)
		}

		// Clear status message on any keypress
		m.statusMsg = ""

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.viewMode == viewDetail {
				m.viewMode = viewList
				m.detailPR = nil
				m.aiAnalysis = nil
				m.aiThread = nil
				return m, nil
			}
			return m, tea.Quit
		case "esc":
			if m.viewMode == viewDetail {
				m.viewMode = viewList
				m.detailPR = nil
				m.aiAnalysis = nil
				m.aiThread = nil
				return m, nil
			}
		case "tab":
			m.section = (m.section + 1) % 5
			m.cursor = 0
			m.viewMode = viewList
		case "shift+tab":
			m.section = (m.section + 4) % 5
			m.cursor = 0
			m.viewMode = viewList
		case "up", "k":
			if m.viewMode == viewDetail {
				if m.threadCursor > 0 {
					m.threadCursor--
				}
			} else if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.viewMode == viewDetail {
				// Count unresolved threads only
				unresolvedCount := 0
				for _, t := range m.detailThreads {
					if !t.IsResolved {
						unresolvedCount++
					}
				}
				maxThread := unresolvedCount - 1
				if maxThread < 0 {
					maxThread = 0
				}
				if m.threadCursor < maxThread {
					m.threadCursor++
				}
			} else {
				max := m.currentListLen() - 1
				if max < 0 {
					max = 0
				}
				if m.cursor < max {
					m.cursor++
				}
			}
		case "enter":
			if m.viewMode == viewList {
				return m.openDetail()
			}
		case "o":
			m.openInBrowser()
		case "c":
			// Checkout PR branch locally
			if m.section != sectionWorkspace {
				return m, m.checkoutPRCmd()
			}
		case "a":
			// Approve PR (only in detail view)
			if m.viewMode == viewDetail && m.detailPR != nil {
				pr := m.detailPR
				return m, func() tea.Msg {
					err := gh.ApprovePR(pr.Repo, pr.Number, "")
					return prActionDoneMsg{action: "approved", err: err}
				}
			}
		case "m":
			// Merge PR (only in detail view)
			if m.viewMode == viewDetail && m.detailPR != nil {
				pr := m.detailPR
				// Default to squash merge (most common)
				return m, func() tea.Msg {
					err := gh.MergePR(pr.Repo, pr.Number, "squash", false)
					return prActionDoneMsg{action: "merged", err: err}
				}
			}
		case "r":
			// Resolve thread (only in detail view with threads)
			if m.viewMode == viewDetail && len(m.detailThreads) > 0 {
				unresolvedIdx := 0
				for _, t := range m.detailThreads {
					if t.IsResolved {
						continue
					}
					if unresolvedIdx == m.threadCursor {
						threadID := t.ID
						return m, func() tea.Msg {
							err := gh.ResolveThread(threadID)
							return prActionDoneMsg{action: "resolved thread", err: err}
						}
					}
					unresolvedIdx++
				}
			}
		case "A":
			// AI analysis
			if m.aiAvailable && m.viewMode == viewDetail && m.detailPR != nil {
				m.aiLoading = true
				m.aiAnalysis = nil
				m.aiThread = nil
				pr := m.detailPR
				repoPath := m.findLocalRepo(pr.Repo)
				return m, func() tea.Msg {
					analysis, err := ai.AnalyzePR(pr.Repo, pr.Number, repoPath)
					return aiAnalysisDoneMsg{analysis: analysis, err: err}
				}
			} else if m.aiAvailable && m.viewMode == viewDetail && len(m.detailThreads) > 0 {
				// Analyze the selected thread
				m.aiLoading = true
				m.aiThread = nil
				pr := m.detailPR
				repoPath := m.findLocalRepo(pr.Repo)
				unresolvedIdx := 0
				for _, t := range m.detailThreads {
					if t.IsResolved {
						continue
					}
					if unresolvedIdx == m.threadCursor {
						thread := t
						return m, func() tea.Msg {
							analysis, err := ai.AnalyzeThread(pr.Repo, pr.Number, thread, repoPath)
							return aiThreadDoneMsg{analysis: analysis, err: err}
						}
					}
					unresolvedIdx++
				}
			}
		case "/":
			// Enter search mode (search org repos to clone)
			m.viewMode = viewSearch
			m.searchQuery = ""
			m.searchResults = nil
			m.searchCursor = 0
			m.searching = false
			return m, nil
		case "C":
			// Clone the repo of the selected PR
			if m.section != sectionWorkspace {
				return m, m.cloneCurrentPRRepo()
			}
		case "R":
			m.loading = true
			m.err = ""
			return m, tea.Batch(syncPRs(m.db, m.cfg, m.username), scanWorkspace(m.cfg))
		case "p":
			if m.section == sectionWorkspace {
				return m, m.gitPullCmd()
			}
		case "P":
			if m.section == sectionWorkspace {
				return m, m.gitPushCmd()
			}
		case "f":
			if m.section == sectionWorkspace {
				m.statusMsg = "Fetching all repos..."
				return m, m.fetchAllCmd()
			}
		}

	case syncDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.doNow = msg.doNow
			m.waiting = msg.waiting
			m.review = msg.review
			m.done = msg.done
			m.lastSync = time.Now()
			m.err = ""
		}

	case workspaceScanMsg:
		m.workspace = msg.repos

	case detailLoadedMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.detailPR = msg.pr
			m.detailThreads = msg.threads
			m.viewMode = viewDetail
			m.threadCursor = 0
		}

	case gitOpDoneMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("✗ %s: %v", msg.msg, msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("✓ %s", msg.msg)
		}
		// Re-scan workspace after git ops
		return m, scanWorkspace(m.cfg)

	case searchResultsMsg:
		m.searching = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Search failed: %v", msg.err)
		} else {
			m.searchResults = msg.repos
			m.searchCursor = 0
		}

	case cloneDoneMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("✗ Clone %s failed: %v", msg.repo, msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("✓ Cloned %s → %s", msg.repo, msg.path)
			m.viewMode = viewList
		}
		return m, scanWorkspace(m.cfg)

	case checkoutDoneMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("✗ Checkout failed: %v", msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("✓ Checked out %s on %s", msg.branch, msg.repo)
		}
		return m, scanWorkspace(m.cfg)

	case aiAnalysisDoneMsg:
		m.aiLoading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("✗ AI analysis failed: %v", msg.err)
		} else {
			m.aiAnalysis = msg.analysis
			m.statusMsg = "✓ AI analysis complete"
		}

	case aiThreadDoneMsg:
		m.aiLoading = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("✗ AI thread analysis failed: %v", msg.err)
		} else {
			m.aiThread = msg.analysis
			m.statusMsg = "✓ Thread analysis complete"
		}

	case prActionDoneMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("✗ %s failed: %v", msg.action, msg.err)
		} else {
			m.statusMsg = fmt.Sprintf("✓ PR %s", msg.action)
			// Reload PR detail to reflect changes
			if m.detailPR != nil {
				return m, loadDetail(m.detailPR)
			}
		}
	}

	return m, nil
}

func (m dashModel) currentListLen() int {
	switch m.section {
	case sectionDoNow:
		return len(m.doNow)
	case sectionWaiting:
		return len(m.waiting)
	case sectionReview:
		return len(m.review)
	case sectionWorkspace:
		return len(m.workspace)
	case sectionDone:
		return len(m.done)
	}
	return 0
}

func (m *dashModel) openDetail() (tea.Model, tea.Cmd) {
	var pr *cache.CachedPR
	switch m.section {
	case sectionDoNow:
		if m.cursor < len(m.doNow) {
			pr = &m.doNow[m.cursor]
		}
	case sectionWaiting:
		if m.cursor < len(m.waiting) {
			pr = &m.waiting[m.cursor]
		}
	case sectionReview:
		if m.cursor < len(m.review) {
			pr = &m.review[m.cursor]
		}
	case sectionDone:
		if m.cursor < len(m.done) {
			pr = &m.done[m.cursor]
		}
	case sectionWorkspace:
		// Workspace items show path info and open in browser
		if m.cursor < len(m.workspace) {
			ws := m.workspace[m.cursor]
			m.statusMsg = fmt.Sprintf("📁 %s", ws.Path)
			if ws.HasRemote {
				gh.OpenInBrowser(fmt.Sprintf("https://github.com/%s", ws.Name))
			}
		}
		return m, nil
	}
	if pr == nil {
		return m, nil
	}

	return m, loadDetail(pr)
}

func loadDetail(pr *cache.CachedPR) tea.Cmd {
	return func() tea.Msg {
		detail, err := gh.GetPRDetail(pr.Repo, pr.Number)
		if err != nil {
			return detailLoadedMsg{err: err}
		}
		cached := &cache.CachedPR{PR: *detail, Repo: pr.Repo, Section: pr.Section}

		threads, _ := gh.GetReviewThreads(pr.Repo, pr.Number)

		return detailLoadedMsg{pr: cached, threads: threads}
	}
}

func (m *dashModel) openInBrowser() {
	// In detail view, open the selected thread URL if available
	if m.viewMode == viewDetail {
		if m.detailPR != nil {
			// Try to open the thread URL at the cursor
			if len(m.detailThreads) > 0 {
				unresolvedIdx := 0
				for _, t := range m.detailThreads {
					if t.IsResolved {
						continue
					}
					if unresolvedIdx == m.threadCursor {
						if len(t.Comments) > 0 {
							gh.OpenInBrowser(t.Comments[0].URL)
							return
						}
					}
					unresolvedIdx++
				}
			}
			// Fallback: open the PR URL
			gh.OpenInBrowser(m.detailPR.URL)
		}
		return
	}

	switch m.section {
	case sectionDoNow:
		if m.cursor < len(m.doNow) {
			gh.OpenInBrowser(m.doNow[m.cursor].URL)
		}
	case sectionWaiting:
		if m.cursor < len(m.waiting) {
			gh.OpenInBrowser(m.waiting[m.cursor].URL)
		}
	case sectionReview:
		if m.cursor < len(m.review) {
			gh.OpenInBrowser(m.review[m.cursor].URL)
		}
	case sectionDone:
		if m.cursor < len(m.done) {
			gh.OpenInBrowser(m.done[m.cursor].URL)
		}
	case sectionWorkspace:
		if m.cursor < len(m.workspace) {
			ws := m.workspace[m.cursor]
			if ws.HasRemote {
				gh.OpenInBrowser(fmt.Sprintf("https://github.com/%s", ws.Name))
			}
		}
	}
}

func (m *dashModel) gitPullCmd() tea.Cmd {
	if m.cursor >= len(m.workspace) {
		return nil
	}
	ws := m.workspace[m.cursor]
	return func() tea.Msg {
		_, err := gitCmd(ws.Path, "pull", "origin", ws.Branch)
		return gitOpDoneMsg{msg: fmt.Sprintf("pull %s/%s", ws.Name, ws.Branch), err: err}
	}
}

func (m *dashModel) gitPushCmd() tea.Cmd {
	if m.cursor >= len(m.workspace) {
		return nil
	}
	ws := m.workspace[m.cursor]
	return func() tea.Msg {
		_, err := gitCmd(ws.Path, "push", "origin", ws.Branch)
		return gitOpDoneMsg{msg: fmt.Sprintf("push %s/%s", ws.Name, ws.Branch), err: err}
	}
}

func (m *dashModel) fetchAllCmd() tea.Cmd {
	workspace := m.workspace
	return func() tea.Msg {
		failed := 0
		for _, ws := range workspace {
			_, err := gitCmd(ws.Path, "fetch", "--all")
			if err != nil {
				failed++
			}
		}
		msg := fmt.Sprintf("fetched %d repos", len(workspace))
		var err error
		if failed > 0 {
			msg = fmt.Sprintf("fetched %d repos (%d failed)", len(workspace), failed)
		}
		return gitOpDoneMsg{msg: msg, err: err}
	}
}

// updateSearch handles key input in search mode
func (m *dashModel) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.viewMode = viewList
		return m, nil
	case "enter":
		if len(m.searchResults) > 0 && m.searchCursor < len(m.searchResults) {
			// Clone selected repo
			repo := m.searchResults[m.searchCursor]
			m.statusMsg = fmt.Sprintf("Cloning %s...", repo)
			return m, m.cloneRepoCmd(repo)
		}
		if len(m.searchQuery) > 0 && !m.searching {
			// Execute search
			m.searching = true
			q := m.searchQuery
			return m, func() tea.Msg {
				repos, err := gh.SearchOrgRepos(q)
				return searchResultsMsg{repos: repos, err: err}
			}
		}
	case "up":
		if m.searchCursor > 0 {
			m.searchCursor--
		}
	case "down":
		if len(m.searchResults) > 0 && m.searchCursor < len(m.searchResults)-1 {
			m.searchCursor++
		}
	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.searchResults = nil
		}
	default:
		ch := msg.String()
		if len(ch) == 1 && ch[0] >= 32 {
			m.searchQuery += ch
			m.searchResults = nil
		}
	}
	return m, nil
}

func (m *dashModel) checkoutPRCmd() tea.Cmd {
	var pr *cache.CachedPR
	switch m.section {
	case sectionDoNow:
		if m.cursor < len(m.doNow) {
			pr = &m.doNow[m.cursor]
		}
	case sectionWaiting:
		if m.cursor < len(m.waiting) {
			pr = &m.waiting[m.cursor]
		}
	case sectionReview:
		if m.cursor < len(m.review) {
			pr = &m.review[m.cursor]
		}
	case sectionDone:
		if m.cursor < len(m.done) {
			pr = &m.done[m.cursor]
		}
	}
	if pr == nil {
		return nil
	}

	repo := pr.Repo
	branch := pr.HeadRefName
	number := pr.Number

	// Capture workspace scan dirs for use inside the closure
	localPath := m.findLocalRepo(repo)
	reposDir := m.cfg.Settings.ReposDir

	m.statusMsg = fmt.Sprintf("Checking out PR #%d...", number)
	return func() tea.Msg {
		// If branch name is unknown (search results), fetch it first
		if branch == "" {
			detail, err := gh.GetPRDetail(repo, number)
			if err != nil || detail.HeadRefName == "" {
				return checkoutDoneMsg{repo: repo, branch: "unknown", err: fmt.Errorf("can't determine branch for PR #%d", number)}
			}
			branch = detail.HeadRefName
		}

		// If repo exists locally, checkout there
		if localPath != "" {
			gitCmd(localPath, "fetch", "--all")
			_, err := gitCmd(localPath, "checkout", branch)
			if err != nil {
				_, err = gitCmd(localPath, "checkout", "-b", branch, "origin/"+branch)
			}
			return checkoutDoneMsg{repo: repo, branch: branch, err: err}
		}

		// Not found locally — clone first, then checkout
		dest := reposDir + "/" + repo
		if !isGitRepo(dest) {
			err := gh.CloneRepo(repo, dest)
			if err != nil {
				return checkoutDoneMsg{repo: repo, branch: branch, err: fmt.Errorf("clone failed: %w", err)}
			}
		}
		gitCmd(dest, "fetch", "--all")
		_, err := gitCmd(dest, "checkout", branch)
		if err != nil {
			_, err = gitCmd(dest, "checkout", "-b", branch, "origin/"+branch)
		}
		return checkoutDoneMsg{repo: repo, branch: branch, err: err}
	}
}

func (m *dashModel) cloneCurrentPRRepo() tea.Cmd {
	var repo string
	switch m.section {
	case sectionDoNow:
		if m.cursor < len(m.doNow) {
			repo = m.doNow[m.cursor].Repo
		}
	case sectionWaiting:
		if m.cursor < len(m.waiting) {
			repo = m.waiting[m.cursor].Repo
		}
	case sectionReview:
		if m.cursor < len(m.review) {
			repo = m.review[m.cursor].Repo
		}
	case sectionDone:
		if m.cursor < len(m.done) {
			repo = m.done[m.cursor].Repo
		}
	}
	if repo == "" {
		return nil
	}
	return m.cloneRepoCmd(repo)
}

func (m *dashModel) cloneRepoCmd(repo string) tea.Cmd {
	// Check if already exists locally
	localPath := m.findLocalRepo(repo)
	if localPath != "" {
		m.statusMsg = fmt.Sprintf("✓ Already cloned at %s", localPath)
		return nil
	}

	reposDir := m.cfg.Settings.ReposDir
	dest := reposDir + "/" + repo
	m.statusMsg = fmt.Sprintf("Cloning %s...", repo)
	return func() tea.Msg {
		// Double-check destination doesn't exist
		if isGitRepo(dest) {
			return cloneDoneMsg{repo: repo, path: dest, err: nil}
		}
		err := gh.CloneRepo(repo, dest)
		return cloneDoneMsg{repo: repo, path: dest, err: err}
	}
}

// findLocalRepo searches workspace scan dirs for a repo matching the given name
func (m *dashModel) findLocalRepo(repo string) string {
	parts := strings.Split(repo, "/")
	repoName := repo
	if len(parts) == 2 {
		repoName = parts[1]
	}

	// Fast path: check already-scanned workspace results first (no subprocess)
	for _, ws := range m.workspace {
		if ws.Name == repo || ws.Name == repoName {
			return ws.Path
		}
	}

	// Check explicit workspace repos mapping
	if path, ok := m.cfg.Workspace.Repos[repo]; ok {
		if isGitRepo(path) {
			return path
		}
	}

	// Check in the configured repos dir
	reposDir := m.cfg.Settings.ReposDir
	if reposDir != "" {
		path := reposDir + "/" + repo
		if isGitRepo(path) {
			return path
		}
		path = reposDir + "/" + repoName
		if isGitRepo(path) {
			return path
		}
	}

	// Slow path: scan dirs (subprocess per check)
	for _, dir := range m.cfg.Workspace.ScanDirs {
		path := dir + "/" + repoName
		if isGitRepo(path) {
			return path
		}
		if len(parts) == 2 {
			path = dir + "/" + parts[0] + "/" + parts[1]
			if isGitRepo(path) {
				return path
			}
		}
	}

	return ""
}

// View renders the dashboard
func (m dashModel) View() string {
	if m.viewMode == viewSearch {
		return m.viewSearchMode()
	}
	if m.viewMode == viewDetail && m.detailPR != nil {
		return m.viewDetail()
	}
	return m.viewDashboard()
}

func (m dashModel) viewSearchMode() string {
	width := m.width
	if width < 60 {
		width = 80
	}

	var s strings.Builder

	header := headerStyle.Width(width).Render(" 🔍 Search Repos")
	s.WriteString(header + "\n\n")

	// Search input
	cursor := "█"
	if m.searching {
		cursor = m.spinFrames[m.spinner%len(m.spinFrames)]
	}
	s.WriteString(fmt.Sprintf("  Search: %s%s\n\n", m.searchQuery, cursor))

	if m.searching {
		s.WriteString(fmt.Sprintf("  %s Searching...\n", m.spinFrames[m.spinner%len(m.spinFrames)]))
	} else if len(m.searchResults) > 0 {
		s.WriteString(fmt.Sprintf("  %d repos found:\n\n", len(m.searchResults)))

		maxShow := m.height - 10
		if maxShow < 5 {
			maxShow = 15
		}

		for i, repo := range m.searchResults {
			if i >= maxShow {
				s.WriteString(fmt.Sprintf("\n  + %d more...\n", len(m.searchResults)-maxShow))
				break
			}
			cursor := "  "
			if i == m.searchCursor {
				cursor = prTitleSelectedStyle.Render("▸ ")
			}

			// Check if repo exists locally
			localPath := m.findLocalRepo(repo)
			localTag := ""
			if localPath != "" {
				localTag = wsCleanStyle.Render(" (local)")
			}

			if i == m.searchCursor {
				s.WriteString(fmt.Sprintf("  %s%s%s\n", cursor, prTitleSelectedStyle.Render(repo), localTag))
			} else {
				s.WriteString(fmt.Sprintf("  %s%s%s\n", cursor, repo, localTag))
			}
		}
	} else if m.searchQuery != "" && !m.searching {
		s.WriteString("  Press [enter] to search\n")
	} else {
		s.WriteString("  Type a query to search repos in your orgs.\n")
		s.WriteString("  Select a result and press [enter] to clone.\n")
	}

	s.WriteString("\n")
	s.WriteString(helpStyle.Render("  " + strings.Join([]string{
		helpPair("enter", "search/clone"),
		helpPair("↑↓", "nav"),
		helpPair("esc", "back"),
	}, "  ")))

	return s.String()
}

func (m dashModel) viewDashboard() string {
	// ─── Header Bar ─────────────────────────────────────────
	syncAgo := ""
	if !m.lastSync.IsZero() {
		syncAgo = fmt.Sprintf("  ⟳ %s", timeSince(m.lastSync))
	}
	totalPRs := len(m.doNow) + len(m.waiting) + len(m.review)

	headerWidth := m.width
	if headerWidth < 60 {
		headerWidth = 80
	}
	header := headerStyle.Width(headerWidth).Render(
		fmt.Sprintf(" ⚡ PRFlow    @%s  ·  %d active PRs%s", m.username, totalPRs, syncAgo))

	// ─── Sidebar ────────────────────────────────────────────
	sidebar := m.renderSidebar()

	// ─── Main Panel ─────────────────────────────────────────
	mainWidth := m.width - 28
	if mainWidth < 40 {
		mainWidth = 60
	}
	main := m.renderMainPanel(mainWidth)

	// ─── Status Bar ─────────────────────────────────────────
	statusBar := m.renderStatusBar()

	// ─── Help Bar ───────────────────────────────────────────
	help := m.renderHelp()

	// ─── Compose Layout ─────────────────────────────────────
	content := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)

	return header + "\n" + content + "\n" + statusBar + help
}

func (m dashModel) renderSidebar() string {
	var s strings.Builder

	type sidebarEntry struct {
		icon  string
		label string
		count int
		sec   section
	}

	entries := []sidebarEntry{
		{"⚡", "Do Now", len(m.doNow), sectionDoNow},
		{"⏳", "Waiting", len(m.waiting), sectionWaiting},
		{"👀", "Review", len(m.review), sectionReview},
		{"📂", "Workspace", len(m.workspace), sectionWorkspace},
		{"✅", "Done", len(m.done), sectionDone},
	}

	for _, e := range entries {
		countStr := sidebarCountStyle.Render(fmt.Sprintf(" %d", e.count))
		if e.sec == m.section {
			s.WriteString(sidebarActiveStyle.Render(
				fmt.Sprintf("▸ %s %s", e.icon, e.label)) + countStr + "\n")
		} else {
			s.WriteString(sidebarSectionStyle.Render(
				fmt.Sprintf("  %s %s", e.icon, e.label)) + countStr + "\n")
		}
	}

	// Favorites
	if len(m.cfg.Favorites) > 0 {
		s.WriteString("\n")
		s.WriteString(favHeaderStyle.Render("★ Favorites") + "\n")
		for _, fav := range m.cfg.Favorites {
			parts := strings.Split(fav, "/")
			name := fav
			if len(parts) == 2 {
				name = parts[1]
			}
			s.WriteString(favItemStyle.Render(name) + "\n")
		}
	}

	return sidebarStyle.Render(s.String())
}

func (m dashModel) renderMainPanel(width int) string {
	var s strings.Builder

	// Section header with rule
	s.WriteString(sectionHeader.Width(width - 4).Render(m.section.String()))
	s.WriteString("\n")

	if m.loading {
		spin := m.spinFrames[m.spinner%len(m.spinFrames)]
		s.WriteString(fmt.Sprintf("\n %s Syncing with GitHub...\n", spin))
		return mainPanelStyle.Width(width).Render(s.String())
	}

	switch m.section {
	case sectionDoNow:
		s.WriteString(m.renderPRCards(m.doNow, width))
	case sectionWaiting:
		s.WriteString(m.renderPRCards(m.waiting, width))
	case sectionReview:
		s.WriteString(m.renderPRCards(m.review, width))
	case sectionWorkspace:
		s.WriteString(m.renderWorkspaceCards(width))
	case sectionDone:
		s.WriteString(m.renderPRCards(m.done, width))
	}

	return mainPanelStyle.Width(width).Render(s.String())
}

func (m dashModel) renderPRCards(prs []cache.CachedPR, width int) string {
	if len(prs) == 0 {
		return emptyStyle.Render("Nothing here — you're all caught up! 🎉")
	}

	var s strings.Builder
	maxShow := m.height - 10
	if maxShow < 5 {
		maxShow = 15
	}

	// Scroll window: keep cursor visible
	start := 0
	if m.cursor >= maxShow {
		start = m.cursor - maxShow + 1
	}

	for i := start; i < len(prs); i++ {
		if i-start >= maxShow {
			s.WriteString(fmt.Sprintf("\n  %s\n",
				repoStyle.Render(fmt.Sprintf("+ %d more...", len(prs)-i))))
			break
		}
		s.WriteString(m.renderPRCard(prs[i], i == m.cursor, width-6))
	}
	return s.String()
}

func (m dashModel) renderPRCard(pr cache.CachedPR, selected bool, width int) string {
	// Repo short name
	parts := strings.Split(pr.Repo, "/")
	repoShort := pr.Repo
	if len(parts) == 2 {
		repoShort = parts[1]
	}

	// Line 1: repo  #number  title
	numStr := prNumberStyle.Render(fmt.Sprintf("#%d", pr.Number))
	repoStr := prRepoStyle.Render(repoShort)

	maxTitle := width - len(repoShort) - 10
	if maxTitle < 20 {
		maxTitle = 30
	}
	title := truncate(pr.Title, maxTitle)

	titleStr := prTitleStyle.Render(title)
	if selected {
		titleStr = prTitleSelectedStyle.Render(title)
	}

	line1 := fmt.Sprintf("%s  %s  %s", repoStr, numStr, titleStr)

	// Line 2: status badge + time
	badge := m.prBadge(pr)
	timeAgo := ""
	if pr.UpdatedAt != "" {
		timeAgo = wsMetaStyle.Render(fmt.Sprintf("  updated %s", formatTimeAgo(pr.UpdatedAt)))
	}
	line2 := badge + timeAgo

	content := line1 + "\n" + line2

	if selected {
		return prCardSelectedStyle.Width(width).Render(content)
	}
	return prCardStyle.Width(width).Render(content)
}

func (m dashModel) prBadge(pr cache.CachedPR) string {
	switch {
	case pr.ReviewDecision == "APPROVED" && pr.Mergeable != "CONFLICTING":
		return badgeMerge.Render("READY TO MERGE")
	case pr.ReviewDecision == "APPROVED" && pr.Mergeable == "CONFLICTING":
		return badgeConflict.Render("CONFLICT")
	case pr.ReviewDecision == "CHANGES_REQUESTED":
		return badgeChanges.Render("CHANGES REQUESTED")
	case pr.Mergeable == "CONFLICTING":
		return badgeConflict.Render("CONFLICT")
	case pr.IsDraft:
		return badgeDraft.Render("DRAFT")
	case pr.ReviewDecision == "REVIEW_REQUIRED":
		return badgeWaiting.Render("AWAITING REVIEW")
	default:
		return badgeWaiting.Render("IN REVIEW")
	}
}

func (m dashModel) renderWorkspaceCards(width int) string {
	if len(m.workspace) == 0 {
		return emptyStyle.Render("No repos found.\nConfigure workspace.scan_dirs in ~/.config/prflow/config.yaml")
	}

	var s strings.Builder
	maxShow := m.height - 10
	if maxShow < 5 {
		maxShow = 15
	}

	// Scroll window: keep cursor visible
	start := 0
	if m.cursor >= maxShow {
		start = m.cursor - maxShow + 1
	}

	for i := start; i < len(m.workspace); i++ {
		if i-start >= maxShow {
			s.WriteString(fmt.Sprintf("\n  %s\n",
				repoStyle.Render(fmt.Sprintf("+ %d more...", len(m.workspace)-i))))
			break
		}
		ws := m.workspace[i]
		s.WriteString(m.renderWorkspaceCard(&ws, i == m.cursor, width-6))
	}
	return s.String()
}

func (m dashModel) renderWorkspaceCard(ws *RepoStatus, selected bool, width int) string {
	nameStr := prNumberStyle.Render(ws.Name)

	// Branch
	branchStr := wsMetaStyle.Render("on ") + detailValueStyle.Render(ws.Branch)

	// Behind/ahead
	var baStr string
	if ws.Behind > 0 && ws.Behind > 20 {
		baStr = wsBehindStyle.Render(fmt.Sprintf("↓%d behind", ws.Behind)) + "  "
	} else if ws.Behind > 0 {
		baStr = wsDirtyStyle.Render(fmt.Sprintf("↓%d behind", ws.Behind)) + "  "
	}
	if ws.Ahead > 0 {
		baStr += wsCleanStyle.Render(fmt.Sprintf("↑%d ahead", ws.Ahead))
	}
	if ws.Behind == 0 && ws.Ahead == 0 {
		baStr = wsCleanStyle.Render("up to date")
	}

	// Working tree
	var treeStr string
	if ws.Clean {
		treeStr = wsCleanStyle.Render("✓ clean")
	} else {
		var parts []string
		if ws.Modified > 0 {
			parts = append(parts, fmt.Sprintf("%d modified", ws.Modified))
		}
		if ws.Staged > 0 {
			parts = append(parts, fmt.Sprintf("%d staged", ws.Staged))
		}
		if ws.Untracked > 0 {
			parts = append(parts, fmt.Sprintf("%d untracked", ws.Untracked))
		}
		treeStr = wsDirtyStyle.Render(strings.Join(parts, " · "))
	}

	// Unpushed
	unpushedStr := ""
	if ws.Unpushed > 0 {
		unpushedStr = "\n" + wsDirtyStyle.Render(fmt.Sprintf("⬆ %d unpushed", ws.Unpushed))
	}

	// Last commit
	commitStr := ""
	if ws.LastCommit != "" {
		commitStr = "\n" + wsMetaStyle.Render(ws.LastCommit)
	}

	content := nameStr + "  " + branchStr + "\n" + baStr + "  " + treeStr + unpushedStr + commitStr

	if selected {
		return wsCardSelectedStyle.Width(width).Render(content)
	}
	return wsCardStyle.Width(width).Render(content)
}

func (m dashModel) viewDetail() string {
	pr := m.detailPR
	var s strings.Builder

	width := m.width
	if width < 60 {
		width = 80
	}

	// Header
	header := headerStyle.Width(width).Render(
		fmt.Sprintf(" %s  #%d", pr.Repo, pr.Number))
	s.WriteString(header + "\n\n")

	// Title
	s.WriteString("  " + lipgloss.NewStyle().Bold(true).Foreground(colorWhite).Render(pr.Title) + "\n\n")

	// Info grid
	s.WriteString(fmt.Sprintf("  %s %s → %s\n",
		detailLabelStyle.Render("Branch"),
		detailValueStyle.Render(pr.HeadRefName),
		wsMetaStyle.Render(pr.BaseRefName)))

	// Status badge
	s.WriteString(fmt.Sprintf("  %s %s\n", detailLabelStyle.Render("Status"), m.prBadge(cache.CachedPR{PR: pr.PR})))

	// Mergeable
	if pr.Mergeable != "" {
		mergeIcon := "  "
		if pr.Mergeable == "MERGEABLE" {
			mergeIcon = wsCleanStyle.Render("✓ MERGEABLE")
		} else if pr.Mergeable == "CONFLICTING" {
			mergeIcon = wsBehindStyle.Render("✗ CONFLICTING")
		} else {
			mergeIcon = wsMetaStyle.Render(pr.Mergeable)
		}
		s.WriteString(fmt.Sprintf("  %s %s\n", detailLabelStyle.Render("Merge"), mergeIcon))
	}

	// CI Checks
	if len(pr.StatusCheckRollup) > 0 {
		s.WriteString(fmt.Sprintf("\n  %s\n", detailLabelStyle.Render("CI Checks")))
		for _, check := range pr.StatusCheckRollup {
			var icon string
			switch check.Conclusion {
			case "SUCCESS":
				icon = wsCleanStyle.Render("✓")
			case "FAILURE":
				icon = wsBehindStyle.Render("✗")
			default:
				icon = wsDirtyStyle.Render("⏳")
			}
			s.WriteString(fmt.Sprintf("    %s %s\n", icon, check.Name))
		}
	}

	// Reviewers
	if len(pr.Reviews.Nodes) > 0 {
		s.WriteString(fmt.Sprintf("\n  %s\n", detailLabelStyle.Render("Reviewers")))
		seen := make(map[string]bool)
		for _, rev := range pr.Reviews.Nodes {
			if seen[rev.Author.Login] {
				continue
			}
			seen[rev.Author.Login] = true
			var icon string
			switch rev.State {
			case "APPROVED":
				icon = wsCleanStyle.Render("✓")
			case "CHANGES_REQUESTED":
				icon = wsBehindStyle.Render("✗")
			default:
				icon = wsDirtyStyle.Render("💬")
			}
			s.WriteString(fmt.Sprintf("    %s %s  %s\n",
				icon,
				threadAuthorStyle.Render("@"+rev.Author.Login),
				wsMetaStyle.Render(rev.State)))
		}
	}

	// Review Threads
	if len(m.detailThreads) > 0 {
		unresolved := 0
		resolved := 0
		for _, t := range m.detailThreads {
			if t.IsResolved {
				resolved++
			} else {
				unresolved++
			}
		}

		s.WriteString("\n" + threadHeaderStyle.Render(
			fmt.Sprintf("  📝 Unresolved Threads (%d)", unresolved)) + "\n\n")

		threadIdx := 0
		for _, t := range m.detailThreads {
			if t.IsResolved {
				continue
			}
			selected := threadIdx == m.threadCursor

			fileStr := threadFileStyle.Render(fmt.Sprintf("%s:%d", t.Path, t.Line))
			var body string
			if len(t.Comments) > 0 {
				last := t.Comments[len(t.Comments)-1]
				body = threadAuthorStyle.Render("@"+last.Author) + "  " +
					threadBodyStyle.Render(truncate(last.Body, 70))
			}

			content := fileStr + "\n" + body
			cardWidth := width - 8
			if cardWidth < 40 {
				cardWidth = 60
			}

			if selected {
				s.WriteString("  " + threadCardSelectedStyle.Width(cardWidth).Render(content) + "\n")
			} else {
				s.WriteString("  " + threadCardStyle.Width(cardWidth).Render(content) + "\n")
			}
			threadIdx++
		}

		if resolved > 0 {
			s.WriteString(fmt.Sprintf("  %s\n",
				wsMetaStyle.Render(fmt.Sprintf("  ✅ %d resolved threads (collapsed)", resolved))))
		}
	}

	// AI Analysis (if available)
	if m.aiLoading {
		s.WriteString(fmt.Sprintf("\n  %s %s Analyzing with Claude Code...\n",
			threadHeaderStyle.Render("🤖 AI Analysis"),
			m.spinFrames[m.spinner%len(m.spinFrames)]))
	} else if m.aiAnalysis != nil {
		a := m.aiAnalysis
		s.WriteString("\n" + threadHeaderStyle.Render("  🤖 AI Analysis") + "\n\n")

		if a.Summary != "" {
			s.WriteString(fmt.Sprintf("  %s %s\n", detailLabelStyle.Render("Summary"), detailValueStyle.Render(a.Summary)))
		}
		if a.ActionNeeded != "" {
			s.WriteString(fmt.Sprintf("  %s %s\n", detailLabelStyle.Render("Next Action"), wsCleanStyle.Render(a.ActionNeeded)))
		}
		if a.ReviewSummary != "" {
			s.WriteString(fmt.Sprintf("  %s %s\n", detailLabelStyle.Render("Reviews"), detailValueStyle.Render(a.ReviewSummary)))
		}
		if a.RiskLevel != "" {
			riskStyle := wsCleanStyle
			if a.RiskLevel == "medium" {
				riskStyle = wsDirtyStyle
			} else if a.RiskLevel == "high" {
				riskStyle = wsBehindStyle
			}
			s.WriteString(fmt.Sprintf("  %s %s\n", detailLabelStyle.Render("Risk"), riskStyle.Render(a.RiskLevel)))
		}
		if a.BlockedBy != "" {
			s.WriteString(fmt.Sprintf("  %s %s\n", detailLabelStyle.Render("Blocked By"), wsBehindStyle.Render(a.BlockedBy)))
		}
		if len(a.SuggestedFixes) > 0 {
			s.WriteString(fmt.Sprintf("  %s\n", detailLabelStyle.Render("Suggestions")))
			for _, fix := range a.SuggestedFixes {
				s.WriteString(fmt.Sprintf("    → %s\n", detailValueStyle.Render(fix)))
			}
		}
	}

	// AI Thread Analysis (if available)
	if m.aiThread != nil {
		t := m.aiThread
		s.WriteString("\n" + threadHeaderStyle.Render("  🤖 Thread Analysis") + "\n\n")
		if t.Intent != "" {
			s.WriteString(fmt.Sprintf("  %s %s\n", detailLabelStyle.Render("Intent"), detailValueStyle.Render(t.Intent)))
		}
		if t.Complexity != "" {
			s.WriteString(fmt.Sprintf("  %s %s\n", detailLabelStyle.Render("Complexity"), detailValueStyle.Render(t.Complexity)))
		}
		if t.Suggestion != "" {
			s.WriteString(fmt.Sprintf("  %s %s\n", detailLabelStyle.Render("Approach"), wsCleanStyle.Render(t.Suggestion)))
		}
		if t.DraftReply != "" {
			s.WriteString(fmt.Sprintf("\n  %s\n", detailLabelStyle.Render("Draft Reply")))
			s.WriteString(fmt.Sprintf("  %s\n", threadBodyStyle.Render(t.DraftReply)))
		}
	}

	// URL
	s.WriteString(fmt.Sprintf("\n  %s %s\n", detailLabelStyle.Render("URL"), urlStyle.Render(pr.URL)))

	// Help
	s.WriteString("\n" + m.renderDetailHelp())

	return s.String()
}

func (m dashModel) renderStatusBar() string {
	var parts []string
	if m.statusMsg != "" {
		parts = append(parts, statusBarStyle.Render("  "+m.statusMsg))
	}
	if m.err != "" {
		parts = append(parts, statusErrorStyle.Render("  ⚠ "+m.err))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "  ") + "\n"
}

func (m dashModel) renderHelp() string {
	var pairs []string

	pairs = append(pairs, helpPair("↑↓", "nav"))
	pairs = append(pairs, helpPair("tab", "section"))
	pairs = append(pairs, helpPair("enter", "expand"))
	pairs = append(pairs, helpPair("o", "open"))

	if m.section == sectionWorkspace {
		pairs = append(pairs, helpPair("p", "pull"))
		pairs = append(pairs, helpPair("P", "push"))
		pairs = append(pairs, helpPair("f", "fetch"))
	} else {
		pairs = append(pairs, helpPair("c", "checkout"))
		pairs = append(pairs, helpPair("C", "clone"))
	}

	pairs = append(pairs, helpPair("/", "search"))
	pairs = append(pairs, helpPair("R", "refresh"))
	pairs = append(pairs, helpPair("q", "quit"))

	return helpStyle.Render("  " + strings.Join(pairs, "  "))
}

func (m dashModel) renderDetailHelp() string {
	pairs := []string{
		helpPair("o", "open in browser"),
		helpPair("c", "checkout"),
		helpPair("a", "approve"),
		helpPair("m", "merge"),
	}
	
	// Only show resolve if there are unresolved threads
	if len(m.detailThreads) > 0 {
		hasUnresolved := false
		for _, t := range m.detailThreads {
			if !t.IsResolved {
				hasUnresolved = true
				break
			}
		}
		if hasUnresolved {
			pairs = append(pairs, helpPair("r", "resolve"))
		}
	}
	
	if m.aiAvailable {
		pairs = append(pairs, helpPair("A", "AI analyze"))
	}
	pairs = append(pairs,
		helpPair("esc", "back"),
		helpPair("q", "quit"),
	)
	return helpStyle.Render("  " + strings.Join(pairs, "  "))
}

func timeSince(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		return "just now"
	}
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%dh ago", hours)
	}
	days := hours / 24
	if days == 1 {
		return "yesterday"
	}
	if days < 30 {
		return fmt.Sprintf("%dd ago", days)
	}
	months := days / 30
	if months == 1 {
		return "1 month ago"
	}
	if months < 12 {
		return fmt.Sprintf("%d months ago", months)
	}
	years := months / 12
	if years == 1 {
		return "1 year ago"
	}
	return fmt.Sprintf("%d years ago", years)
}

func formatTimeAgo(isoTime string) string {
	if isoTime == "" {
		return ""
	}
	// Try multiple time formats
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05 -0700 MST",
		time.RFC3339Nano,
	}
	for _, layout := range formats {
		t, err := time.Parse(layout, isoTime)
		if err == nil {
			return timeSince(t)
		}
	}
	return ""
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if max < 4 {
		if len(s) > max {
			return s[:max]
		}
		return s
	}
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
