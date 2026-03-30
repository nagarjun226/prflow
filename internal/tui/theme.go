package tui

import "github.com/charmbracelet/lipgloss"

// Theme holds all color values used throughout the TUI.
type Theme struct {
	Primary     lipgloss.Color
	Success     lipgloss.Color
	Warning     lipgloss.Color
	Danger      lipgloss.Color
	Info        lipgloss.Color
	Accent      lipgloss.Color
	Purple      lipgloss.Color
	Bg          lipgloss.Color
	Surface     lipgloss.Color
	Overlay     lipgloss.Color
	Border      lipgloss.Color
	BorderFocus lipgloss.Color
	Text        lipgloss.Color
	Subtext     lipgloss.Color
	Muted       lipgloss.Color
}

// CurrentTheme is the active theme used by style variables.
var CurrentTheme Theme

func init() {
	CurrentTheme = DarkTheme()
}

// DarkTheme returns the default Tokyo Night inspired dark theme.
func DarkTheme() Theme {
	return Theme{
		Primary:     lipgloss.Color("#7aa2f7"),
		Success:     lipgloss.Color("#9ece6a"),
		Warning:     lipgloss.Color("#e0af68"),
		Danger:      lipgloss.Color("#f7768e"),
		Info:        lipgloss.Color("#7dcfff"),
		Accent:      lipgloss.Color("#ff9e64"),
		Purple:      lipgloss.Color("#bb9af7"),
		Bg:          lipgloss.Color("#1a1b26"),
		Surface:     lipgloss.Color("#24283b"),
		Overlay:     lipgloss.Color("#414868"),
		Border:      lipgloss.Color("#3b4261"),
		BorderFocus: lipgloss.Color("#7aa2f7"),
		Text:        lipgloss.Color("#c0caf5"),
		Subtext:     lipgloss.Color("#565f89"),
		Muted:       lipgloss.Color("#444b6a"),
	}
}

// LightTheme returns a light-friendly theme with high contrast colors.
func LightTheme() Theme {
	return Theme{
		Primary:     lipgloss.Color("#2e59a1"),
		Success:     lipgloss.Color("#387a2e"),
		Warning:     lipgloss.Color("#9a6700"),
		Danger:      lipgloss.Color("#cf222e"),
		Info:        lipgloss.Color("#0969da"),
		Accent:      lipgloss.Color("#bc4c00"),
		Purple:      lipgloss.Color("#8250df"),
		Bg:          lipgloss.Color("#ffffff"),
		Surface:     lipgloss.Color("#f6f8fa"),
		Overlay:     lipgloss.Color("#d0d7de"),
		Border:      lipgloss.Color("#d0d7de"),
		BorderFocus: lipgloss.Color("#2e59a1"),
		Text:        lipgloss.Color("#1f2328"),
		Subtext:     lipgloss.Color("#656d76"),
		Muted:       lipgloss.Color("#8b949e"),
	}
}

// SetTheme sets CurrentTheme by name and reinitializes all styles.
// Valid names are "dark", "light", and "auto". Unknown values fall back to "auto".
func SetTheme(name string) {
	switch name {
	case "dark":
		CurrentTheme = DarkTheme()
	case "light":
		CurrentTheme = LightTheme()
	case "auto":
		autoDetectTheme()
	default:
		// Unknown theme name — fall back to auto detection
		autoDetectTheme()
	}
	ApplyTheme(CurrentTheme)
}

func autoDetectTheme() {
	if lipgloss.HasDarkBackground() {
		CurrentTheme = DarkTheme()
	} else {
		CurrentTheme = LightTheme()
	}
}
