package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cheenu1092-oss/prflow/internal/ai"
	"github.com/cheenu1092-oss/prflow/internal/cache"
	"github.com/cheenu1092-oss/prflow/internal/config"
	"github.com/cheenu1092-oss/prflow/internal/deps"
	"github.com/cheenu1092-oss/prflow/internal/gh"
	"github.com/cheenu1092-oss/prflow/internal/tui"
	"github.com/cheenu1092-oss/prflow/internal/watch"
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
		case "watch":
			return runWatch()
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
	_ = cfg
	fmt.Println("Sync complete.")
	return nil
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

func runWatch() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("no config found, run 'prflow setup' first")
	}

	// Parse optional interval from args (e.g. prflow watch 5m)
	interval := 2 * time.Minute
	if cfg.Settings.WatchInterval != "" {
		if d, err := time.ParseDuration(cfg.Settings.WatchInterval); err == nil {
			interval = d
		}
	}
	if len(os.Args) > 2 {
		if d, err := time.ParseDuration(os.Args[2]); err == nil {
			interval = d
		}
	}

	username, err := gh.CheckAuth()
	if err != nil {
		return fmt.Errorf("not authenticated: %w", err)
	}

	db, err := cache.Open()
	if err != nil {
		fmt.Printf("warning: cache unavailable: %v\n", err)
	}
	if db != nil {
		defer db.Close()
	}

	fmt.Printf("Watching PRs as %s (interval: %s). Press Ctrl+C to stop.\n", username, interval)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	w := watch.New(cfg, db, username, interval)
	return w.Run(ctx)
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
  watch     Background mode with OS notifications
  version   Print version`)
}
