package tui

import (
	"testing"
	"time"
)

func TestParseRefreshInterval(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
	}{
		{
			name:     "2 minutes",
			input:    "2m",
			expected: 2 * time.Minute,
		},
		{
			name:     "30 seconds",
			input:    "30s",
			expected: 30 * time.Second,
		},
		{
			name:     "1 hour",
			input:    "1h",
			expected: 1 * time.Hour,
		},
		{
			name:     "mixed units",
			input:    "1h30m",
			expected: 90 * time.Minute,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "invalid format",
			input:    "invalid",
			expected: 0,
		},
		{
			name:     "bare number (minutes)",
			input:    "5",
			expected: 5 * time.Minute,
		},
		{
			name:     "bare number 10",
			input:    "10",
			expected: 10 * time.Minute,
		},
		{
			name:     "zero",
			input:    "0",
			expected: 0,
		},
		{
			name:     "negative number",
			input:    "-5",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRefreshInterval(tt.input)
			if result != tt.expected {
				t.Errorf("parseRefreshInterval(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStartAutoRefreshTimer(t *testing.T) {
	// Test that timer creates a command (basic smoke test)
	cmd := startAutoRefreshTimer("2m")
	if cmd == nil {
		t.Error("startAutoRefreshTimer should return a non-nil command")
	}

	// Test with empty interval (should default to 2m)
	cmdDefault := startAutoRefreshTimer("")
	if cmdDefault == nil {
		t.Error("startAutoRefreshTimer with empty interval should return a non-nil command")
	}
}
