package orchestrator

import "testing"

func TestRoutingUserMessage(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain message unchanged",
			in:   "fix this bug",
			want: "fix this bug",
		},
		{
			name: "inline context stripped for routing",
			in: `[Files loaded via @ references]

@goclaw
---
package main

[End of @ context]

has creado el plan para las mejoras?`,
			want: "has creado el plan para las mejoras?",
		},
		{
			name: "malformed block preserved",
			in: `[Files loaded via @ references]
missing end marker`,
			want: `[Files loaded via @ references]
missing end marker`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := routingUserMessage(tt.in)
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}
