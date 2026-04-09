package slashcmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
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

func testSlashCtx(t *testing.T, mem *memory.Store, orch *orchestrator.Orchestrator, sp **session.Session, store *session.Store) SlashContext {
	t.Helper()
	return SlashContext{SlashEnv: testSlashEnv(t), Mem: mem, Orch: orch, Sess: sp, Store: store}
}

func TestHandleSlashHelp(t *testing.T) {
	s := &session.Session{ID: "abc123"}
	sp := &s
	handled, out, _, _, err := HandleSlash(context.Background(), testSlashCtx(t, memory.New(t.TempDir()), nil, sp, nil), "/help")
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
	handled, out, quit, ms, err := HandleSlash(context.Background(), testSlashCtx(t, memory.New(t.TempDir()), nil, &sp, nil), "/quit")
	if !handled || !quit || !errors.Is(err, ErrReplQuit) || out != "bye" || ms != "" {
		t.Fatalf("quit: handled=%v quit=%v err=%v out=%q ms=%q", handled, quit, err, out, ms)
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
	handled, out, _, _, err := HandleSlash(context.Background(), testSlashCtx(t, memory.New(t.TempDir()), nil, &sp, store), "/sessions")
	if err != nil || !handled || !strings.Contains(out, s.ID) {
		t.Fatalf("sessions: handled=%v err=%v out=%q", handled, err, out)
	}
}

func TestHandleSlashMemoryAddListDelete(t *testing.T) {
	dir := t.TempDir()
	mem := memory.New(dir)
	var sp *session.Session
	handled, out, _, _, err := HandleSlash(context.Background(), testSlashCtx(t, mem, nil, &sp, nil),
		"/memory add user mynote this is the stored body")
	if err != nil || !handled || !strings.Contains(out, "saved memory") {
		t.Fatalf("add: handled=%v err=%v out=%q", handled, err, out)
	}
	handled, out, _, _, err = HandleSlash(context.Background(), testSlashCtx(t, mem, nil, &sp, nil), "/memory list")
	if err != nil || !handled || !strings.Contains(out, "mynote") {
		t.Fatalf("list: out=%q err=%v", out, err)
	}
	listed, err := mem.List()
	if err != nil || len(listed) < 1 {
		t.Fatalf("mem.List: err=%v n=%d", err, len(listed))
	}
	base := listed[0].Filename
	handled, out, _, _, err = HandleSlash(context.Background(), testSlashCtx(t, mem, nil, &sp, nil), "/memory delete "+base)
	if err != nil || !handled || !strings.Contains(out, "deleted") {
		t.Fatalf("delete: handled=%v err=%v out=%q", handled, err, out)
	}
	if _, statErr := os.Stat(filepath.Join(dir, base)); !os.IsNotExist(statErr) {
		t.Fatalf("expected file removed: %v", statErr)
	}
}

func TestHandleSlashCompactRequiresOrchestrator(t *testing.T) {
	var sp *session.Session
	_, _, _, _, err := HandleSlash(context.Background(), testSlashCtx(t, memory.New(t.TempDir()), nil, &sp, nil), "/compact")
	if err == nil {
		t.Fatal("expected error when orchestrator is nil")
	}
}

func TestHandleSlashEditRequiresOrchestrator(t *testing.T) {
	var sp *session.Session
	_, _, _, _, err := HandleSlash(context.Background(), testSlashCtx(t, memory.New(t.TempDir()), nil, &sp, nil), "/edit")
	if err == nil || !strings.Contains(err.Error(), "requires a running agent") {
		t.Fatalf("expected /edit orchestrator error, got %v", err)
	}
}

func TestHandleSlashEditModelSubmit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake editor test")
	}
	wd := t.TempDir()
	script := filepath.Join(wd, "fake-editor.sh")
	body := "#!/bin/sh\nprintf '\\nline from fake editor\\n' >> \"$1\"\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", script)

	s := session.New()
	sp := &s
	orch := orchestrator.New(config.Default(), nil, s, tools.New(), permissions.NewPolicy(), hooks.New(), agents.All()["guide"])
	env := SlashEnv{Workdir: wd, Profs: agents.All()}
	handled, out, quit, ms, err := HandleSlash(context.Background(), SlashContext{SlashEnv: env, Mem: memory.New(t.TempDir()), Orch: orch, Sess: sp, Store: nil}, "/edit")
	if err != nil || !handled || quit {
		t.Fatalf("edit: handled=%v quit=%v err=%v out=%q", handled, quit, err, out)
	}
	if !strings.Contains(ms, "line from fake editor") {
		t.Fatalf("expected model submit body from editor, got %q", ms)
	}
}

func TestHandleSlashPlanPath(t *testing.T) {
	wd := t.TempDir()
	env := SlashEnv{Workdir: wd, Profs: agents.All()}
	var sp *session.Session
	handled, out, _, _, err := HandleSlash(context.Background(), SlashContext{SlashEnv: env, Mem: memory.New(t.TempDir()), Orch: nil, Sess: &sp, Store: nil}, "/plan path")
	if err != nil || !handled || !strings.Contains(out, ".goclaw") || !strings.Contains(out, "plan.md") {
		t.Fatalf("plan path: handled=%v err=%v out=%q", handled, err, out)
	}
}

func TestHandleSlashProfile(t *testing.T) {
	s := session.New()
	sp := &s
	orch := orchestrator.New(config.Default(), nil, s, tools.New(), permissions.NewPolicy(), hooks.New(), agents.All()["plan"])
	env := SlashEnv{Workdir: t.TempDir(), Profs: agents.All()}
	handled, out, _, _, err := HandleSlash(context.Background(), SlashContext{SlashEnv: env, Mem: memory.New(t.TempDir()), Orch: orch, Sess: sp, Store: nil}, "/profile explore")
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

	handled, out, _, _, err := HandleSlash(context.Background(), testSlashCtx(t, memory.New(t.TempDir()), orch, sp, store), "/new")
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
	if _, err := os.Stat(filepath.Join(sessDir, origID+".jsonl")); err != nil {
		t.Fatalf("expected saved jsonl for old session: %v", err)
	}
	// Previous session must be fully flushed before the new empty session is active.
	prev, err := store.Load(origID)
	if err != nil {
		t.Fatalf("load saved session: %v", err)
	}
	if prev == nil || prev.Len() != 1 || prev.Messages[0].Role != "user" || prev.Messages[0].Content != "hello" {
		t.Fatalf("saved transcript before /new: got %+v (len=%d)", prev, prev.Len())
	}
	if (*sp).Len() != 0 {
		t.Fatalf("new session should start empty, got len=%d", (*sp).Len())
	}

	handled, out, _, _, err = HandleSlash(context.Background(), testSlashCtx(t, memory.New(t.TempDir()), orch, sp, store), "/save")
	if err != nil || !handled {
		t.Fatalf("/save: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(out, "session saved") {
		t.Fatalf("unexpected /save output: %q", out)
	}
}
