package config

import "testing"

func TestNormalizeTUIInteractMode(t *testing.T) {
	t.Parallel()
	if g := NormalizeTUIInteractMode(""); g != TUIInteractModeChat {
		t.Fatalf("%q", g)
	}
	if g := NormalizeTUIInteractMode("CODE"); g != TUIInteractModeCode {
		t.Fatalf("%q", g)
	}
	if g := NormalizeTUIInteractMode("nope"); g != TUIInteractModeChat {
		t.Fatalf("%q", g)
	}
}

func TestCycleTUIInteractMode(t *testing.T) {
	t.Parallel()
	if g := CycleTUIInteractMode(TUIInteractModeChat); g != TUIInteractModeCode {
		t.Fatalf("%q", g)
	}
	if g := CycleTUIInteractMode(TUIInteractModeCode); g != TUIInteractModeAgent {
		t.Fatalf("%q", g)
	}
	if g := CycleTUIInteractMode(TUIInteractModeAgent); g != TUIInteractModeChat {
		t.Fatalf("%q", g)
	}
}
