package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/slashcmd"
)

// defaultTUIProfileCycleOrder is used when settings do not set tui_profile_cycle (after filtering
// to profiles that exist in built-in + custom agents).
var defaultTUIProfileCycleOrder = []string{"general-purpose", "plan", "explore", "coordinator"}

// tuiProfileCycleOrder returns distinct lower-case profile keys present in built-in+custom agents.
func tuiProfileCycleOrder(rt *ChatRuntime) []string {
	if rt == nil {
		return nil
	}
	order := defaultTUIProfileCycleOrder
	if len(rt.Cfg.TUIProfileCycle) > 0 {
		order = append([]string(nil), rt.Cfg.TUIProfileCycle...)
	}
	profs, _ := agents.AllWithCustom(rt.UserAgentsDir, rt.ProjectAgentsDir)
	var out []string
	seen := make(map[string]struct{})
	for _, raw := range order {
		k := strings.ToLower(strings.TrimSpace(raw))
		if k == "" {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		if _, ok := profs[k]; !ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	if len(out) == 0 {
		for _, k := range defaultTUIProfileCycleOrder {
			if _, ok := profs[k]; ok {
				out = append(out, k)
			}
		}
	}
	return out
}

func cycleTUIAgentProfile(ctx context.Context, rt *ChatRuntime, orch *orchestrator.Orchestrator, slashEnv *slashcmd.SlashEnv, sess **session.Session, backward bool) (slashcmd.UIHints, error) {
	names := tuiProfileCycleOrder(rt)
	if len(names) == 0 {
		return slashcmd.UIHints{}, fmt.Errorf("no agent profiles available to cycle")
	}
	cur := strings.ToLower(strings.TrimSpace(orch.ProfileName()))
	idx := 0
	for i, n := range names {
		if strings.ToLower(strings.TrimSpace(n)) == cur {
			idx = i
			break
		}
	}
	step := 1
	if backward {
		step = len(names) - 1
	}
	next := names[(idx+step)%len(names)]
	sc := slashcmd.SlashContext{SlashEnv: *slashEnv, Mem: rt.MemStore, Orch: orch, Sess: sess, Store: rt.Store}
	var hi slashcmd.UIHints
	handled, _, _, _, err := slashcmd.HandleSlash(ctx, sc, "/profile "+next, &hi)
	if err != nil {
		return slashcmd.UIHints{}, err
	}
	if !handled {
		return slashcmd.UIHints{}, fmt.Errorf("profile cycle: /profile %q was not handled", next)
	}
	return hi, nil
}
