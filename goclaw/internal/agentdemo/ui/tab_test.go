package ui

import "testing"

func TestTabExpand(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"de", "demo-tool", true},
		{"hel", "help", true},
		{"demo-tool", "", false},
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
