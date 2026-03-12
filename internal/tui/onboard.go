package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cheenu1092-oss/prflow/internal/config"
	"github.com/cheenu1092-oss/prflow/internal/gh"
)

type onboardStep int

const (
	stepAuth onboardStep = iota
	stepScanRepos
	stepSelectRepos
	stepSelectFavorites
	stepDone
)

type onboardModel struct {
	step     onboardStep
	username string
	authErr  string

	// Repos found
	allRepos  []repoItem
	cursor    int
	
	// For favorites step
	favCursor int

	width  int
	height int
}

type repoItem struct {
	name     string
	selected bool
	starred  bool
}

type authCheckMsg struct {
	username string
	err      error
}

type repoScanMsg struct {
	repos []string
	prs   map[string]int // repo -> open PR count
	err   error
}

func RunOnboarding() error {
	m := onboardModel{step: stepAuth}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m onboardModel) Init() tea.Cmd {
	return checkAuth
}

func checkAuth() tea.Msg {
	username, err := gh.CheckAuth()
	return authCheckMsg{username: username, err: err}
}

func scanRepos() tea.Msg {
	// Get repos from user's PRs
	prs, err := gh.SearchMyPRs()
	if err != nil {
		return repoScanMsg{err: err}
	}

	repoMap := make(map[string]int)
	for _, pr := range prs {
		repoMap[pr.Repository.NameWithOwner]++
	}

	// Also get user's own repos
	repos, _ := gh.ListUserRepos()
	for _, r := range repos {
		if _, exists := repoMap[r]; !exists {
			repoMap[r] = 0
		}
	}

	var repoNames []string
	for name := range repoMap {
		repoNames = append(repoNames, name)
	}

	return repoScanMsg{repos: repoNames, prs: repoMap}
}

func (m onboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	switch m.step {
	case stepAuth:
		return m.updateAuth(msg)
	case stepScanRepos:
		return m.updateScan(msg)
	case stepSelectRepos:
		return m.updateSelectRepos(msg)
	case stepSelectFavorites:
		return m.updateSelectFavorites(msg)
	case stepDone:
		return m.updateDone(msg)
	}

	return m, nil
}

func (m onboardModel) updateAuth(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case authCheckMsg:
		if msg.err != nil {
			m.authErr = fmt.Sprintf("gh CLI not authenticated. Run: gh auth login")
			return m, nil
		}
		m.username = msg.username
		m.step = stepScanRepos
		return m, scanRepos
	}
	return m, nil
}

func (m onboardModel) updateScan(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case repoScanMsg:
		if msg.err != nil {
			m.authErr = fmt.Sprintf("Failed to scan repos: %v", msg.err)
			return m, nil
		}
		for _, name := range msg.repos {
			prCount := msg.prs[name]
			item := repoItem{name: name, selected: prCount > 0}
			m.allRepos = append(m.allRepos, item)
		}
		m.step = stepSelectRepos
		return m, nil
	}
	return m, nil
}

func (m onboardModel) updateSelectRepos(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.allRepos)-1 {
				m.cursor++
			}
		case " ":
			m.allRepos[m.cursor].selected = !m.allRepos[m.cursor].selected
		case "a":
			allSelected := true
			for _, r := range m.allRepos {
				if !r.selected {
					allSelected = false
					break
				}
			}
			for i := range m.allRepos {
				m.allRepos[i].selected = !allSelected
			}
		case "enter":
			m.step = stepSelectFavorites
			m.favCursor = 0
			return m, nil
		}
	}
	return m, nil
}

func (m onboardModel) updateSelectFavorites(msg tea.Msg) (tea.Model, tea.Cmd) {
	selected := m.selectedRepos()
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.favCursor > 0 {
				m.favCursor--
			}
		case "down", "j":
			if m.favCursor < len(selected)-1 {
				m.favCursor++
			}
		case " ":
			// Find this repo in allRepos and toggle star
			if m.favCursor < len(selected) {
				name := selected[m.favCursor]
				for i := range m.allRepos {
					if m.allRepos[i].name == name {
						m.allRepos[i].starred = !m.allRepos[i].starred
						break
					}
				}
			}
		case "enter":
			// Save config
			m.saveConfig()
			m.step = stepDone
			return m, nil
		}
	}
	return m, nil
}

func (m onboardModel) updateDone(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m onboardModel) selectedRepos() []string {
	var selected []string
	for _, r := range m.allRepos {
		if r.selected {
			selected = append(selected, r.name)
		}
	}
	return selected
}

func (m onboardModel) starredRepos() []string {
	var starred []string
	for _, r := range m.allRepos {
		if r.starred {
			starred = append(starred, r.name)
		}
	}
	return starred
}

func (m onboardModel) saveConfig() {
	cfg := config.DefaultConfig()
	cfg.Repos = m.selectedRepos()
	cfg.Favorites = m.starredRepos()
	config.Save(cfg)
}

func (m onboardModel) View() string {
	var s strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPrimary).
		Padding(1, 0).
		Render("  Welcome to PRFlow! 🚀")

	s.WriteString(title + "\n\n")

	switch m.step {
	case stepAuth:
		s.WriteString(m.viewAuth())
	case stepScanRepos:
		s.WriteString(m.viewScan())
	case stepSelectRepos:
		s.WriteString(m.viewSelectRepos())
	case stepSelectFavorites:
		s.WriteString(m.viewSelectFavorites())
	case stepDone:
		s.WriteString(m.viewDone())
	}

	return s.String()
}

func (m onboardModel) viewAuth() string {
	if m.authErr != "" {
		return fmt.Sprintf("  Step 1/4: GitHub Authentication\n\n  ✗ %s\n", m.authErr)
	}
	if m.username != "" {
		return fmt.Sprintf("  Step 1/4: GitHub Authentication\n\n  ✓ Authenticated as @%s\n", m.username)
	}
	return "  Step 1/4: GitHub Authentication\n\n  Checking gh CLI...\n"
}

func (m onboardModel) viewScan() string {
	return "  Step 2/4: Scanning Repos\n\n  Scanning your recent activity...\n"
}

func (m onboardModel) viewSelectRepos() string {
	var s strings.Builder
	s.WriteString(fmt.Sprintf("  Step 2/4: Select Repos (%d found)\n\n", len(m.allRepos)))

	maxShow := m.height - 10
	if maxShow < 5 {
		maxShow = 15
	}
	start := 0
	if m.cursor >= maxShow {
		start = m.cursor - maxShow + 1
	}

	for i := start; i < len(m.allRepos) && i < start+maxShow; i++ {
		r := m.allRepos[i]
		cursor := "  "
		if i == m.cursor {
			cursor = "▸ "
		}
		check := "[ ]"
		if r.selected {
			check = "[x]"
		}
		s.WriteString(fmt.Sprintf("  %s%s %s\n", cursor, check, r.name))
	}

	s.WriteString(fmt.Sprintf("\n  %s\n",
		helpStyle.Render("[space] toggle · [a] select all · [enter] next · [q] quit")))
	return s.String()
}

func (m onboardModel) viewSelectFavorites() string {
	var s strings.Builder
	selected := m.selectedRepos()
	s.WriteString(fmt.Sprintf("  Step 3/4: Star your favorites (%d repos)\n\n", len(selected)))
	s.WriteString("  Favorites get detailed tracking in the sidebar.\n\n")

	for i, name := range selected {
		cursor := "  "
		if i == m.favCursor {
			cursor = "▸ "
		}
		star := "  "
		for _, r := range m.allRepos {
			if r.name == name && r.starred {
				star = "★ "
				break
			}
		}
		s.WriteString(fmt.Sprintf("  %s%s%s\n", cursor, star, name))
	}

	s.WriteString(fmt.Sprintf("\n  %s\n",
		helpStyle.Render("[space] toggle star · [enter] finish · [q] quit")))
	return s.String()
}

func (m onboardModel) viewDone() string {
	selected := m.selectedRepos()
	starred := m.starredRepos()
	return fmt.Sprintf(`  Step 4/4: Setup Complete! ✓

  ✓ Config saved to %s
  ✓ Tracking %d repos (%d favorites)

  %s
`,
		config.Path(),
		len(selected),
		len(starred),
		helpStyle.Render("[enter] launch PRFlow"))
}
