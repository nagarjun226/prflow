package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

type dashTickMsg time.Time

func RunDashboard(cfg *config.Config) error {
	db, err := cache.Open()
	if err != nil {
		return fmt.Errorf("failed to open cache: %w", err)
	}
	defer db.Close()

	username, _ := gh.CheckAuth()

	m := dashModel{
		cfg:        cfg,
		db:         db,
		username:   username,
		loading:    true,
		spinFrames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
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
	return tea.Batch(syncPRs(m.db, m.cfg), scanWorkspace(m.cfg), dashTickCmd())
}

func syncPRs(db *cache.DB, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		var doNow, waiting, review, done []cache.CachedPR
		seenPRs := make(map[string]bool)

		// Get current username for filtering
		username, _ := gh.CheckAuth()

		// Step 1: Search for my authored PRs (fast, cross-repo)
		myPRs, _ := gh.SearchMyPRs()

		// Build set of repos from: search results + config
		repoSet := make(map[string]bool)
		for _, pr := range myPRs {
			if pr.Repository.NameWithOwner != "" {
				repoSet[pr.Repository.NameWithOwner] = true
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

				// Only show PRs authored by current user in Do Now / Waiting
				isMyPR := strings.EqualFold(pr.Author.Login, username)

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
		// Clear status message on any keypress
		m.statusMsg = ""

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.viewMode == viewDetail {
				m.viewMode = viewList
				m.detailPR = nil
				return m, nil
			}
			return m, tea.Quit
		case "esc":
			if m.viewMode == viewDetail {
				m.viewMode = viewList
				m.detailPR = nil
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
				maxThread := len(m.detailThreads) - 1
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
		case "R":
			m.loading = true
			m.err = ""
			return m, tea.Batch(syncPRs(m.db, m.cfg), scanWorkspace(m.cfg))
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
	case sectionWorkspace:
		// Workspace items open in browser
		if m.cursor < len(m.workspace) {
			ws := m.workspace[m.cursor]
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

// View renders the dashboard
func (m dashModel) View() string {
	if m.viewMode == viewDetail && m.detailPR != nil {
		return m.viewDetail()
	}
	return m.viewDashboard()
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

	for i, pr := range prs {
		if i >= maxShow {
			s.WriteString(fmt.Sprintf("\n  %s\n",
				repoStyle.Render(fmt.Sprintf("+ %d more...", len(prs)-maxShow))))
			break
		}
		s.WriteString(m.renderPRCard(pr, i == m.cursor, width-6))
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

	title := pr.Title
	maxTitle := width - len(repoShort) - 10
	if maxTitle < 20 {
		maxTitle = 30
	}
	if len(title) > maxTitle {
		title = title[:maxTitle-3] + "..."
	}

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
	for i, ws := range m.workspace {
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
		for i, t := range m.detailThreads {
			if t.IsResolved {
				continue
			}
			selected := i == m.threadCursor

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
	}

	pairs = append(pairs, helpPair("R", "refresh"))
	pairs = append(pairs, helpPair("q", "quit"))

	return helpStyle.Render("  " + strings.Join(pairs, "  "))
}

func (m dashModel) renderDetailHelp() string {
	pairs := []string{
		helpPair("o", "open in browser"),
		helpPair("esc", "back"),
		helpPair("q", "quit"),
	}
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
	for _, fmt := range formats {
		t, err := time.Parse(fmt, isoTime)
		if err == nil {
			return timeSince(t)
		}
	}
	return ""
}

func timeSinceStr(isoTime string) string {
	result := formatTimeAgo(isoTime)
	if result == "" {
		return ""
	}
	return result
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
