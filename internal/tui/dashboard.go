package tui

import (
	"fmt"
	osexec "os/exec"
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
	loading  bool
	lastSync time.Time
	width    int
	height   int
	err      string
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

func RunDashboard(cfg *config.Config) error {
	db, err := cache.Open()
	if err != nil {
		return fmt.Errorf("failed to open cache: %w", err)
	}
	defer db.Close()

	username, _ := gh.CheckAuth()

	m := dashModel{
		cfg:      cfg,
		db:       db,
		username: username,
		loading:  true,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func (m dashModel) Init() tea.Cmd {
	return tea.Batch(syncPRs(m.db), scanWorkspace(m.cfg))
}

func syncPRs(db *cache.DB) tea.Cmd {
	return func() tea.Msg {
		// Fetch my PRs
		myPRs, err := gh.SearchMyPRs()
		if err != nil {
			return syncDoneMsg{err: err}
		}

		// Fetch review requests
		reviewPRs, err := gh.SearchReviewRequests()
		if err != nil {
			return syncDoneMsg{err: err}
		}

		var doNow, waiting, review, done []cache.CachedPR

		for i := range myPRs {
			pr := &myPRs[i]
			cached := cache.CachedPR{
				PR:   *pr,
				Repo: pr.Repository.NameWithOwner,
			}

			// Classify
			switch {
			case pr.ReviewDecision == "CHANGES_REQUESTED":
				cached.Section = "do_now"
				doNow = append(doNow, cached)
			case pr.ReviewDecision == "APPROVED":
				cached.Section = "do_now" // ready to merge
				doNow = append(doNow, cached)
			case pr.Mergeable == "CONFLICTING":
				cached.Section = "do_now"
				doNow = append(doNow, cached)
			default:
				cached.Section = "waiting"
				waiting = append(waiting, cached)
			}

			db.UpsertPR(pr, cached.Repo, cached.Section)
		}

		for i := range reviewPRs {
			pr := &reviewPRs[i]
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
	cmd := fmt.Sprintf("ls -1 %s 2>/dev/null", dir)
	out, err := runShell(cmd)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func isGitRepo(path string) bool {
	_, err := gitCmd(path, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

func runShell(cmd string) (string, error) {
	c := osexec.Command("sh", "-c", cmd)
	out, err := c.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (m dashModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
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
				m.threadCursor++
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
			return m, tea.Batch(syncPRs(m.db), scanWorkspace(m.cfg))
		case "p":
			if m.section == sectionWorkspace {
				m.gitPull()
			}
		case "P":
			if m.section == sectionWorkspace {
				m.gitPush()
			}
		case "f":
			if m.section == sectionWorkspace {
				return m, m.fetchAll()
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

func (m *dashModel) gitPull() {
	if m.cursor < len(m.workspace) {
		ws := m.workspace[m.cursor]
		gitCmd(ws.Path, "pull", "origin", ws.Branch)
	}
}

func (m *dashModel) gitPush() {
	if m.cursor < len(m.workspace) {
		ws := m.workspace[m.cursor]
		gitCmd(ws.Path, "push", "origin", ws.Branch)
	}
}

func (m *dashModel) fetchAll() tea.Cmd {
	return func() tea.Msg {
		for _, ws := range m.workspace {
			gitCmd(ws.Path, "fetch", "--all")
		}
		return workspaceScanMsg{repos: m.workspace}
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
	// Header
	syncAgo := ""
	if !m.lastSync.IsZero() {
		syncAgo = fmt.Sprintf("⟳ %s", timeSince(m.lastSync))
	}

	totalPRs := len(m.doNow) + len(m.waiting) + len(m.review)
	header := headerStyle.Render(fmt.Sprintf("  PRFlow  @%s · %d active PRs  %s", m.username, totalPRs, syncAgo))

	// Sidebar
	sidebar := m.renderSidebar()

	// Main content
	main := m.renderMainPanel()

	// Help bar
	help := m.renderHelp()

	// Error
	errLine := ""
	if m.err != "" {
		errLine = "\n" + lipgloss.NewStyle().Foreground(colorDanger).Render("  Error: "+m.err)
	}

	// Layout: sidebar | main
	content := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)

	return header + "\n" + content + errLine + "\n" + help
}

func (m dashModel) renderSidebar() string {
	var s strings.Builder

	sections := []section{sectionDoNow, sectionWaiting, sectionReview, sectionWorkspace, sectionDone}
	counts := []int{len(m.doNow), len(m.waiting), len(m.review), len(m.workspace), len(m.done)}

	s.WriteString("\n")
	for i, sec := range sections {
		label := fmt.Sprintf("%s (%d)", sec, counts[i])
		if sec == m.section {
			s.WriteString(sidebarItemSelectedStyle.Render("▸ "+label) + "\n")
		} else {
			s.WriteString(sidebarItemStyle.Render("  "+label) + "\n")
		}
	}

	// Favorites
	if len(m.cfg.Favorites) > 0 {
		s.WriteString("\n")
		s.WriteString(favStarStyle.Render("  ★ Favorites") + "\n")
		for _, fav := range m.cfg.Favorites {
			parts := strings.Split(fav, "/")
			name := fav
			if len(parts) == 2 {
				name = parts[1]
			}
			s.WriteString(sidebarItemStyle.Render("    "+name) + "\n")
		}
	}

	return sidebarStyle.Render(s.String())
}

func (m dashModel) renderMainPanel() string {
	var s strings.Builder

	s.WriteString("\n")
	s.WriteString(sectionHeader.Render(m.section.String()) + "\n")

	if m.loading {
		s.WriteString("  Loading...\n")
		return mainPanelStyle.Render(s.String())
	}

	switch m.section {
	case sectionDoNow:
		s.WriteString(m.renderPRList(m.doNow))
	case sectionWaiting:
		s.WriteString(m.renderPRList(m.waiting))
	case sectionReview:
		s.WriteString(m.renderPRList(m.review))
	case sectionWorkspace:
		s.WriteString(m.renderWorkspace())
	case sectionDone:
		if len(m.done) == 0 {
			s.WriteString("  No recently merged PRs\n")
		} else {
			s.WriteString(m.renderPRList(m.done))
		}
	}

	return mainPanelStyle.Render(s.String())
}

func (m dashModel) renderPRList(prs []cache.CachedPR) string {
	if len(prs) == 0 {
		return "  Nothing here! 🎉\n"
	}

	var s strings.Builder
	maxShow := m.height - 8
	if maxShow < 5 {
		maxShow = 20
	}

	for i, pr := range prs {
		if i >= maxShow {
			s.WriteString(fmt.Sprintf("  ... and %d more\n", len(prs)-maxShow))
			break
		}

		selected := i == m.cursor
		s.WriteString(m.renderPRItem(pr, selected))
		s.WriteString("\n")
	}
	return s.String()
}

func (m dashModel) renderPRItem(pr cache.CachedPR, selected bool) string {
	cursor := "  "
	if selected {
		cursor = "▸ "
	}

	// Repo short name
	parts := strings.Split(pr.Repo, "/")
	repoShort := pr.Repo
	if len(parts) == 2 {
		repoShort = parts[1]
	}

	numStr := prNumberStyle.Render(fmt.Sprintf("#%d", pr.Number))
	repoStr := repoStyle.Render(repoShort)

	// Status indicator
	status := m.prStatusStr(pr)

	// Title (truncated)
	title := pr.Title
	maxTitle := 50
	if len(title) > maxTitle {
		title = title[:maxTitle-3] + "..."
	}

	if selected {
		return prItemSelectedStyle.Render(fmt.Sprintf("%s%s %s  %s\n    %s",
			cursor, repoStr, numStr, title, status))
	}
	return prItemStyle.Render(fmt.Sprintf("%s%s %s  %s\n    %s",
		cursor, repoStr, numStr, title, status))
}

func (m dashModel) prStatusStr(pr cache.CachedPR) string {
	switch {
	case pr.ReviewDecision == "APPROVED" && pr.Mergeable != "CONFLICTING":
		return statusApproved.Render("✓ approved — ready to merge")
	case pr.ReviewDecision == "CHANGES_REQUESTED":
		return statusChanges.Render("changes requested")
	case pr.Mergeable == "CONFLICTING":
		return statusChanges.Render("⚠️ merge conflict")
	case pr.ReviewDecision == "REVIEW_REQUIRED":
		return statusPending.Render("waiting for review")
	case pr.IsDraft:
		return repoStyle.Render("draft")
	default:
		return statusPending.Render("in review")
	}
}

func (m dashModel) renderWorkspace() string {
	if len(m.workspace) == 0 {
		return "  No repos found. Configure workspace.scan_dirs in config.\n"
	}

	var s strings.Builder
	for i, ws := range m.workspace {
		selected := i == m.cursor
		s.WriteString(RenderRepoStatus(&ws, selected))
		s.WriteString("\n")
	}
	return s.String()
}

func (m dashModel) viewDetail() string {
	pr := m.detailPR
	var s strings.Builder

	// Header
	header := headerStyle.Render(fmt.Sprintf("  %s #%d", pr.Repo, pr.Number))
	s.WriteString(header + "\n\n")

	// Title
	s.WriteString(fmt.Sprintf("  %s\n\n", lipgloss.NewStyle().Bold(true).Render(pr.Title)))

	// Branch
	s.WriteString(fmt.Sprintf("  %s %s → %s\n",
		detailLabelStyle.Render("Branch:"),
		detailValueStyle.Render(pr.HeadRefName),
		detailValueStyle.Render(pr.BaseRefName)))

	// Status
	statusStr := ""
	if pr.IsDraft {
		statusStr = "Draft"
	} else if pr.ReviewDecision == "APPROVED" {
		statusStr = statusApproved.Render("Approved ✓")
	} else if pr.ReviewDecision == "CHANGES_REQUESTED" {
		statusStr = statusChanges.Render("Changes Requested")
	} else {
		statusStr = statusPending.Render("In Review")
	}
	s.WriteString(fmt.Sprintf("  %s %s\n", detailLabelStyle.Render("Status:"), statusStr))

	// Mergeable
	mergeStr := pr.Mergeable
	if pr.Mergeable == "CONFLICTING" {
		mergeStr = statusChanges.Render("CONFLICTING ⚠️")
	} else if pr.Mergeable == "MERGEABLE" {
		mergeStr = statusApproved.Render("MERGEABLE ✓")
	}
	s.WriteString(fmt.Sprintf("  %s %s\n", detailLabelStyle.Render("Mergeable:"), mergeStr))

	// CI Status
	if len(pr.StatusCheckRollup) > 0 {
		s.WriteString(fmt.Sprintf("  %s\n", detailLabelStyle.Render("CI Checks:")))
		for _, check := range pr.StatusCheckRollup {
			icon := "⏳"
			if check.Conclusion == "SUCCESS" {
				icon = "✓"
			} else if check.Conclusion == "FAILURE" {
				icon = "✗"
			}
			s.WriteString(fmt.Sprintf("    %s %s\n", icon, check.Name))
		}
	}

	// Reviews
	if len(pr.Reviews.Nodes) > 0 {
		s.WriteString(fmt.Sprintf("\n  %s\n", detailLabelStyle.Render("Reviewers:")))
		seen := make(map[string]bool)
		for _, rev := range pr.Reviews.Nodes {
			if seen[rev.Author.Login] {
				continue
			}
			seen[rev.Author.Login] = true
			icon := "⏳"
			switch rev.State {
			case "APPROVED":
				icon = "✓"
			case "CHANGES_REQUESTED":
				icon = "✗"
			case "COMMENTED":
				icon = "💬"
			}
			s.WriteString(fmt.Sprintf("    %s @%s (%s)\n", icon, rev.Author.Login, rev.State))
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

		s.WriteString(fmt.Sprintf("\n  %s\n",
			threadHeaderStyle.Render(fmt.Sprintf("📝 Unresolved Threads (%d)", unresolved))))

		for i, t := range m.detailThreads {
			if t.IsResolved {
				continue
			}
			selected := i == m.threadCursor
			prefix := "  "
			if selected {
				prefix = "▸ "
			}
			s.WriteString(fmt.Sprintf("  %s%s:%d\n", prefix, t.Path, t.Line))
			if len(t.Comments) > 0 {
				last := t.Comments[len(t.Comments)-1]
				s.WriteString(fmt.Sprintf("    %s: %s\n",
					threadAuthorStyle.Render("@"+last.Author),
					truncate(last.Body, 80)))
			}
			s.WriteString("\n")
		}

		if resolved > 0 {
			s.WriteString(fmt.Sprintf("  %s\n",
				repoStyle.Render(fmt.Sprintf("✅ Resolved Threads (%d) — collapsed", resolved))))
		}
	}

	// URL
	s.WriteString(fmt.Sprintf("\n  %s %s\n",
		detailLabelStyle.Render("URL:"),
		urlStyle.Render(pr.URL)))

	// Help
	s.WriteString(fmt.Sprintf("\n  %s\n",
		helpStyle.Render("[o] open in browser · [esc] back · [q] quit")))

	return s.String()
}

func (m dashModel) renderHelp() string {
	switch m.section {
	case sectionWorkspace:
		return helpStyle.Render("  [↑↓] navigate · [tab] sections · [enter] open · [o] github.com · [p] pull · [P] push · [f] fetch all · [R] refresh · [q] quit")
	default:
		return helpStyle.Render("  [↑↓] navigate · [tab] sections · [enter] expand · [o] github.com · [R] refresh · [q] quit")
	}
}

func timeSince(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh ago", int(d.Hours()))
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
