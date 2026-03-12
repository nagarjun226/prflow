package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	colorPrimary   = lipgloss.Color("#7C3AED") // purple
	colorAccent    = lipgloss.Color("#F59E0B") // amber
	colorSuccess   = lipgloss.Color("#10B981") // green
	colorWarning   = lipgloss.Color("#F59E0B") // yellow
	colorDanger    = lipgloss.Color("#EF4444") // red
	colorMuted     = lipgloss.Color("#6B7280") // gray
	colorBg        = lipgloss.Color("#1F2937") // dark bg
	colorSidebarBg = lipgloss.Color("#111827") // darker bg
	colorWhite     = lipgloss.Color("#F9FAFB")
	colorCyan      = lipgloss.Color("#06B6D4")

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			PaddingLeft(1)

	sectionHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			PaddingLeft(1).
			PaddingBottom(1)

	sidebarStyle = lipgloss.NewStyle().
			Width(20).
			PaddingLeft(1).
			PaddingRight(1).
			BorderRight(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorMuted)

	mainPanelStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			PaddingRight(1)

	prItemStyle = lipgloss.NewStyle().
			PaddingLeft(1)

	prItemSelectedStyle = lipgloss.NewStyle().
				PaddingLeft(1).
				Bold(true).
				Foreground(colorCyan)

	repoStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	prNumberStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	urlStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	statusApproved = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	statusChanges = lipgloss.NewStyle().
			Foreground(colorDanger).
			Bold(true)

	statusPending = lipgloss.NewStyle().
			Foreground(colorWarning)

	statusClean = lipgloss.NewStyle().
			Foreground(colorSuccess)

	statusDirty = lipgloss.NewStyle().
			Foreground(colorWarning)

	statusBehind = lipgloss.NewStyle().
			Foreground(colorDanger)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			PaddingTop(1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorPrimary).
			Padding(0, 1).
			Width(80)

	favStarStyle = lipgloss.NewStyle().
			Foreground(colorAccent)

	sidebarItemStyle = lipgloss.NewStyle().
				PaddingLeft(1).
				Foreground(colorMuted)

	sidebarItemSelectedStyle = lipgloss.NewStyle().
					PaddingLeft(1).
					Foreground(colorCyan).
					Bold(true)

	detailLabelStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Width(14)

	detailValueStyle = lipgloss.NewStyle().
				Foreground(colorWhite)

	threadHeaderStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true)

	threadBodyStyle = lipgloss.NewStyle().
			Foreground(colorWhite).
			PaddingLeft(2)

	threadAuthorStyle = lipgloss.NewStyle().
				Foreground(colorCyan).
				Bold(true)
)
