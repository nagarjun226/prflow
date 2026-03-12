package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ─── Color Palette ──────────────────────────────────────────
// Inspired by Tokyo Night / Catppuccin — dark, readable, purposeful
var (
	// Base colors
	colorBg        = lipgloss.Color("#1a1b26")
	colorSurface   = lipgloss.Color("#24283b")
	colorOverlay   = lipgloss.Color("#414868")
	colorBorder    = lipgloss.Color("#3b4261")
	colorBorderFoc = lipgloss.Color("#7aa2f7") // focused border
	colorText      = lipgloss.Color("#c0caf5")
	colorSubtext   = lipgloss.Color("#565f89")
	colorMuted     = lipgloss.Color("#444b6a")

	// Semantic colors
	colorPrimary = lipgloss.Color("#7aa2f7") // blue — primary actions
	colorSuccess = lipgloss.Color("#9ece6a") // green — approved, clean
	colorWarning = lipgloss.Color("#e0af68") // yellow — waiting, stale
	colorDanger  = lipgloss.Color("#f7768e") // red — conflicts, changes req
	colorInfo    = lipgloss.Color("#7dcfff") // cyan — info, links
	colorAccent  = lipgloss.Color("#ff9e64") // orange — favorites
	colorPurple  = lipgloss.Color("#bb9af7") // purple — PR numbers
	colorWhite   = lipgloss.Color("#c0caf5")

	// ─── Layout ─────────────────────────────────────────────

	// Header bar (full width)
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1a1b26")).
			Background(colorPrimary).
			Padding(0, 2)

	// Sidebar container
	sidebarStyle = lipgloss.NewStyle().
			Width(26).
			Padding(1, 1, 1, 1).
			BorderRight(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorBorder)

	// Main panel container
	mainPanelStyle = lipgloss.NewStyle().
			Padding(1, 2)

	// ─── Sidebar Items ──────────────────────────────────────

	sidebarSectionStyle = lipgloss.NewStyle().
				Foreground(colorSubtext).
				PaddingLeft(1)

	sidebarActiveStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true).
				PaddingLeft(0)

	sidebarCountStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	favHeaderStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true).
			MarginTop(1).
			PaddingLeft(1)

	favItemStyle = lipgloss.NewStyle().
			Foreground(colorSubtext).
			PaddingLeft(3)

	// ─── Section Headers ────────────────────────────────────

	sectionHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorBorder).
			MarginBottom(1).
			PaddingBottom(0)

	// ─── PR Items ───────────────────────────────────────────

	prCardStyle = lipgloss.NewStyle().
			BorderLeft(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(colorBorder).
			PaddingLeft(1).
			MarginBottom(1)

	prCardSelectedStyle = lipgloss.NewStyle().
				BorderLeft(true).
				BorderStyle(lipgloss.ThickBorder()).
				BorderForeground(colorPrimary).
				PaddingLeft(1).
				MarginBottom(1)

	prRepoStyle = lipgloss.NewStyle().
			Foreground(colorSubtext)

	prNumberStyle = lipgloss.NewStyle().
			Foreground(colorPurple).
			Bold(true)

	prTitleStyle = lipgloss.NewStyle().
			Foreground(colorWhite)

	prTitleSelectedStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true)

	// ─── Status Badges ──────────────────────────────────────

	badgeChanges = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1b26")).
			Background(colorDanger).
			Padding(0, 1).
			Bold(true)

	badgeConflict = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1b26")).
			Background(colorDanger).
			Padding(0, 1).
			Bold(true)

	badgeWaiting = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1b26")).
			Background(colorWarning).
			Padding(0, 1)

	badgeDraft = lipgloss.NewStyle().
			Foreground(colorSubtext).
			Border(lipgloss.NormalBorder()).
			BorderForeground(colorMuted).
			Padding(0, 1)

	badgeMerge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1b26")).
			Background(colorSuccess).
			Padding(0, 1).
			Bold(true)

	// ─── Workspace ──────────────────────────────────────────

	wsCardStyle = lipgloss.NewStyle().
			BorderLeft(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(colorBorder).
			PaddingLeft(1).
			MarginBottom(1)

	wsCardSelectedStyle = lipgloss.NewStyle().
				BorderLeft(true).
				BorderStyle(lipgloss.ThickBorder()).
				BorderForeground(colorPrimary).
				PaddingLeft(1).
				MarginBottom(1)

	wsCleanStyle = lipgloss.NewStyle().
			Foreground(colorSuccess)

	wsDirtyStyle = lipgloss.NewStyle().
			Foreground(colorWarning)

	wsBehindStyle = lipgloss.NewStyle().
			Foreground(colorDanger)

	wsMetaStyle = lipgloss.NewStyle().
			Foreground(colorSubtext)

	// ─── Detail View ────────────────────────────────────────

	detailLabelStyle = lipgloss.NewStyle().
				Foreground(colorSubtext).
				Width(16)

	detailValueStyle = lipgloss.NewStyle().
				Foreground(colorWhite)

	threadHeaderStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true).
				MarginTop(1)

	threadCardStyle = lipgloss.NewStyle().
			BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorWarning).
			PaddingLeft(1).
			MarginBottom(1)

	threadCardSelectedStyle = lipgloss.NewStyle().
				BorderLeft(true).
				BorderStyle(lipgloss.ThickBorder()).
				BorderForeground(colorPrimary).
				PaddingLeft(1).
				MarginBottom(1)

	threadAuthorStyle = lipgloss.NewStyle().
				Foreground(colorInfo).
				Bold(true)

	threadBodyStyle = lipgloss.NewStyle().
			Foreground(colorText)

	threadFileStyle = lipgloss.NewStyle().
			Foreground(colorPurple)

	// ─── URL & Links ────────────────────────────────────────

	urlStyle = lipgloss.NewStyle().
			Foreground(colorInfo).
			Italic(true)

	// ─── Help Bar ───────────────────────────────────────────

	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginTop(1)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorSubtext).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// ─── Status Bar ─────────────────────────────────────────

	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorInfo)

	statusErrorStyle = lipgloss.NewStyle().
				Foreground(colorDanger)

	// ─── Empty State ────────────────────────────────────────

	emptyStyle = lipgloss.NewStyle().
			Foreground(colorSubtext).
			Italic(true).
			PaddingLeft(2).
			PaddingTop(1)

	// Legacy aliases (keep old code working)
	statusApproved = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	statusChanges  = lipgloss.NewStyle().Foreground(colorDanger).Bold(true)
	statusPending  = lipgloss.NewStyle().Foreground(colorWarning)
	statusClean    = lipgloss.NewStyle().Foreground(colorSuccess)
	statusDirty    = lipgloss.NewStyle().Foreground(colorWarning)
	statusBehind   = lipgloss.NewStyle().Foreground(colorDanger)
	repoStyle      = lipgloss.NewStyle().Foreground(colorSubtext)
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)

	prItemStyle         = lipgloss.NewStyle()
	prItemSelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
)

// Helper to render a key-desc help pair
func helpPair(key, desc string) string {
	return helpKeyStyle.Render(key) + " " + helpDescStyle.Render(desc)
}
