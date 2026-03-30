package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func allFieldsNonEmpty(t *testing.T, theme Theme, name string) {
	t.Helper()
	fields := []struct {
		label string
		value lipgloss.Color
	}{
		{"Primary", theme.Primary},
		{"Success", theme.Success},
		{"Warning", theme.Warning},
		{"Danger", theme.Danger},
		{"Info", theme.Info},
		{"Accent", theme.Accent},
		{"Purple", theme.Purple},
		{"Bg", theme.Bg},
		{"Surface", theme.Surface},
		{"Overlay", theme.Overlay},
		{"Border", theme.Border},
		{"BorderFocus", theme.BorderFocus},
		{"Text", theme.Text},
		{"Subtext", theme.Subtext},
		{"Muted", theme.Muted},
	}
	for _, f := range fields {
		if string(f.value) == "" {
			t.Errorf("%s theme: field %s is empty", name, f.label)
		}
	}
}

func TestDarkTheme(t *testing.T) {
	theme := DarkTheme()
	allFieldsNonEmpty(t, theme, "Dark")

	// Verify it uses the expected Tokyo Night primary color
	if string(theme.Primary) != "#7aa2f7" {
		t.Errorf("expected Tokyo Night primary #7aa2f7, got %s", string(theme.Primary))
	}
}

func TestLightTheme(t *testing.T) {
	theme := LightTheme()
	allFieldsNonEmpty(t, theme, "Light")

	dark := DarkTheme()
	// Key colors should differ between light and dark
	diffs := 0
	if theme.Primary != dark.Primary {
		diffs++
	}
	if theme.Bg != dark.Bg {
		diffs++
	}
	if theme.Text != dark.Text {
		diffs++
	}
	if theme.Surface != dark.Surface {
		diffs++
	}
	if theme.Subtext != dark.Subtext {
		diffs++
	}
	if diffs < 4 {
		t.Errorf("expected light theme to differ from dark in at least 4 key fields, got %d differences", diffs)
	}
}

func TestSetThemeDark(t *testing.T) {
	SetTheme("dark")
	dark := DarkTheme()
	if CurrentTheme.Primary != dark.Primary {
		t.Errorf("expected dark primary %s, got %s", string(dark.Primary), string(CurrentTheme.Primary))
	}
	if CurrentTheme.Bg != dark.Bg {
		t.Errorf("expected dark bg %s, got %s", string(dark.Bg), string(CurrentTheme.Bg))
	}
	if CurrentTheme.Text != dark.Text {
		t.Errorf("expected dark text %s, got %s", string(dark.Text), string(CurrentTheme.Text))
	}
}

func TestSetThemeLight(t *testing.T) {
	SetTheme("light")
	light := LightTheme()
	if CurrentTheme.Primary != light.Primary {
		t.Errorf("expected light primary %s, got %s", string(light.Primary), string(CurrentTheme.Primary))
	}
	if CurrentTheme.Bg != light.Bg {
		t.Errorf("expected light bg %s, got %s", string(light.Bg), string(CurrentTheme.Bg))
	}
	if CurrentTheme.Text != light.Text {
		t.Errorf("expected light text %s, got %s", string(light.Text), string(CurrentTheme.Text))
	}
	// Restore dark theme so other tests aren't affected
	SetTheme("dark")
}

func TestSetThemeAuto(t *testing.T) {
	// Should not panic regardless of terminal capabilities
	SetTheme("auto")

	// CurrentTheme should be either dark or light — just verify fields are populated
	allFieldsNonEmpty(t, CurrentTheme, "Auto")

	// Restore dark theme
	SetTheme("dark")
}

func TestSetThemeInvalid(t *testing.T) {
	// Unknown theme should fall back to auto (which selects dark or light)
	SetTheme("nonexistent")
	allFieldsNonEmpty(t, CurrentTheme, "Invalid/Fallback")

	// Restore dark theme
	SetTheme("dark")
}

func TestApplyTheme(t *testing.T) {
	// Apply light theme and verify style variables changed
	light := LightTheme()
	ApplyTheme(light)

	if colorPrimary != light.Primary {
		t.Errorf("colorPrimary not updated: got %s, want %s", string(colorPrimary), string(light.Primary))
	}
	if colorBg != light.Bg {
		t.Errorf("colorBg not updated: got %s, want %s", string(colorBg), string(light.Bg))
	}
	if colorText != light.Text {
		t.Errorf("colorText not updated: got %s, want %s", string(colorText), string(light.Text))
	}
	if colorSuccess != light.Success {
		t.Errorf("colorSuccess not updated: got %s, want %s", string(colorSuccess), string(light.Success))
	}
	if colorDanger != light.Danger {
		t.Errorf("colorDanger not updated: got %s, want %s", string(colorDanger), string(light.Danger))
	}
	if colorWhite != light.Text {
		t.Errorf("colorWhite not updated: got %s, want %s", string(colorWhite), string(light.Text))
	}

	// Restore dark theme
	ApplyTheme(DarkTheme())
}
