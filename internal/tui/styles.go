package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ─── Color Palette ──────────────────────────────────────────
// Colors are derived from CurrentTheme. Call ApplyTheme to reinitialise.
var (
	// Base colors
	colorBg        lipgloss.Color
	colorSurface   lipgloss.Color
	colorOverlay   lipgloss.Color
	colorBorder    lipgloss.Color
	colorBorderFoc lipgloss.Color
	colorText      lipgloss.Color
	colorSubtext   lipgloss.Color
	colorMuted     lipgloss.Color

	// Semantic colors
	colorPrimary lipgloss.Color
	colorSuccess lipgloss.Color
	colorWarning lipgloss.Color
	colorDanger  lipgloss.Color
	colorInfo    lipgloss.Color
	colorAccent  lipgloss.Color
	colorPurple  lipgloss.Color
	colorWhite   lipgloss.Color

	// ─── Layout ─────────────────────────────────────────────

	headerStyle    lipgloss.Style
	sidebarStyle   lipgloss.Style
	mainPanelStyle lipgloss.Style

	// ─── Sidebar Items ──────────────────────────────────────

	sidebarSectionStyle lipgloss.Style
	sidebarActiveStyle  lipgloss.Style
	sidebarCountStyle   lipgloss.Style
	favHeaderStyle      lipgloss.Style
	favItemStyle        lipgloss.Style

	// ─── Section Headers ────────────────────────────────────

	sectionHeader lipgloss.Style

	// ─── PR Items ───────────────────────────────────────────

	prCardStyle         lipgloss.Style
	prCardSelectedStyle lipgloss.Style
	prRepoStyle         lipgloss.Style
	prNumberStyle       lipgloss.Style
	prTitleStyle        lipgloss.Style
	prTitleSelectedStyle lipgloss.Style

	// ─── Status Badges ──────────────────────────────────────

	badgeChanges  lipgloss.Style
	badgeConflict lipgloss.Style
	badgeWaiting  lipgloss.Style
	badgeDraft    lipgloss.Style
	badgeMerge    lipgloss.Style

	// ─── Workspace ──────────────────────────────────────────

	wsCardStyle         lipgloss.Style
	wsCardSelectedStyle lipgloss.Style
	wsCleanStyle        lipgloss.Style
	wsDirtyStyle        lipgloss.Style
	wsBehindStyle       lipgloss.Style
	wsMetaStyle         lipgloss.Style

	// ─── Detail View ────────────────────────────────────────

	detailLabelStyle       lipgloss.Style
	detailValueStyle       lipgloss.Style
	threadHeaderStyle      lipgloss.Style
	threadCardStyle        lipgloss.Style
	threadCardSelectedStyle lipgloss.Style
	threadAuthorStyle      lipgloss.Style
	threadBodyStyle        lipgloss.Style
	threadFileStyle        lipgloss.Style

	// ─── URL & Links ────────────────────────────────────────

	urlStyle lipgloss.Style

	// ─── Help Bar ───────────────────────────────────────────

	helpStyle     lipgloss.Style
	helpKeyStyle  lipgloss.Style
	helpDescStyle lipgloss.Style

	// ─── Status Bar ─────────────────────────────────────────

	statusBarStyle   lipgloss.Style
	statusErrorStyle lipgloss.Style

	// ─── Empty State ────────────────────────────────────────

	emptyStyle lipgloss.Style

	// Legacy aliases
	statusApproved      lipgloss.Style
	statusChanges       lipgloss.Style
	statusPending       lipgloss.Style
	statusClean         lipgloss.Style
	statusDirty         lipgloss.Style
	statusBehind        lipgloss.Style
	repoStyle           lipgloss.Style
	titleStyle          lipgloss.Style
	prItemStyle         lipgloss.Style
	prItemSelectedStyle lipgloss.Style
)

func init() {
	ApplyTheme(CurrentTheme)
}

// ApplyTheme reinitialises all style variables from the given theme.
func ApplyTheme(t Theme) {
	// Base colors
	colorBg = t.Bg
	colorSurface = t.Surface
	colorOverlay = t.Overlay
	colorBorder = t.Border
	colorBorderFoc = t.BorderFocus
	colorText = t.Text
	colorSubtext = t.Subtext
	colorMuted = t.Muted

	// Semantic colors
	colorPrimary = t.Primary
	colorSuccess = t.Success
	colorWarning = t.Warning
	colorDanger = t.Danger
	colorInfo = t.Info
	colorAccent = t.Accent
	colorPurple = t.Purple
	colorWhite = t.Text // maps to theme text color

	// ─── Layout ─────────────────────────────────────────────

	headerStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Bg).
		Background(t.Primary).
		Padding(0, 2)

	sidebarStyle = lipgloss.NewStyle().
		Width(26).
		Padding(1, 1, 1, 1).
		BorderRight(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(t.Border)

	mainPanelStyle = lipgloss.NewStyle().
		Padding(1, 2)

	// ─── Sidebar Items ──────────────────────────────────────

	sidebarSectionStyle = lipgloss.NewStyle().
		Foreground(t.Subtext).
		PaddingLeft(1)

	sidebarActiveStyle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		PaddingLeft(0)

	sidebarCountStyle = lipgloss.NewStyle().
		Foreground(t.Muted)

	favHeaderStyle = lipgloss.NewStyle().
		Foreground(t.Accent).
		Bold(true).
		MarginTop(1).
		PaddingLeft(1)

	favItemStyle = lipgloss.NewStyle().
		Foreground(t.Subtext).
		PaddingLeft(3)

	// ─── Section Headers ────────────────────────────────────

	sectionHeader = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Text).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(t.Border).
		MarginBottom(1).
		PaddingBottom(0)

	// ─── PR Items ───────────────────────────────────────────

	prCardStyle = lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(t.Border).
		PaddingLeft(1).
		MarginBottom(1)

	prCardSelectedStyle = lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(t.Primary).
		PaddingLeft(1).
		MarginBottom(1)

	prRepoStyle = lipgloss.NewStyle().
		Foreground(t.Subtext)

	prNumberStyle = lipgloss.NewStyle().
		Foreground(t.Purple).
		Bold(true)

	prTitleStyle = lipgloss.NewStyle().
		Foreground(t.Text)

	prTitleSelectedStyle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	// ─── Status Badges ──────────────────────────────────────

	badgeChanges = lipgloss.NewStyle().
		Foreground(t.Bg).
		Background(t.Danger).
		Padding(0, 1).
		Bold(true)

	badgeConflict = lipgloss.NewStyle().
		Foreground(t.Bg).
		Background(t.Danger).
		Padding(0, 1).
		Bold(true)

	badgeWaiting = lipgloss.NewStyle().
		Foreground(t.Bg).
		Background(t.Warning).
		Padding(0, 1)

	badgeDraft = lipgloss.NewStyle().
		Foreground(t.Subtext).
		Border(lipgloss.NormalBorder()).
		BorderForeground(t.Muted).
		Padding(0, 1)

	badgeMerge = lipgloss.NewStyle().
		Foreground(t.Bg).
		Background(t.Success).
		Padding(0, 1).
		Bold(true)

	// ─── Workspace ──────────────────────────────────────────

	wsCardStyle = lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(t.Border).
		PaddingLeft(1).
		MarginBottom(1)

	wsCardSelectedStyle = lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(t.Primary).
		PaddingLeft(1).
		MarginBottom(1)

	wsCleanStyle = lipgloss.NewStyle().
		Foreground(t.Success)

	wsDirtyStyle = lipgloss.NewStyle().
		Foreground(t.Warning)

	wsBehindStyle = lipgloss.NewStyle().
		Foreground(t.Danger)

	wsMetaStyle = lipgloss.NewStyle().
		Foreground(t.Subtext)

	// ─── Detail View ────────────────────────────────────────

	detailLabelStyle = lipgloss.NewStyle().
		Foreground(t.Subtext).
		Width(16)

	detailValueStyle = lipgloss.NewStyle().
		Foreground(t.Text)

	threadHeaderStyle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		MarginTop(1)

	threadCardStyle = lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(t.Warning).
		PaddingLeft(1).
		MarginBottom(1)

	threadCardSelectedStyle = lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(t.Primary).
		PaddingLeft(1).
		MarginBottom(1)

	threadAuthorStyle = lipgloss.NewStyle().
		Foreground(t.Info).
		Bold(true)

	threadBodyStyle = lipgloss.NewStyle().
		Foreground(t.Text)

	threadFileStyle = lipgloss.NewStyle().
		Foreground(t.Purple)

	// ─── URL & Links ────────────────────────────────────────

	urlStyle = lipgloss.NewStyle().
		Foreground(t.Info).
		Italic(true)

	// ─── Help Bar ───────────────────────────────────────────

	helpStyle = lipgloss.NewStyle().
		Foreground(t.Muted).
		MarginTop(1)

	helpKeyStyle = lipgloss.NewStyle().
		Foreground(t.Subtext).
		Bold(true)

	helpDescStyle = lipgloss.NewStyle().
		Foreground(t.Muted)

	// ─── Status Bar ─────────────────────────────────────────

	statusBarStyle = lipgloss.NewStyle().
		Foreground(t.Info)

	statusErrorStyle = lipgloss.NewStyle().
		Foreground(t.Danger)

	// ─── Empty State ────────────────────────────────────────

	emptyStyle = lipgloss.NewStyle().
		Foreground(t.Subtext).
		Italic(true).
		PaddingLeft(2).
		PaddingTop(1)

	// Legacy aliases
	statusApproved = lipgloss.NewStyle().Foreground(t.Success).Bold(true)
	statusChanges = lipgloss.NewStyle().Foreground(t.Danger).Bold(true)
	statusPending = lipgloss.NewStyle().Foreground(t.Warning)
	statusClean = lipgloss.NewStyle().Foreground(t.Success)
	statusDirty = lipgloss.NewStyle().Foreground(t.Warning)
	statusBehind = lipgloss.NewStyle().Foreground(t.Danger)
	repoStyle = lipgloss.NewStyle().Foreground(t.Subtext)
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Primary)

	prItemStyle = lipgloss.NewStyle()
	prItemSelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(t.Primary)
}

// Helper to render a key-desc help pair
func helpPair(key, desc string) string {
	return helpKeyStyle.Render(key) + " " + helpDescStyle.Render(desc)
}
