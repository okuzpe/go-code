package config

import "testing"

func TestNormalizeUIAppearance(t *testing.T) {
	if got := NormalizeUIAppearance(""); got != UIAppearanceAuto {
		t.Fatalf("empty: got %q", got)
	}
	if got := NormalizeUIAppearance("DARK"); got != UIAppearanceDark {
		t.Fatalf("dark: got %q", got)
	}
	if got := NormalizeUIAppearance("nope"); got != UIAppearanceAuto {
		t.Fatalf("unknown: got %q", got)
	}
}
