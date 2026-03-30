package tui

import (
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// autoRefreshMsg is sent when the auto-refresh timer fires
type autoRefreshMsg struct{}

// startAutoRefreshTimer creates a timer command for background refresh
func startAutoRefreshTimer(interval string) tea.Cmd {
	duration := parseRefreshInterval(interval)
	if duration == 0 {
		duration = 2 * time.Minute // default: 2 minutes
	}
	
	return tea.Tick(duration, func(t time.Time) tea.Msg {
		return autoRefreshMsg{}
	})
}

// parseRefreshInterval converts config strings like "2m", "30s", "1h" to time.Duration
func parseRefreshInterval(s string) time.Duration {
	if s == "" {
		return 0
	}
	
	// Try common formats
	d, err := time.ParseDuration(s)
	if err == nil {
		return d
	}
	
	// Fallback: assume minutes if no unit specified
	// e.g., "2" → 2 minutes
	if mins, err := strconv.Atoi(s); err == nil && mins > 0 {
		return time.Duration(mins) * time.Minute
	}
	
	return 0
}
