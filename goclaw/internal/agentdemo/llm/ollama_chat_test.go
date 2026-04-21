package llm

import "testing"

func TestParseChatStreamLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		line    string
		want    string
		wantDone bool
		wantErr bool
	}{
		{
			name: "delta",
			line: `{"model":"m","created_at":"x","message":{"role":"assistant","content":"hi"},"done":false}`,
			want: "hi",
		},
		{
			name:     "done empty",
			line:     `{"done":true}`,
			want:     "",
			wantDone: true,
		},
		{
			name: "empty content",
			line: `{"message":{"role":"assistant","content":""},"done":false}`,
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, done, err := ParseChatStreamLine([]byte(tc.line))
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("content: got %q want %q", got, tc.want)
			}
			if done != tc.wantDone {
				t.Fatalf("done: got %v want %v", done, tc.wantDone)
			}
		})
	}
}
