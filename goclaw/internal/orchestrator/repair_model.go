package orchestrator

import "strings"

// applyRepairModelOverride forces the next LLM request in this user turn to use the coding-tier model
// (task_models["code"] when routing is active, otherwise cfg.Model()). Cleared by runUserTurn defer.
func (o *Orchestrator) applyRepairModelOverride() {
	if o == nil {
		return
	}
	if strings.TrimSpace(o.profile.ModelOverride) != "" {
		return
	}
	if o.ut == nil {
		return
	}
	o.ut.taskRole = "code"
	cfg := o.cfg
	if !cfg.TaskModelRoutingActive() {
		o.ut.turnModel = cfg.NormalizeModelForProvider(cfg.Model())
		return
	}
	if m, ok := cfg.TaskModels["code"]; ok && strings.TrimSpace(m) != "" {
		o.ut.turnModel = cfg.NormalizeModelForProvider(m)
		return
	}
	if m, ok := cfg.TaskModels["default"]; ok && strings.TrimSpace(m) != "" {
		o.ut.turnModel = cfg.NormalizeModelForProvider(m)
		return
	}
	o.ut.turnModel = cfg.NormalizeModelForProvider(cfg.Model())
}

// turnModelIsLightweightForRepair reports whether the active turn model matches explore/default/fast tier
// or the compaction model.
func (o *Orchestrator) turnModelIsLightweightForRepair() bool {
	if o == nil {
		return false
	}
	if o.ut == nil {
		return false
	}
	cur := strings.TrimSpace(o.ut.turnModel)
	if cur == "" {
		return true
	}
	cfg := o.cfg
	if cfg.NormalizeModelForProvider(cur) == cfg.NormalizeModelForProvider(cfg.ModelForCompaction()) {
		return true
	}
	for _, key := range []string{"explore", "default", "fast"} {
		if m, ok := cfg.TaskModels[key]; ok && cfg.NormalizeModelForProvider(cur) == cfg.NormalizeModelForProvider(m) {
			return true
		}
	}
	return false
}

// tryActionRepairEscalation returns true when the orchestrator should inject one repair user line and
// continue the loop with applyRepairModelOverride already applied.
func (o *Orchestrator) tryActionRepairEscalation(
	userMessage string,
	toolCalls int,
	hadToolRound bool,
	lastBatchReadOnly bool,
	actionNudges int,
	repairEscalations *int,
) bool {
	if o == nil || repairEscalations == nil || *repairEscalations > 0 {
		return false
	}
	if o.usesBuildLiteRuntime() {
		return false
	}
	if !o.cfg.ActionRepairEscalation || !o.cfg.AutoContinueActionRequests {
		return false
	}
	if o.profile.ReadOnly || !toolSpecsAllowWorkspaceWrite(o.effectiveToolSpecs()) {
		return false
	}
	if !userMessageWantsWorkspaceWrites(userMessage) {
		return false
	}
	if actionNudges < o.effectiveMaxActionNudges() {
		return false
	}
	if hadToolRound && !lastBatchReadOnly && toolCalls > 0 {
		return false
	}
	mainModel := o.cfg.NormalizeModelForProvider(o.cfg.Model())
	if o.ut == nil {
		return false
	}
	if strings.TrimSpace(o.ut.turnModel) == mainModel && !o.turnModelIsLightweightForRepair() {
		// Already on the primary model id and not a known light tier — avoid a useless loop.
		return false
	}
	*repairEscalations++
	o.applyRepairModelOverride()
	return true
}
