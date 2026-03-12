package cmd

import (
	"fmt"
	"os"

	"github.com/cheenu1092-oss/prflow/internal/config"
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
		return tui.RunOnboarding()
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

func runList() error {
	fmt.Println("prflow ls — quick list (TODO)")
	return nil
}

func runConfig() error {
	cfgPath := config.Path()
	fmt.Printf("Config: %s\n", cfgPath)
	fmt.Println("Open with: $EDITOR " + cfgPath)
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
  version   Print version`)
}
