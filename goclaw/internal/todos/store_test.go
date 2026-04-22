package todos

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestStoreReplaceAndMerge(t *testing.T) {
	s := NewStore()
	in1 := `{"merge":false,"todos":[{"id":"a","content":"one","status":"pending"}]}`
	if err := s.Apply(in1); err != nil {
		t.Fatal(err)
	}
	want1 := "- [pending] a: one\n\n**Current task:** a: one\n**Next task:** (none queued after current — complete work or update todo_write.)"
	if got := s.FormatForPrompt(); got != want1 {
		t.Fatalf("prompt: %q", got)
	}
	in2 := `{"merge":true,"todos":[{"id":"a","content":"one done","status":"completed"},{"id":"b","content":"two","status":"in_progress"}]}`
	if err := s.Apply(in2); err != nil {
		t.Fatal(err)
	}
	out := s.FormatForPrompt()
	if out == "" {
		t.Fatal("empty after merge")
	}
}

func TestStoreValidation(t *testing.T) {
	s := NewStore()
	if err := s.Apply(`{"merge":false,"todos":[]}`); err == nil {
		t.Fatal("expected error for empty todos")
	}
	if err := s.Apply(`{"merge":false,"todos":[{"id":"","content":"x","status":"pending"}]}`); err == nil {
		t.Fatal("expected error for empty id")
	}
	if err := s.Apply(`{"merge":false,"todos":[{"id":"x","content":"y","status":"bogus"}]}`); err == nil {
		t.Fatal("expected error for bad status")
	}
}

func TestStoreApplyRejectsMoreThanMaxItems(t *testing.T) {
	s := NewStore()
	todos := make([]map[string]string, 0, MaxItems+1)
	for i := range MaxItems + 1 {
		todos = append(todos, map[string]string{
			"id":      fmt.Sprintf("t%d", i),
			"content": "x",
			"status":  "pending",
		})
	}
	raw, jerr := json.Marshal(map[string]any{"merge": false, "todos": todos})
	if jerr != nil {
		t.Fatal(jerr)
	}
	err := s.Apply(string(raw))
	if err == nil {
		t.Fatal("expected error when todos count exceeds MaxItems")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("%d", MaxItems)) {
		t.Fatalf("error should mention limit: %v", err)
	}
}

func TestStoreApplyRejectsContentOverMaxRunes(t *testing.T) {
	s := NewStore()
	long := strings.Repeat("a", MaxContentRunes+1)
	raw := fmt.Sprintf(`{"merge":false,"todos":[{"id":"one","content":%q,"status":"pending"}]}`, long)
	err := s.Apply(raw)
	if err == nil {
		t.Fatal("expected error when content exceeds MaxContentRunes")
	}
	if !strings.Contains(err.Error(), "exceeds") || !strings.Contains(err.Error(), "runes") {
		t.Fatalf("error should mention rune limit: %v", err)
	}
}
