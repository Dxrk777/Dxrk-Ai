// SPDX-License-Identifier: MIT
package styles

import "testing"

func TestTagline(t *testing.T) {
	got := Tagline("1.0.0")
	want := "Dxrk 1.0.0 — Ecosystem, Frameworks, Workflows"
	if got != want {
		t.Errorf("Tagline(\"1.0.0\") = %q, want %q", got, want)
	}
}

func TestTagline_EmptyVersion(t *testing.T) {
	got := Tagline("")
	if got == "" {
		t.Error("Tagline(\"\") returned empty string")
	}
}

func TestRenderLogo_NotEmpty(t *testing.T) {
	got := RenderLogo()
	if got == "" {
		t.Error("RenderLogo() returned empty string")
	}
}

func TestRenderLogo_ContainsRoseLines(t *testing.T) {
	got := RenderLogo()
	if len(got) < 100 {
		t.Errorf("RenderLogo() output too short (%d bytes), expected substantial ASCII art", len(got))
	}
}

func TestRenderLogo_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RenderLogo() panicked: %v", r)
		}
	}()
	_ = RenderLogo()
}

func TestCursorConstant(t *testing.T) {
	if Cursor != "▸ " {
		t.Errorf("Cursor = %q, want \"▸ \"", Cursor)
	}
}

func TestStyleRenders_NoPanic(t *testing.T) {
	tests := []struct {
		name  string
		style string
		value string
	}{
		{"TitleStyle", TitleStyle.Render("title"), "title"},
		{"HeadingStyle", HeadingStyle.Render("heading"), "heading"},
		{"HelpStyle", HelpStyle.Render("help"), "help"},
		{"SubtextStyle", SubtextStyle.Render("sub"), "sub"},
		{"SelectedStyle", SelectedStyle.Render("selected"), "selected"},
		{"UnselectedStyle", UnselectedStyle.Render("unselected"), "unselected"},
		{"SuccessStyle", SuccessStyle.Render("success"), "success"},
		{"ErrorStyle", ErrorStyle.Render("error"), "error"},
		{"WarningStyle", WarningStyle.Render("warning"), "warning"},
		{"FrameStyle", FrameStyle.Render("frame"), "frame"},
		{"PanelStyle", PanelStyle.Render("panel"), "panel"},
		{"ProgressFilled", ProgressFilled.Render("filled"), "filled"},
		{"ProgressEmpty", ProgressEmpty.Render("empty"), "empty"},
		{"PercentStyle", PercentStyle.Render("100%"), "100%"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.style == "" {
				t.Errorf("%s.Render() returned empty string", tt.name)
			}
		})
	}
}
