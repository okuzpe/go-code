package config

import "testing"

func TestNormalizeTUIFooterDensity(t *testing.T) {
	t.Parallel()
	if got := NormalizeTUIFooterDensity(""); got != TUIFooterDensityStandard {
		t.Fatalf("empty: got %q", got)
	}
	if got := NormalizeTUIFooterDensity("MINIMAL"); got != TUIFooterDensityMinimal {
		t.Fatalf("minimal: got %q", got)
	}
	if got := NormalizeTUIFooterDensity("Debug"); got != TUIFooterDensityDebug {
		t.Fatalf("debug: got %q", got)
	}
	if got := NormalizeTUIFooterDensity("nope"); got != TUIFooterDensityStandard {
		t.Fatalf("unknown: got %q", got)
	}
}
