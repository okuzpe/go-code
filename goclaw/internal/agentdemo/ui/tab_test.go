package ui

import "testing"

func TestTabExpand(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"hel", "help", true},
		{"cl", "clear", true},
		{"q", "quit", true},
		{"help", "", false}, // exact match — no expand
		{"", "", false},
		{"xyz", "", false},
	}
	for _, tc := range cases {
		got, ok := TabExpand(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("TabExpand(%q) = (%q,%v) want (%q,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
