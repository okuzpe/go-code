package orchestrator

import (
	"encoding/json"
	"strings"

	"github.com/okuzpe/goclaw/internal/tools"
)

type toolPathInput struct {
	Path string `json:"path"`
}

func toolJSONPath(toolInput string) (string, bool) {
	var in toolPathInput
	if json.Unmarshal([]byte(toolInput), &in) != nil || strings.TrimSpace(in.Path) == "" {
		return "", false
	}
	return strings.TrimSpace(in.Path), true
}

// writeTargetsUnderScratch reports whether every file path in the tool input resolves under scratchDir.
func (o *Orchestrator) writeTargetsUnderScratch(toolName, toolInput string) bool {
	if strings.TrimSpace(o.scratchDir) == "" || strings.TrimSpace(o.workdir) == "" {
		return false
	}
	scope := tools.PathScope{Root: o.workdir, RelativeBase: o.launchDir}
	if strings.TrimSpace(scope.RelativeBase) == "" {
		scope.RelativeBase = o.workdir
	}

	pathUnderScratch := func(userPath string) bool {
		p, err := tools.CandidateWriteTargetPath(scope, userPath)
		if err != nil {
			return false
		}
		return tools.PathDescendsFrom(o.scratchDir, p)
	}

	switch toolName {
	case "write_file":
		p, ok := toolJSONPath(toolInput)
		if !ok {
			return false
		}
		return pathUnderScratch(p)

	case "write_files":
		var in struct {
			Files []toolPathInput `json:"files"`
		}
		if json.Unmarshal([]byte(toolInput), &in) != nil || len(in.Files) == 0 {
			return false
		}
		for _, f := range in.Files {
			if strings.TrimSpace(f.Path) == "" {
				return false
			}
			if !pathUnderScratch(f.Path) {
				return false
			}
		}
		return true

	case "edit_file", "patch":
		p, ok := toolJSONPath(toolInput)
		if !ok {
			return false
		}
		return pathUnderScratch(p)

	default:
		return false
	}
}
