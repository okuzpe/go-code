// Package permissions implements the Deny / Ask / Allow policy engine.
package permissions

import (
	"fmt"
	"strings"
)

// Mode is the permission level for a tool call.
type Mode int

const (
	ModeAsk   Mode = iota // default: prompt the user
	ModeAllow             // auto-approve
	ModeDeny              // always reject
)

// Decision is the result of policy evaluation.
type Decision int

const (
	DecisionAllow Decision = iota
	DecisionDeny
	DecisionAsk
)

// Policy maps tool names to their allowed mode.
type Policy struct {
	defaults map[string]Mode
	global   Mode
}

// NewPolicy returns a fail-closed policy (Ask by default).
func NewPolicy() *Policy {
	return &Policy{
		defaults: make(map[string]Mode),
		global:   ModeAsk,
	}
}

// Set overrides the mode for a specific tool.
func (p *Policy) Set(toolName string, m Mode) {
	p.defaults[toolName] = m
}

// Evaluate returns the decision for a given tool name.
func (p *Policy) Evaluate(toolName string) Decision {
	m, ok := p.defaults[toolName]
	if !ok {
		m = p.global
	}
	switch m {
	case ModeAllow:
		return DecisionAllow
	case ModeDeny:
		return DecisionDeny
	default:
		return DecisionAsk
	}
}

// ParseMode converts settings strings ("ask", "allow", "deny") into a Mode.
func ParseMode(s string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ask":
		return ModeAsk, nil
	case "allow":
		return ModeAllow, nil
	case "deny":
		return ModeDeny, nil
	default:
		return ModeAsk, fmt.Errorf("permissions: unknown mode %q (want ask, allow, deny)", s)
	}
}
