package permissions_test

import (
	"testing"

	"github.com/okuzpe/goclaw/internal/permissions"
)

func TestParseMode(t *testing.T) {
	tests := []struct {
		in   string
		want permissions.Mode
	}{
		{"allow", permissions.ModeAllow},
		{"ASK", permissions.ModeAsk},
		{"  deny ", permissions.ModeDeny},
	}
	for _, tt := range tests {
		got, err := permissions.ParseMode(tt.in)
		if err != nil {
			t.Fatalf("ParseMode(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("ParseMode(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
	if _, err := permissions.ParseMode("nope"); err == nil {
		t.Fatal("expected error for invalid mode")
	}
}
