package cmd

import (
	"fmt"
	"os"
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
			return runList()
		case "config":
			return runConfig()
		case "doctor":
			return runDoctor()
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

func runList() error {
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

	sections := []struct {
		name    string
		section string
		icon    string
	}{
		{"Do Now", "do_now", "⚡"},
		{"Waiting", "waiting", "⏳"},
		{"Review", "review", "👀"},
	}

	total := 0
	for _, sec := range sections {
		prs, err := db.GetPRsBySection(sec.section)
		if err != nil {
			continue
		}
		if len(prs) == 0 {
			continue
		}
		total += len(prs)
		fmt.Printf("\n%s %s (%d)\n", sec.icon, sec.name, len(prs))
		for _, pr := range prs {
			parts := strings.Split(pr.Repo, "/")
			repoShort := pr.Repo
			if len(parts) == 2 {
				repoShort = parts[1]
			}
			fmt.Printf("  #%-5d %-15s %s\n", pr.Number, repoShort, pr.Title)
		}
	}

	if total == 0 {
		fmt.Println("No cached PRs. Run 'prflow sync' first.")
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

func printUsage() {
	fmt.Println(`Usage: prflow [command]

Commands:
  (none)    Launch TUI dashboard
  setup     Run onboarding wizard
  sync      Force refresh PR cache
  ls        Quick list (no TUI)
  config    Show config path
  doctor    Check dependencies (gh, git, claude)
  version   Print version`)
}
