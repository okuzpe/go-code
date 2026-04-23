package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const defaultToolSearchLimit = 8

// ToolSearchTool lets the model discover registered tools without paying the
// prompt cost of exposing every MCP tool on every turn.
type ToolSearchTool struct {
	reg *Registry
}

var _ Tool = (*ToolSearchTool)(nil)

// NewToolSearch returns a registry-backed tool search helper.
func NewToolSearch(reg *Registry) (*ToolSearchTool, error) {
	if reg == nil {
		return nil, fmt.Errorf("goclaw/tools: NewToolSearch(nil)")
	}
	return &ToolSearchTool{reg: reg}, nil
}

func (t *ToolSearchTool) Name() string { return "tool_search" }

func (t *ToolSearchTool) Description() string {
	return "Search available tools by name or description, including hidden MCP tools not currently exposed in the prompt. " +
		"Use this when you need to discover the right tool before calling it."
}

func (t *ToolSearchTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search terms such as a tool name, capability, API, or task.",
			},
			"kind": map[string]any{
				"type":        "string",
				"enum":        []string{"all", "mcp", "builtin"},
				"description": "Optional filter: all tools, only MCP tools, or only built-in tools.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of matches to return. Defaults to 8; capped at 20.",
			},
		},
	}
}

type toolSearchInput struct {
	Query string `json:"query"`
	Kind  string `json:"kind"`
	Limit int    `json:"limit"`
}

type toolSearchMatch struct {
	spec  ToolSpec
	score int
}

func (t *ToolSearchTool) Execute(ctx context.Context, input string) (Result, error) {
	_ = ctx
	var req toolSearchInput
	if strings.TrimSpace(input) != "" {
		if err := UnmarshalToolInputJSON(input, &req); err != nil {
			return Result{Content: err.Error(), IsError: true}, nil
		}
	}

	kind := normalizeToolSearchKind(req.Kind)
	limit := req.Limit
	if limit <= 0 {
		limit = defaultToolSearchLimit
	}
	if limit > 20 {
		limit = 20
	}

	query := strings.TrimSpace(req.Query)
	specs := t.reg.AllSpecs()
	matches := make([]toolSearchMatch, 0, len(specs))
	for _, spec := range specs {
		if !toolSearchKindMatches(spec.Name, kind) {
			continue
		}
		score := scoreToolSearchMatch(spec, query)
		if query != "" && score <= 0 {
			continue
		}
		matches = append(matches, toolSearchMatch{spec: spec, score: score})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].spec.Name < matches[j].spec.Name
	})

	if len(matches) == 0 {
		scope := kind
		if scope == "all" {
			scope = "available"
		}
		if query == "" {
			return Result{Content: fmt.Sprintf("no %s tools found", scope)}, nil
		}
		return Result{Content: fmt.Sprintf("no %s tools found for query %q", scope, query)}, nil
	}
	if len(matches) > limit {
		matches = matches[:limit]
	}

	var b strings.Builder
	fmt.Fprintf(&b, "tool_search matches (%d):", len(matches))
	for _, match := range matches {
		fmt.Fprintf(&b, "\n- %s: %s", match.spec.Name, strings.TrimSpace(match.spec.Description))
	}
	return Result{Content: b.String()}, nil
}

func normalizeToolSearchKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "all":
		return "all"
	case "mcp":
		return "mcp"
	case "builtin":
		return "builtin"
	default:
		return "all"
	}
}

func toolSearchKindMatches(name string, kind string) bool {
	isMCP := strings.HasPrefix(name, "mcp__")
	switch kind {
	case "mcp":
		return isMCP
	case "builtin":
		return !isMCP
	default:
		return true
	}
}

func scoreToolSearchMatch(spec ToolSpec, query string) int {
	if strings.TrimSpace(query) == "" {
		return 1
	}
	q := strings.ToLower(strings.TrimSpace(query))
	name := strings.ToLower(spec.Name)
	desc := strings.ToLower(spec.Description)

	score := 0
	switch {
	case name == q:
		score += 100
	case strings.Contains(name, q):
		score += 60
	case strings.Contains(desc, q):
		score += 35
	}

	for _, token := range strings.Fields(q) {
		if len(token) < 2 {
			continue
		}
		if strings.Contains(name, token) {
			score += 12
		}
		if strings.Contains(desc, token) {
			score += 6
		}
	}
	return score
}
