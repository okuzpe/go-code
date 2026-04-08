package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/okuzpe/goclaw/internal/agents"
	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/hooks"
	"github.com/okuzpe/goclaw/internal/permissions"
	"github.com/okuzpe/goclaw/internal/session"
	"github.com/okuzpe/goclaw/internal/tools"
)

type fakeTool struct {
	name string
}

func (f fakeTool) Name() string                       { return f.name }
func (f fakeTool) Description() string                 { return "fake" }
func (f fakeTool) InputSchema() any                    { return map[string]any{"type": "object"} }
func (f fakeTool) Execute(context.Context, string) (tools.Result, error) {
	return tools.Result{}, nil
}

func TestBuildRequestExploreProfileOmitsBash(t *testing.T) {
	reg := tools.New()
	reg.Register(tools.NewBash())
	reg.Register(tools.NewReadFile(t.TempDir()))

	o := &Orchestrator{
		cfg:     config.Default(),
		session: session.New(),
		tools:   reg,
		perms:   permissions.NewPolicy(),
		hooks:   hooks.New(),
		profile: agents.Explore,
	}

	req := o.buildRequest()
	for _, s := range req.Tools {
		if s.Name == "bash" {
			t.Fatal("explore profile is read-only: bash must not appear in tool specs")
		}
	}
}

func TestBuildRequestAllowlistWildcardMCP(t *testing.T) {
	reg := tools.New()
	reg.Register(fakeTool{name: "read_file"})
	reg.Register(fakeTool{name: "mcp__demo__echo"})
	o := &Orchestrator{
		cfg:     config.Default(),
		session: session.New(),
		tools:   reg,
		perms:   permissions.NewPolicy(),
		hooks:   hooks.New(),
		profile: agents.Profile{
			Name:          "t",
			ToolAllowlist: []string{"read_file", "mcp__demo__*"},
		},
	}
	req := o.buildRequest()
	var saw bool
	for _, s := range req.Tools {
		if s.Name == "mcp__demo__echo" {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("tools: %#v", req.Tools)
	}
}

func TestBuildRequestReadOnlyStripsMCP(t *testing.T) {
	reg := tools.New()
	reg.Register(fakeTool{name: "mcp__srv__t"})
	reg.Register(fakeTool{name: "read_file"})
	o := &Orchestrator{
		cfg:     config.Default(),
		session: session.New(),
		tools:   reg,
		perms:   permissions.NewPolicy(),
		hooks:   hooks.New(),
		profile: agents.Profile{Name: "ro", ReadOnly: true},
	}
	req := o.buildRequest()
	for _, s := range req.Tools {
		if strings.HasPrefix(s.Name, "mcp__") {
			t.Fatalf("mcp tool leaked into read-only specs: %s", s.Name)
		}
	}
}
