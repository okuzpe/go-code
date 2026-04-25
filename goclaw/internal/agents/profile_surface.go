package agents

import (
	"sort"
	"strings"

	"github.com/okuzpe/goclaw/internal/text"
)

const PublicBuildProfileName = "build"

const (
	RuntimeClassBuildLite   = "build-lite"
	RuntimeClassBuilderFull = "builder-full"
	RuntimeClassPlan        = "plan-read-only"
	RuntimeClassExplore     = "explore-read-only"
	RuntimeClassHub         = "hub"
	RuntimeClassReadOnly    = "read-only"
	RuntimeClassCustom      = "custom"
)

// CanonicalProfileName resolves public aliases to the internal built-in profile key used at runtime.
// Unknown names are normalized for map lookup only.
func CanonicalProfileName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return ""
	}
	switch strings.ToLower(name) {
	case "build", "general-purpose", "general", "gp":
		return "general-purpose"
	default:
		return strings.ToLower(name)
	}
}

// DisplayProfileName returns the preferred user-facing name for a profile key.
func DisplayProfileName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return ""
	}
	if CanonicalProfileName(name) == "general-purpose" {
		return PublicBuildProfileName
	}
	return name
}

// DisplayProfile returns a copy with the preferred user-facing Name.
func DisplayProfile(p Profile) Profile {
	q := p
	q.Name = DisplayProfileName(p.Name)
	return q
}

func IsBuildLiteProfileName(raw string) bool {
	return CanonicalProfileName(raw) == "general-purpose"
}

func IsBuildLiteProfile(p Profile) bool {
	return IsBuildLiteProfileName(p.Name)
}

func IsBuilderFullProfileName(raw string) bool {
	return CanonicalProfileName(raw) == "builder"
}

func RuntimeClassName(raw string, readOnly bool) string {
	switch CanonicalProfileName(raw) {
	case "general-purpose":
		return RuntimeClassBuildLite
	case "builder":
		return RuntimeClassBuilderFull
	case "plan":
		return RuntimeClassPlan
	case "explore":
		return RuntimeClassExplore
	case "coordinator":
		return RuntimeClassHub
	default:
		if readOnly {
			return RuntimeClassReadOnly
		}
		return RuntimeClassCustom
	}
}

// PublicModeNames is the short primary-mode surface shown to most users.
func PublicModeNames() []string {
	return []string{PublicBuildProfileName, "plan"}
}

// UserFacingSortedKeys keeps build/plan first, then custom profiles, then advanced built-ins.
func UserFacingSortedKeys(profs map[string]Profile) []string {
	if len(profs) == 0 {
		return nil
	}
	names := make([]string, 0, len(profs))
	for k := range profs {
		names = append(names, k)
	}
	sort.SliceStable(names, func(i, j int) bool {
		ri := userFacingProfileRank(names[i])
		rj := userFacingProfileRank(names[j])
		if ri != rj {
			return ri < rj
		}
		di := strings.ToLower(DisplayProfileName(names[i]))
		dj := strings.ToLower(DisplayProfileName(names[j]))
		if di != dj {
			return di < dj
		}
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	return names
}

func userFacingProfileRank(name string) int {
	switch CanonicalProfileName(name) {
	case "general-purpose":
		return 0
	case "plan":
		return 1
	case "builder":
		return 20
	case "coordinator":
		return 30
	case "explore":
		return 31
	case "verification":
		return 32
	case "code-review":
		return 33
	case "guide":
		return 34
	case "statusline":
		return 35
	default:
		return 10
	}
}

// ProfileListHint is a comma-separated list of profile names for error messages.
func ProfileListHint() string {
	names := make([]string, 0, len(SortedProfileNames()))
	for _, name := range SortedProfileNames() {
		names = append(names, DisplayProfileName(name))
	}
	return strings.Join(names, ", ")
}

// SortedKeys returns profile map keys sorted lexically (built-in + custom merged maps).
func SortedKeys(profs map[string]Profile) []string {
	if len(profs) == 0 {
		return nil
	}
	names := make([]string, 0, len(profs))
	for k := range profs {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// JoinSortedProfileKeys returns SortedKeys joined with ", " for errors and slash-command hints.
func JoinSortedProfileKeys(profs map[string]Profile) string {
	if len(profs) == 0 {
		return ""
	}
	keys := UserFacingSortedKeys(profs)
	names := make([]string, 0, len(keys))
	for _, key := range keys {
		names = append(names, DisplayProfileName(key))
	}
	return strings.Join(names, ", ")
}

// Summary is a single-line description for listings (/agents, docs).
func (p Profile) Summary() string {
	if s := strings.TrimSpace(p.Description); s != "" {
		return text.TruncateRunes(s, 96)
	}
	switch p.Name {
	case "general-purpose":
		return "Primary build mode: direct coding with a tight, deterministic tool set."
	case "builder":
		return "Advanced direct coding: richer context and broader tools."
	case "explore":
		return "Advanced read-only explorer: read, search, web - no writes."
	case "plan":
		return "Primary planning mode: read-only architecture and step-by-step plans."
	case "verification":
		return "Advanced verifier: PASS/FAIL/PARTIAL checks with limited tools."
	case "code-review":
		return "Advanced diff review: no writes; bash for git/vet only."
	case "guide":
		return "Advanced chat-only guide for repo questions."
	case "statusline":
		return "Advanced single-line status output; no tools."
	case "coordinator":
		return "Advanced hub mode: delegates work to workers via spawn_agent."
	default:
		if s := strings.TrimSpace(p.SystemPrompt); s != "" {
			line := s
			if i := strings.IndexByte(line, '\n'); i >= 0 {
				line = strings.TrimSpace(line[:i])
			}
			line = strings.TrimSpace(line)
			if line != "" {
				return text.TruncateRunes(line, 96)
			}
		}
		if p.ReadOnly {
			return "Read-only custom profile."
		}
		return "Custom agent profile."
	}
}

var workspaceWriteTools = []string{"write_file", "write_files", "create_project", "edit_file", "patch"}

// disallowedToolSet is the set of tool names removed after allowlist filtering (orchestrator.buildRequest).
func (p Profile) disallowedToolSet() map[string]struct{} {
	out := make(map[string]struct{})
	for _, n := range p.DisallowedTools {
		n = strings.TrimSpace(n)
		if n != "" {
			out[n] = struct{}{}
		}
	}
	return out
}

// allowlistSet is non-nil only when ToolAllowlist is set; empty slice yields an empty map (no tools).
func (p Profile) allowlistSet() (allow map[string]struct{}, hasAllowlist bool) {
	if p.ToolAllowlist == nil {
		return nil, false
	}
	allow = make(map[string]struct{}, len(p.ToolAllowlist))
	for _, n := range p.ToolAllowlist {
		n = strings.TrimSpace(n)
		if n != "" {
			allow[n] = struct{}{}
		}
	}
	return allow, true
}

// profileToolMatchesAllowlist mirrors orchestrator.toolMatchesAllowlist for static profile analysis.
func profileToolMatchesAllowlist(name string, allow map[string]struct{}) bool {
	if _, ok := allow[name]; ok {
		return true
	}
	for pat := range allow {
		if strings.HasSuffix(pat, "*") && len(pat) > 1 {
			prefix := strings.TrimSuffix(pat, "*")
			if strings.HasPrefix(name, prefix) {
				return true
			}
		}
	}
	return false
}

func (p Profile) anyWriteToolAllowed(denied map[string]struct{}, allow map[string]struct{}, hasAllowlist bool) bool {
	for _, w := range workspaceWriteTools {
		if _, blocked := denied[w]; blocked {
			continue
		}
		if !hasAllowlist {
			return true
		}
		if profileToolMatchesAllowlist(w, allow) {
			return true
		}
	}
	return false
}

// AllowsWorkspaceFileWrites reports whether write_file, edit_file, or patch can appear in the
// tool list sent to the LLM (same rules as orchestrator.buildRequest).
func (p Profile) AllowsWorkspaceFileWrites() bool {
	if p.ReadOnly {
		return false
	}
	denied := p.disallowedToolSet()
	allow, hasAllowlist := p.allowlistSet()
	return p.anyWriteToolAllowed(denied, allow, hasAllowlist)
}

// AllowsSpawnAgentDelegation reports whether spawn_agent can appear on the model-visible tool list
// for this profile (nil allowlist means full registry, which includes spawn_agent on the parent orchestrator).
func (p Profile) AllowsSpawnAgentDelegation() bool {
	allow, hasAllowlist := p.allowlistSet()
	if !hasAllowlist {
		return true
	}
	if len(p.ToolAllowlist) == 0 {
		return false
	}
	denied := p.disallowedToolSet()
	if _, blocked := denied["spawn_agent"]; blocked {
		return false
	}
	return profileToolMatchesAllowlist("spawn_agent", allow)
}
