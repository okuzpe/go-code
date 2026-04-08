package todos

import (
	"testing"
)

func TestStoreReplaceAndMerge(t *testing.T) {
	s := NewStore()
	in1 := `{"merge":false,"todos":[{"id":"a","content":"one","status":"pending"}]}`
	if err := s.Apply(in1); err != nil {
		t.Fatal(err)
	}
	if got := s.FormatForPrompt(); got != "- [pending] a: one" {
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
