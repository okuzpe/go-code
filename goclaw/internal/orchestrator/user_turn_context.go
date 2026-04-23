package orchestrator

import "sync"

// userTurnState holds fields scoped to a single user turn (one call to runUserTurn).
// It is allocated at the start of runUserTurn and cleared in defer.
type userTurnState struct {
	// turnModel is the resolved model id for the current user turn (tool iterations reuse it).
	turnModel string
	// taskRole is the classified task role (e.g. "code", "explore") for buildRequest hints.
	taskRole string

	budgetIter           int
	budgetLimit          int
	budgetToolCalls      int
	turnUserMessage      string
	turnHadToolRound     bool
	turnWorkspaceWriteOK bool

	turnToolCache map[string]string
	cacheMu       sync.RWMutex

	turnInputLang string

	tuiInteractApply bool
	tuiInteractMode  string

	// Post-write verify gate (opt-in via config): after workspace writes, require a verify tool.
	verifyPending   bool
	verifySatisfied bool
	verifyNudges    int

	// revealedToolNames tracks hidden tools (e.g. MCP) revealed via tool_search during this turn.
	// Revealed tools are included in effectiveToolSpecs for subsequent LLM iterations.
	revealedToolNames map[string]bool

	// sysPrefixMemo caches the expensive static portion of the system prompt for this turn only.
	sysPrefixMemo string
}

func (u *userTurnState) toolCacheGet(key string) (string, bool) {
	if u == nil || u.turnToolCache == nil {
		return "", false
	}
	u.cacheMu.RLock()
	defer u.cacheMu.RUnlock()
	v, ok := u.turnToolCache[key]
	return v, ok
}

func (u *userTurnState) toolCacheSet(key, value string) {
	if u == nil || u.turnToolCache == nil {
		return
	}
	u.cacheMu.Lock()
	defer u.cacheMu.Unlock()
	if len(u.turnToolCache) < turnToolCacheMaxEntries {
		u.turnToolCache[key] = value
	}
}
