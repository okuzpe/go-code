package slashcmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/coordinator"
	"github.com/okuzpe/goclaw/internal/memory"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/session"
)

func slashNextStepError(cause, nextStep string) error {
	cause = strings.TrimSpace(cause)
	nextStep = strings.TrimSpace(nextStep)
	if nextStep == "" {
		return errors.New(cause)
	}
	return fmt.Errorf("%s\nnext step: %s", cause, nextStep)
}

func isHelpAlias(input string) bool {
	low := strings.ToLower(strings.TrimSpace(input))
	return low == "help" || low == "?"
}

// normalizeCommandPrefix converts a leading ':' to '/' so ':cmd' is treated identically to '/cmd'.
func normalizeCommandPrefix(input string) string {
	trimmed := strings.TrimSpace(input)
	if strings.HasPrefix(trimmed, ":") && !strings.HasPrefix(trimmed, "::") {
		return "/" + trimmed[1:]
	}
	return input
}

func parseSlashCommand(input string) (fields []string, cmd string, ok bool) {
	trimmed := strings.TrimSpace(normalizeCommandPrefix(input))
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		return nil, "", false
	}
	fields = strings.Fields(trimmed)
	if len(fields) == 0 {
		return nil, "", false
	}
	return fields, strings.ToLower(strings.TrimPrefix(fields[0], "/")), true
}

func requireRunningAgent(command string, orch *orchestrator.Orchestrator) error {
	if orch != nil {
		return nil
	}
	return fmt.Errorf("/%s requires a running agent", strings.TrimPrefix(strings.TrimSpace(command), "/"))
}

func requireSessionStore(command string, store *session.Store) error {
	if store != nil {
		return nil
	}
	return fmt.Errorf("/%s: no session store configured", strings.TrimPrefix(strings.TrimSpace(command), "/"))
}

func requireActiveSession(command string, sess **session.Session) error {
	if sess != nil && *sess != nil {
		return nil
	}
	return fmt.Errorf("/%s: no active session", strings.TrimPrefix(strings.TrimSpace(command), "/"))
}

func requireFocusRouter(command string, env SlashEnv) error {
	if env.Focus != nil {
		return nil
	}
	return fmt.Errorf("focus routing not enabled (/%s)", strings.TrimPrefix(strings.TrimSpace(command), "/"))
}

// PlanGateConfig mirrors plan workflow flags from runtime settings (see config.Config).
type PlanGateConfig struct {
	RequireApplyApproval bool
	ApplyUseCoordinator  bool
	AgentPickerHide      []string
}

// SlashEnv carries workspace paths and profile lookup for slash commands.
type SlashEnv struct {
	Workdir       string
	UserConfigDir string // ~/.goclaw — for /theme merge-write
	// DisableInteractiveThemePick skips arrow-key /theme picker in plain REPL (e.g. when stdin is not a TTY).
	DisableInteractiveThemePick bool
	// DisableInteractiveAgentPick skips arrow-key /agents picker (fullscreen TUI uses its own overlay).
	DisableInteractiveAgentPick bool
	Profs                       map[string]agents.Profile
	UserAgentsDir               string // for hot-reload of custom profiles on /profile
	ProjectAgentsDir            string
	Doctor                      func(ctx context.Context) (string, error)
	// Focus is optional; when set, /focus (/in) and /detach (/back, /hub) route input to interactive workers.
	Focus *coordinator.FocusRouter
	// ChatSubtitle optional; after profile switches returns window subtitle (e.g. provider · model · profile).
	ChatSubtitle func() string
	// SessionModel returns the process default model id for the active provider (optional; for /model).
	SessionModel func() string
	// SetSessionModel updates the in-process default model id where supported (optional; for /model).
	SetSessionModel func(id string) error
	// ToolLog optional; when set, /tools prints formatted tool history as plain text. Fullscreen TUI
	// normally uses Ctrl+T instead; HandleSlash returns a hint when ToolLog is nil.
	ToolLog func(n int) string
	// PlanGate optional; supplies plan approval / hub / agent-picker flags from settings. Nil → all off.
	PlanGate func() PlanGateConfig
	// FullscreenTUI is true when slash commands run inside the Bubble Tea chat (cmd/goclaw fullscreen runner).
	// Used to avoid raw stdout control sequences that would fight the TUI (e.g. /clear).
	FullscreenTUI bool
	// OnProfileChange optional; invoked after a successful /profile or /agents hot-switch so the host can
	// mirror agents.Profile (e.g. ChatRuntime.Profile for doctor and auto-profile helpers).
	OnProfileChange func(agents.Profile)
}

// SlashContext carries dependencies for HandleSlash (memory, orchestrator, session pointer, disk store).
type SlashContext struct {
	SlashEnv
	Mem   *memory.Store
	Orch  *orchestrator.Orchestrator
	Sess  **session.Session
	Store *session.Store
}

// ErrReplQuit is returned by HandleSlash for /quit and /exit so the REPL can save and exit cleanly.
var ErrReplQuit = errors.New("repl quit")

// HandleSlash processes REPL slash commands. Returns handled=true if input was consumed.
// modelSubmit is non-empty when the caller should send that text to the model (e.g. /edit).
// quit with ErrReplQuit means the REPL should exit after printing out.
// hintsOut when non-nil receives TUI refresh hints (welcome bar, transcript rebuild); pass nil for non-TUI callers.
func HandleSlash(ctx context.Context, sc SlashContext, input string, hintsOut *UIHints) (handled bool, out string, quit bool, modelSubmit string, err error) {
	clearHints(hintsOut)

	s := strings.TrimSpace(normalizeCommandPrefix(input))
	if s == "" {
		return false, "", false, "", nil
	}
	if isHelpAlias(s) {
		return true, replHelpText(sc.SlashEnv, sc.Sess, sc.Orch), false, "", nil
	}
	fields, cmd, ok := parseSlashCommand(s)
	if !ok {
		return false, "", false, "", nil
	}

	switch cmd {
	case "help":
		return true, replHelpText(sc.SlashEnv, sc.Sess, sc.Orch), false, "", nil
	case "btw":
		return handleSlashBTW(sc.Orch, fields)
	case "doctor":
		return handleSlashDoctor(ctx, sc.SlashEnv, hintsOut)
	case "model":
		return handleSlashModel(sc.SlashEnv, sc.Orch, fields, hintsOut)
	case "tools":
		return handleSlashTools(sc.SlashEnv, fields)
	case "quit", "exit":
		return true, "bye", true, "", ErrReplQuit
	case "sessions":
		return handleSlashSessions(sc.Store, hintsOut)
	case "clear":
		return handleSlashClear(sc.SlashEnv, hintsOut)
	case "resume":
		return handleSlashResume(sc.Orch, sc.Sess, sc.Store, fields, hintsOut)
	case "new":
		return handleSlashNew(sc.Orch, sc.Sess, sc.Store)
	case "save":
		return handleSlashSave(sc.Sess, sc.Store)
	case "session":
		return handleSlashSession(sc.Sess)
	case "theme":
		return handleSlashTheme(sc.SlashEnv, fields)
	case "capabilities":
		setTUIDocOverlay(hintsOut, "Capabilities")
		return true, UserCapabilitiesGuide(), false, "", nil
	case "copy":
		return handleSlashCopy(sc.Sess)
	case "export":
		return handleSlashExport(sc.SlashEnv, sc.Sess, fields)
	case "compact":
		return handleSlashCompact(sc.Orch, sc.Sess)
	case "continue":
		return handleSlashContinue(sc.Orch, sc.Sess)
	case "undo":
		return handleSlashUndo(sc.Orch, hintsOut)
	case "edit":
		return handleSlashEdit(ctx, sc.SlashEnv, sc.Orch)
	case "init":
		return handleSlashInit(sc.SlashEnv, hintsOut)
	case "memory":
		return handleSlashMemory(sc.Mem, fields, hintsOut)
	case "profile":
		return handleSlashProfile(sc.SlashEnv, sc.Orch, fields, hintsOut)
	case "mode":
		return handleSlashMode(sc.SlashEnv, sc.Orch, fields, hintsOut)
	case "agents":
		return handleSlashAgents(sc.SlashEnv, sc.Orch, fields, hintsOut)
	case "allow-writes":
		return handleSlashAllowWrites(sc.Orch, hintsOut)
	case "plan":
		return handleSlashPlan(sc, fields, hintsOut)
	case "workers":
		return handleSlashWorkers()
	case "detach", "back", "parent", "hub":
		return handleSlashDetach(sc.SlashEnv)
	case "focus", "in":
		return handleSlashFocus(sc.SlashEnv, fields)
	case "apply-plan":
		return handleSlashApplyPlan(s, sc.SlashEnv, sc.Orch, hintsOut)
	case "audit":
		return handleSlashAudit(sc.SlashEnv, sc.Orch, fields, hintsOut)
	case "review":
		return handleSlashReview(ctx, sc.SlashEnv, sc.Orch, fields, hintsOut)
	case "research":
		return handleSlashResearch(sc.Orch, fields)
	default:
		return true, "", false, "", slashNextStepError(fmt.Sprintf("unknown command /%s", cmd), "run /help for the full command list")
	}
}
