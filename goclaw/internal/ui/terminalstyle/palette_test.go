package terminalstyle

import (
	"testing"

	"github.com/okuzpe/goclaw/internal/config"
)

func TestPaletteForAppearance_auto(t *testing.T) {
	p := PaletteForAppearance("")
	if p.Appearance != config.UIAppearanceAuto {
		t.Fatalf("appearance: got %q want %q", p.Appearance, config.UIAppearanceAuto)
	}
	if p.GlamourStyle != "auto" {
		t.Fatalf("glamour: got %q want auto", p.GlamourStyle)
	}
}

func TestPaletteForAppearance_dark(t *testing.T) {
	p := PaletteForAppearance("dark")
	if p.Appearance != config.UIAppearanceDark {
		t.Fatalf("appearance: got %q", p.Appearance)
	}
	if p.GlamourStyle != "dark" {
		t.Fatalf("glamour: got %q want dark", p.GlamourStyle)
	}
}
