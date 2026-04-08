package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/memory"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/todos"
	"github.com/okuzpe/goclaw/internal/tools"
)

func testSlashEnv(t *testing.T) SlashEnv {
	t.Helper()
	return SlashEnv{Workdir: t.TempDir(), Profs: agents.All()}
}

func TestHandleSlashHelp(t *testing.T) {
	s := &session.Session{ID: "abc123"}
	sp := &s
	handled, out, err := handleSlash(context.Background(), testSlashEnv(t), "/help", memory.New(t.TempDir()), nil, sp, nil)
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if !strings.Contains(out, "abc123") || !strings.Contains(out, "/memory") || !strings.Contains(out, "/new") ||
		!strings.Contains(out, "/sessions") || !strings.Contains(out, "/quit") ||
		!strings.Contains(out, "/apply-plan") || !strings.Contains(out, "/plan path") ||
		!strings.Contains(out, "sessions list") {
		t.Fatalf("unexpected help: %s", out)
	}
}

func TestHandleSlashQuit(t *testing.T) {
	var sp *session.Session
	handled, out, err := handleSlash(context.Background(), testSlashEnv(t), "/quit", memory.New(t.TempDir()), nil, &sp, nil)
	if !handled || !errors.Is(err, ErrReplQuit) || out != "bye" {
		t.Fatalf("quit: handled=%v err=%v out=%q", handled, err, out)
	}
}

func TestHandleSlashSessions(t *testing.T) {
	sessDir := t.TempDir()
	store, err := session.NewStore(sessDir)
	if err != nil {
		t.Fatal(err)
	}
	s := session.New()
	if err := store.Save(s); err != nil {
		t.Fatal(err)
	}
	var sp *session.Session
	handled, out, err := handleSlash(context.Background(), testSlashEnv(t), "/sessions", memory.New(t.TempDir()), nil, &sp, store)
	if err != nil || !handled || !strings.Contains(out, s.ID) {
		t.Fatalf("sessions: handled=%v err=%v out=%q", handled, err, out)
	}
}

func TestHandleSlashMemoryAddListDelete(t *testing.T) {
	dir := t.TempDir()
	mem := memory.New(dir)
	var sp *session.Session
	handled, out, err := handleSlash(context.Background(), testSlashEnv(t),
		"/memory add user mynote this is the stored body", mem, nil, &sp, nil)
	if err != nil || !handled || !strings.Contains(out, "saved memory") {
		t.Fatalf("add: handled=%v err=%v out=%q", handled, err, out)
	}
	handled, out, err = handleSlash(context.Background(), testSlashEnv(t), "/memory list", mem, nil, &sp, nil)
	if err != nil || !handled || !strings.Contains(out, "mynote") {
		t.Fatalf("list: out=%q err=%v", out, err)
	}
	listed, err := mem.List()
	if err != nil || len(listed) < 1 {
		t.Fatalf("mem.List: err=%v n=%d", err, len(listed))
	}
	base := listed[0].Filename
	handled, out, err = handleSlash(context.Background(), testSlashEnv(t), "/memory delete "+base, mem, nil, &sp, nil)
	if err != nil || !handled || !strings.Contains(out, "deleted") {
		t.Fatalf("delete: handled=%v err=%v out=%q", handled, err, out)
	}
	if _, statErr := os.Stat(filepath.Join(dir, base)); !os.IsNotExist(statErr) {
		t.Fatalf("expected file removed: %v", statErr)
	}
}

func TestHandleSlashCompactRequiresOrchestrator(t *testing.T) {
	var sp *session.Session
	_, _, err := handleSlash(context.Background(), testSlashEnv(t), "/compact", memory.New(t.TempDir()), nil, &sp, nil)
	if err == nil {
		t.Fatal("expected error when orchestrator is nil")
	}
}

func TestHandleSlashPlanPath(t *testing.T) {
	wd := t.TempDir()
	env := SlashEnv{Workdir: wd, Profs: agents.All()}
	var sp *session.Session
	handled, out, err := handleSlash(context.Background(), env, "/plan path", memory.New(t.TempDir()), nil, &sp, nil)
	if err != nil || !handled || !strings.Contains(out, ".goclaw") || !strings.Contains(out, "plan.md") {
		t.Fatalf("plan path: handled=%v err=%v out=%q", handled, err, out)
	}
}

func TestHandleSlashProfile(t *testing.T) {
	s := session.New()
	sp := &s
	orch := orchestrator.New(config.Default(), nil, s, tools.New(), permissions.NewPolicy(), hooks.New(), agents.All()["plan"])
	env := SlashEnv{Workdir: t.TempDir(), Profs: agents.All()}
	handled, out, err := handleSlash(context.Background(), env, "/profile explore", memory.New(t.TempDir()), orch, sp, nil)
	if err != nil || !handled || !strings.Contains(out, "explore") {
		t.Fatalf("profile: handled=%v err=%v out=%q", handled, err, out)
	}
	if orch.ProfileName() != "explore" {
		t.Fatalf("orchestrator profile: %q", orch.ProfileName())
	}
}

func TestHandleSlashNewAndSave(t *testing.T) {
	sessDir := t.TempDir()
	store, err := session.NewStore(sessDir)
	if err != nil {
		t.Fatal(err)
	}
	s := session.New()
	origID := s.ID
	s.Add("user", "hello")
	sp := &s
	profile := agents.All()["general-purpose"]
	td := todos.NewStore()
	if err := td.Apply(`{"merge":false,"todos":[{"id":"t1","content":"task","status":"pending"}]}`); err != nil {
		t.Fatal(err)
	}
	if td.FormatForPrompt() == "" {
		t.Fatal("expected todo in store")
	}
	orch := orchestrator.New(config.Default(), nil, s, tools.New(), permissions.NewPolicy(), hooks.New(), profile,
		orchestrator.WithTodoStore(td))

	handled, out, err := handleSlash(context.Background(), testSlashEnv(t), "/new", memory.New(t.TempDir()), orch, sp, store)
	if err != nil || !handled {
		t.Fatalf("/new: handled=%v err=%v out=%q", handled, err, out)
	}
	if (*sp).ID == origID {
		t.Fatalf("expected new session id, still %s", origID)
	}
	if !strings.Contains(out, "new session id:") {
		t.Fatalf("unexpected /new output: %q", out)
	}
	if td.FormatForPrompt() != "" {
		t.Fatal("expected session todos cleared after /new")
	}
	// Previous session should be on disk.
	if _, err := os.Stat(filepath.Join(sessDir, origID+".jsonl")); err != nil {
		t.Fatalf("expected saved jsonl for old session: %v", err)
	}

	handled, out, err = handleSlash(context.Background(), testSlashEnv(t), "/save", memory.New(t.TempDir()), orch, sp, store)
	if err != nil || !handled {
		t.Fatalf("/save: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(out, "session saved") {
		t.Fatalf("unexpected /save output: %q", out)
	}
}
