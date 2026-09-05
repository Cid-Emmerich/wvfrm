// Package theme defines colour themes, including the "match" theme that is
// generated from the current album art.
package theme

import (
	"strings"

	"github.com/Cid-Emmerich/wvfrm/internal/art"
)

// Theme is a small palette used everywhere in the UI. Terminal background is
// left untouched so wvfrm blends into whatever the terminal already uses.
type Theme struct {
	Name      string
	Text      art.RGB // primary text
	Muted     art.RGB // secondary text, borders
	Accent    art.RGB // highlights, progress bar, active items
	Secondary art.RGB // gradient partner for the accent
	Tertiary  art.RGB // third gradient stop
	Select    art.RGB // selection background in lists
	Warn      art.RGB
}

func rgb(hex uint32) art.RGB {
	return art.RGB{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex)}
}

// Builtin themes, in cycling order. "match" is appended dynamically.
var Builtin = []Theme{
	{Name: "wvfrm", Text: rgb(0xE6E6F0), Muted: rgb(0x6C6F85), Accent: rgb(0x7AA2F7), Secondary: rgb(0xBB9AF7), Tertiary: rgb(0x7DCFFF), Select: rgb(0x2A2E45), Warn: rgb(0xF7768E)},
	{Name: "nord", Text: rgb(0xECEFF4), Muted: rgb(0x4C566A), Accent: rgb(0x88C0D0), Secondary: rgb(0x81A1C1), Tertiary: rgb(0xB48EAD), Select: rgb(0x3B4252), Warn: rgb(0xBF616A)},
	{Name: "dracula", Text: rgb(0xF8F8F2), Muted: rgb(0x6272A4), Accent: rgb(0xBD93F9), Secondary: rgb(0xFF79C6), Tertiary: rgb(0x8BE9FD), Select: rgb(0x44475A), Warn: rgb(0xFF5555)},
	{Name: "gruvbox", Text: rgb(0xEBDBB2), Muted: rgb(0x928374), Accent: rgb(0xFABD2F), Secondary: rgb(0xFE8019), Tertiary: rgb(0xB8BB26), Select: rgb(0x3C3836), Warn: rgb(0xFB4934)},
	{Name: "catppuccin", Text: rgb(0xCDD6F4), Muted: rgb(0x6C7086), Accent: rgb(0xCBA6F7), Secondary: rgb(0xF5C2E7), Tertiary: rgb(0x89DCEB), Select: rgb(0x313244), Warn: rgb(0xF38BA8)},
	{Name: "solarized", Text: rgb(0xEEE8D5), Muted: rgb(0x586E75), Accent: rgb(0x2AA198), Secondary: rgb(0x268BD2), Tertiary: rgb(0xB58900), Select: rgb(0x073642), Warn: rgb(0xDC322F)},
	{Name: "synthwave", Text: rgb(0xF4EEFF), Muted: rgb(0x7A5C99), Accent: rgb(0xFF2ED2), Secondary: rgb(0x2DE2E6), Tertiary: rgb(0xF6F740), Select: rgb(0x3A1F5D), Warn: rgb(0xFF6E6E)},
	{Name: "sunset", Text: rgb(0xFFF1E6), Muted: rgb(0x8C6A5D), Accent: rgb(0xFF7B54), Secondary: rgb(0xFFB26B), Tertiary: rgb(0xFFD56F), Select: rgb(0x4A2B2B), Warn: rgb(0xFF4C4C)},
	{Name: "forest", Text: rgb(0xE3EBD9), Muted: rgb(0x6B7F62), Accent: rgb(0x8FD694), Secondary: rgb(0x4FA37A), Tertiary: rgb(0xC9E4A6), Select: rgb(0x2C3A2A), Warn: rgb(0xE07A5F)},
	{Name: "ocean", Text: rgb(0xE0F2FE), Muted: rgb(0x557A95), Accent: rgb(0x38BDF8), Secondary: rgb(0x818CF8), Tertiary: rgb(0x34D399), Select: rgb(0x1E3A5F), Warn: rgb(0xFB7185)},
	{Name: "mono", Text: rgb(0xF0F0F0), Muted: rgb(0x707070), Accent: rgb(0xFFFFFF), Secondary: rgb(0xBDBDBD), Tertiary: rgb(0x8A8A8A), Select: rgb(0x333333), Warn: rgb(0xFFFFFF)},
	{Name: "amber", Text: rgb(0xFFE7B3), Muted: rgb(0x8A6A2E), Accent: rgb(0xFFB000), Secondary: rgb(0xFF8C00), Tertiary: rgb(0xFFD866), Select: rgb(0x3A2A0A), Warn: rgb(0xFF5A36)},
}

// Names lists every theme name including "match".
func Names() []string {
	out := make([]string, 0, len(Builtin)+1)
	for _, t := range Builtin {
		out = append(out, t.Name)
	}
	return append(out, "match")
}

// Get returns a builtin theme by name (default theme when unknown).
func Get(name string) Theme {
	name = strings.ToLower(name)
	for _, t := range Builtin {
		if t.Name == name {
			return t
		}
	}
	return Builtin[0]
}

// FromPalette builds a "match" theme from album-art colours. base supplies
// the text/selection colours so readability never depends on the cover.
func FromPalette(p art.Palette, base Theme) Theme {
	t := base
	t.Name = "match"
	if len(p.Dominant) == 0 {
		return t
	}
	t.Accent = p.Accent
	t.Secondary = p.Secondary
	t.Tertiary = art.Mix(p.Accent, p.Light, 0.5)
	t.Muted = p.Dim
	t.Select = art.Mix(p.Dark, art.RGB{R: 40, G: 40, B: 48}, 0.6)
	// Keep the text readable: if the cover is very bright, darken nothing
	// (terminal bg is unknown) but make sure muted is not too dark.
	if art.Luminance(t.Muted) < 0.25 {
		t.Muted = art.Mix(t.Muted, art.RGB{R: 160, G: 160, B: 170}, 0.5)
	}
	return t
}
