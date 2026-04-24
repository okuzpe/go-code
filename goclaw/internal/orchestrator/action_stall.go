package orchestrator

import (
	"errors"
	"fmt"
	"strings"

	"github.com/okuzpe/goclaw/internal/text"
)

// ErrActionStalled is returned when a coding-intent turn exhausts its recovery ladder
// without producing the real tool-driven action the user asked for.
var ErrActionStalled = errors.New("orchestrator: action stalled")

// ActionStalledError explains why a coding-intent turn failed loudly instead of ending
// as a normal assistant completion.
type ActionStalledError struct {
	Reason       string
	LastResponse string
}

func (e *ActionStalledError) Error() string {
	if e == nil {
		return ErrActionStalled.Error()
	}
	if strings.TrimSpace(e.Reason) == "" {
		return ErrActionStalled.Error()
	}
	if excerpt := actionStallExcerpt(e.LastResponse); excerpt != "" {
		return fmt.Sprintf("%s: %s (last response: %q)", ErrActionStalled.Error(), e.Reason, excerpt)
	}
	return fmt.Sprintf("%s: %s", ErrActionStalled.Error(), e.Reason)
}

func (e *ActionStalledError) Unwrap() error { return ErrActionStalled }

func newActionStalledError(reason, response string) error {
	return &ActionStalledError{
		Reason:       strings.TrimSpace(reason),
		LastResponse: strings.TrimSpace(response),
	}
}

func shouldFailPendingEditRecovery(ut *userTurnState, workspaceWriteOK bool) (string, bool) {
	if ut == nil || !ut.editFileRecoveryPending || workspaceWriteOK {
		return "", false
	}
	return "edit_file recovery was pending after old_string not found, but the model stopped without rereading the file and retrying the edit", true
}

func actionStallExcerpt(response string) string {
	response = strings.Join(strings.Fields(strings.TrimSpace(response)), " ")
	if response == "" {
		return ""
	}
	return text.TruncateRunes(response, 96)
}

func (o *Orchestrator) shouldFailActionStalled(
	response string,
	userMessage string,
	toolCalls int,
	hadToolRound bool,
	workspaceWriteOK bool,
	actionNudges int,
	repairEscalations int,
) (string, bool) {
	if o == nil || o.profile.ReadOnly || workspaceWriteOK {
		return "", false
	}
	if !userMessageWantsWorkspaceWrites(userMessage) {
		return "", false
	}
	if !toolSpecsAllowWorkspaceWrite(o.effectiveToolSpecs()) {
		return "", false
	}

	recoveryExhausted := actionNudges >= o.effectiveMaxActionNudges() &&
		(repairEscalations > 0 || !o.cfg.ActionRepairEscalation)
	if !recoveryExhausted {
		return "", false
	}

	if reason, bad := nonActionCompletionReason(response); bad {
		return reason, true
	}
	if toolCalls == 0 {
		return "no native tool calls were made after recovery", true
	}
	if hadToolRound && !workspaceWriteOK && responseAppearsComplete(response) {
		return "response sounded complete even though no workspace edits were made", true
	}
	return "", false
}

func nonActionCompletionReason(response string) (string, bool) {
	trimmed := strings.TrimSpace(response)
	switch {
	case trimmed == "":
		return "empty or whitespace-only response", true
	case looksLikeFenceOnlyJunk(trimmed):
		return "fence-only or placeholder response", true
	case containsFakeToolNarration(trimmed):
		return "fake tool narration without native tool calls", true
	case containsToolAccessMetaReply(trimmed):
		return "meta reply claiming tool access is unavailable", true
	case containsFutureActionNarration(trimmed):
		return "future-tense narration about next steps instead of taking action now", true
	default:
		return "", false
	}
}

func looksLikeFenceOnlyJunk(response string) bool {
	trimmed := strings.TrimSpace(response)
	if trimmed == "" {
		return true
	}
	replacer := strings.NewReplacer(
		"```", "",
		"`", "",
		"[assistant]", "",
		"[assistant tool_use", "",
		"]", "",
		"json", "",
		"tool", "",
		"tool_use", "",
	)
	cleaned := replacer.Replace(strings.ToLower(trimmed))
	cleaned = strings.Join(strings.Fields(cleaned), "")
	return cleaned == ""
}

func containsFakeToolNarration(response string) bool {
	lower := strings.ToLower(response)
	markers := []string{
		"tool call",
		"[assistant tool_use",
		"<function_calls>",
		"</function_calls>",
		`{"action":"`,
		`{"tool_name":"`,
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func containsToolAccessMetaReply(response string) bool {
	lower := strings.ToLower(strings.Join(strings.Fields(response), " "))
	phrases := []string{
		"i don't have access to your terminal",
		"i dont have access to your terminal",
		"i don't have access to your code execution environment",
		"i dont have access to your code execution environment",
		"as an ai language model, i don't have access",
		"as an ai language model, i dont have access",
		"please provide more details on what specific changes are required",
	}
	for _, phrase := range phrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

func containsFutureActionNarration(response string) bool {
	lower := strings.ToLower(strings.Join(strings.Fields(response), " "))
	phrases := []string{
		"i will continue reading",
		"i'll continue reading",
		"i will keep reading",
		"i'll keep reading",
		"i will read more files",
		"i'll read more files",
		"i need to read additional files",
		"i need to read more files",
		"continuare leyendo",
		"continuaré leyendo",
		"seguiré leyendo",
		"voy a seguir leyendo",
		"voy a leer más archivos",
		"necesito leer más archivos",
		"continuare revisando",
		"continuaré revisando",
	}
	for _, phrase := range phrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}
