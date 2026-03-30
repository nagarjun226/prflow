package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/cheenu1092-oss/prflow/internal/ai"
	"github.com/cheenu1092-oss/prflow/internal/cache"
	"github.com/cheenu1092-oss/prflow/internal/config"
	"github.com/cheenu1092-oss/prflow/internal/deps"
	"github.com/cheenu1092-oss/prflow/internal/gh"
	"github.com/cheenu1092-oss/prflow/internal/tui"
)

func Execute() error {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			fmt.Println("prflow v0.1.0")
			return nil
		case "setup":
			return tui.RunOnboarding()
		case "sync":
			return runSync()
		case "ls":
			jsonFlag := hasFlag(os.Args[2:], "--json")
			return runListTo(os.Stdout, jsonFlag)
		case "config":
			return runConfig()
		case "doctor":
			return runDoctor()
		case "open":
			return runOpen()
		default:
			fmt.Printf("Unknown command: %s\n", os.Args[1])
			printUsage()
			return nil
		}
	}

	// Default: launch TUI
	cfg, err := config.Load()
	if err != nil || len(cfg.Repos) == 0 {
		// First run — launch onboarding
		if err := tui.RunOnboarding(); err != nil {
			return err
		}
		// Reload config after onboarding
		cfg, err = config.Load()
		if err != nil || len(cfg.Repos) == 0 {
			fmt.Println("Setup complete. Run 'prflow' again to launch the dashboard.")
			return nil
		}
	}
	return tui.RunDashboard(cfg)
}

func runSync() error {
	fmt.Println("Syncing PRs...")
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("no config found, run 'prflow setup' first")
	}
	cfg.Validate()

	db, err := cache.Open()
	if err != nil {
		return fmt.Errorf("failed to open cache: %w", err)
	}
	defer db.Close()

	username, _ := gh.CheckAuth()

	// Search for authored PRs
	myPRs, _ := gh.SearchMyPRs()
	repoSet := make(map[string]bool)
	for _, pr := range myPRs {
		if pr.Repository.NameWithOwner != "" {
			repoSet[pr.Repository.NameWithOwner] = true
		}
	}
	for _, repo := range cfg.Repos {
		repoSet[repo] = true
	}

	synced := 0
	for repo := range repoSet {
		repoPRs, err := gh.ListPRsForRepo(repo)
		if err != nil {
			fmt.Printf("  ✗ %s: %v\n", repo, err)
			continue
		}
		for i := range repoPRs {
			pr := &repoPRs[i]
			section := classifyPR(pr, username)
			db.UpsertPR(pr, repo, section)
			synced++
		}
		fmt.Printf("  ✓ %s (%d PRs)\n", repo, len(repoPRs))
	}

	// Also sync review requests
	reviewPRs, _ := gh.SearchReviewRequests()
	for i := range reviewPRs {
		pr := &reviewPRs[i]
		db.UpsertPR(pr, pr.Repository.NameWithOwner, "review")
		synced++
	}

	fmt.Printf("Sync complete: %d PRs cached across %d repos.\n", synced, len(repoSet))
	return nil
}

// classifyPR determines which section a PR belongs in
func classifyPR(pr *gh.PR, username string) string {
	isMyPR := strings.EqualFold(pr.Author.Login, username)
	if isMyPR {
		switch {
		case pr.ReviewDecision == "CHANGES_REQUESTED":
			return "do_now"
		case pr.ReviewDecision == "APPROVED":
			return "do_now"
		case pr.Mergeable == "CONFLICTING":
			return "do_now"
		default:
			return "waiting"
		}
	}
	return "review"
}

// hasFlag checks whether flag appears in args.
func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// jsonPR is the JSON-output representation of a cached PR.
type jsonPR struct {
	Repo           string `json:"repo"`
	Number         int    `json:"number"`
	Title          string `json:"title"`
	ReviewDecision string `json:"review_decision"`
	Mergeable      string `json:"mergeable"`
	UpdatedAt      string `json:"updated_at"`
}

// jsonOutput is the top-level JSON structure for prflow ls --json.
type jsonOutput struct {
	DoNow          []jsonPR `json:"do_now"`
	Waiting        []jsonPR `json:"waiting"`
	Review         []jsonPR `json:"review"`
	NeedsAttention []jsonPR `json:"needs_attention"`
}

func toJSONPRs(prs []cache.CachedPR) []jsonPR {
	out := make([]jsonPR, 0, len(prs))
	for _, p := range prs {
		out = append(out, jsonPR{
			Repo:           p.Repo,
			Number:         p.Number,
			Title:          p.Title,
			ReviewDecision: p.ReviewDecision,
			Mergeable:      p.Mergeable,
			UpdatedAt:      p.UpdatedAt,
		})
	}
	return out
}

// runListTo writes the PR list to w. When jsonMode is true it outputs JSON;
// otherwise it writes human-readable plaintext.
func runListTo(w io.Writer, jsonMode bool) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("no config found, run 'prflow setup' first")
	}
	_ = cfg

	db, err := cache.Open()
	if err != nil {
		return fmt.Errorf("failed to open cache: %w", err)
	}
	defer db.Close()

	sections := []struct {
		name string
		key  string
		icon string
	}{
		{"Do Now", "do_now", "⚡"},
		{"Waiting", "waiting", "⏳"},
		{"Review", "review", "👀"},
		{"Needs Attention", "needs_attention", "🔔"},
	}

	// Collect PRs per section.
	prsBySection := make(map[string][]cache.CachedPR)
	for _, sec := range sections {
		prs, err := db.GetPRsBySection(sec.key)
		if err != nil {
			prs = nil
		}
		prsBySection[sec.key] = prs
	}

	if jsonMode {
		out := jsonOutput{
			DoNow:          toJSONPRs(prsBySection["do_now"]),
			Waiting:        toJSONPRs(prsBySection["waiting"]),
			Review:         toJSONPRs(prsBySection["review"]),
			NeedsAttention: toJSONPRs(prsBySection["needs_attention"]),
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	// Plaintext output.
	total := 0
	for _, sec := range sections {
		prs := prsBySection[sec.key]
		if len(prs) == 0 {
			continue
		}
		total += len(prs)
		fmt.Fprintf(w, "\n%s %s (%d)\n", sec.icon, sec.name, len(prs))
		for _, pr := range prs {
			parts := strings.Split(pr.Repo, "/")
			repoShort := pr.Repo
			if len(parts) == 2 {
				repoShort = parts[1]
			}
			fmt.Fprintf(w, "  #%-5d %-15s %s\n", pr.Number, repoShort, pr.Title)
		}
	}

	if total == 0 {
		fmt.Fprintln(w, "No cached PRs. Run 'prflow sync' first.")
	}
	return nil
}

func runConfig() error {
	cfgPath := config.Path()
	fmt.Printf("Config: %s\n", cfgPath)
	fmt.Println("Open with: $EDITOR " + cfgPath)
	return nil
}

func runDoctor() error {
	fmt.Println(deps.PrintStatus())

	if ai.Available() {
		fmt.Println("🤖 AI features: ENABLED")
		fmt.Println("   Claude Code detected — PR analysis, review assistance, and auto-fix available.")
	} else {
		fmt.Println("🤖 AI features: DISABLED (optional)")
		fmt.Println("   Install Claude Code for AI-powered PR analysis:")
		fmt.Println("   npm install -g @anthropic-ai/claude-code")
		fmt.Println("   Then run: claude  (to complete auth)")
		fmt.Println("")
		fmt.Println("   Without it, PRFlow works as a standard PR dashboard.")
	}

	if err := deps.CheckRequired(); err != nil {
		fmt.Printf("\n⚠️  %v\n", err)
		return err
	}

	fmt.Println("\n✓ All required dependencies OK")
	return nil
}

// openArgs holds the parsed result of a prflow open argument.
type openArgs struct {
	Repo   string // "org/repo" or empty
	Number int    // PR number or 0
}

// parseOpenArgs parses the argument to `prflow open`.
// Supported formats:
//   - "org/repo#42" -> repo="org/repo", number=42
//   - "#42"         -> repo="", number=42
//   - "org/repo"    -> repo="org/repo", number=0
//   - ""            -> repo="", number=0
func parseOpenArgs(arg string) (openArgs, error) {
	if arg == "" {
		return openArgs{}, nil
	}

	// Check for # separator
	if idx := strings.Index(arg, "#"); idx >= 0 {
		numStr := arg[idx+1:]
		repo := arg[:idx]
		if numStr == "" {
			return openArgs{}, fmt.Errorf("missing PR number after #")
		}
		n, err := strconv.Atoi(numStr)
		if err != nil || n <= 0 {
			return openArgs{}, fmt.Errorf("invalid PR number: %s", numStr)
		}
		return openArgs{Repo: repo, Number: n}, nil
	}

	// No #, treat as repo
	if strings.Contains(arg, "/") {
		return openArgs{Repo: arg, Number: 0}, nil
	}

	return openArgs{}, fmt.Errorf("invalid argument: %s (expected org/repo, #number, or org/repo#number)", arg)
}

// repoFromRemote infers the "org/repo" from the current directory's git remote.
var repoFromRemote = func() (string, error) {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return "", fmt.Errorf("could not detect repo from git remote: %w", err)
	}
	return parseRepoFromURL(strings.TrimSpace(string(out)))
}

// parseRepoFromURL extracts "org/repo" from a git remote URL.
// Supports both HTTPS and SSH formats.
func parseRepoFromURL(rawURL string) (string, error) {
	u := rawURL
	// SSH: git@github.com:org/repo.git
	if strings.HasPrefix(u, "git@") {
		u = strings.TrimPrefix(u, "git@")
		if i := strings.Index(u, ":"); i >= 0 {
			u = u[i+1:]
		}
		u = strings.TrimSuffix(u, ".git")
		if strings.Contains(u, "/") {
			return u, nil
		}
	}
	// HTTPS: https://github.com/org/repo.git
	u = strings.TrimSuffix(u, ".git")
	parts := strings.Split(u, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1], nil
	}
	return "", fmt.Errorf("could not parse repo from URL: %s", rawURL)
}

func runOpen() error {
	arg := ""
	if len(os.Args) > 2 {
		arg = os.Args[2]
	}

	parsed, err := parseOpenArgs(arg)
	if err != nil {
		return err
	}

	repo := parsed.Repo

	// If no repo specified, infer from git remote
	if repo == "" {
		inferred, err := repoFromRemote()
		if err != nil {
			return err
		}
		repo = inferred
	}

	var url string
	if parsed.Number > 0 {
		url = fmt.Sprintf("https://github.com/%s/pull/%d", repo, parsed.Number)
	} else {
		url = fmt.Sprintf("https://github.com/%s/pulls", repo)
	}

	fmt.Printf("Opening %s\n", url)
	return gh.OpenInBrowser(url)
}

func printUsage() {
	fmt.Println(`Usage: prflow [command]

Commands:
  (none)    Launch TUI dashboard
  setup     Run onboarding wizard
  sync      Force refresh PR cache
  ls        Quick list (no TUI) [--json for JSON output]
  config    Show config path
  open      Open PR in browser (org/repo#42, #42, org/repo)
  doctor    Check dependencies (gh, git, claude)
  version   Print version`)
}
