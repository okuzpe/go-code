package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// PatchTool applies a unified diff (git-style or standard unified) to one workspace file.
type PatchTool struct {
	root string
}

// NewPatch returns a patch tool scoped to root.
func NewPatch(root string) *PatchTool {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return &PatchTool{root: filepath.Clean(abs)}
}

var _ Tool = (*PatchTool)(nil)

func (PatchTool) Name() string { return "patch" }

func (PatchTool) Description() string {
	return "Apply a unified diff to a single workspace file. " +
		"The diff must change exactly one file; path must match the relative path in the --- a/... and +++ b/... headers. " +
		"Binary patches are not supported. " +
		"Prefer edit_file for small string replacements; use patch for multi-hunk or git-generated diffs. " +
		"Minimal valid diff shape (context lines start with a single space; removed with -; added with +): " +
		`diff --git a/foo.go b/foo.go` + "\n" +
		`--- a/foo.go` + "\n" +
		`+++ b/foo.go` + "\n" +
		`@@ -1,3 +1,3 @@` + "\n" +
		` package main` + "\n" +
		`-old line` + "\n" +
		`+new line` + "\n" +
		` func main() {}`
}

func (PatchTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Workspace-relative path to the file being patched (must match the single file in the diff)",
			},
			"diff": map[string]any{
				"type": "string",
				"description": "Complete unified diff for one file: diff --git line, --- a/<path>, +++ b/<path>, @@ hunk @@, " +
					"then lines with leading space (context), - (remove), + (add). Must match path in headers.",
			},
		},
		"required": []string{"path", "diff"},
	}
}

type patchInput struct {
	Path string `json:"path"`
	Diff string `json:"diff"`
}

// Execute implements Tool.
func (t *PatchTool) Execute(_ context.Context, input string) (Result, error) {
	var in patchInput
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return Result{Content: fmt.Sprintf("invalid json input: %v", err), IsError: true}, nil
	}

	in.Path = strings.TrimSpace(in.Path)
	in.Diff = strings.ReplaceAll(in.Diff, "\r\n", "\n")
	// Do not TrimSpace(in.Diff): a trailing newline is part of the unified diff format;
	// trimming would strip the last context line's LF and break application.
	if in.Path == "" {
		return Result{Content: "path is required", IsError: true}, nil
	}
	if strings.TrimSpace(in.Diff) == "" {
		return Result{Content: "diff is required", IsError: true}, nil
	}
	if len(in.Diff) > MaxPatchDiffBytes {
		return Result{
			Content: fmt.Sprintf("diff too large: %d bytes exceeds %d-byte limit", len(in.Diff), MaxPatchDiffBytes),
			IsError: true,
		}, nil
	}

	files, _, err := gitdiff.Parse(strings.NewReader(in.Diff))
	if err != nil {
		return Result{Content: fmt.Sprintf("parse diff: %v", err), IsError: true}, nil
	}
	if len(files) == 0 {
		return Result{Content: "diff contains no file hunks", IsError: true}, nil
	}
	if len(files) != 1 {
		return Result{Content: fmt.Sprintf("diff must affect exactly one file (found %d)", len(files)), IsError: true}, nil
	}

	f := files[0]
	if f.IsBinary {
		return Result{Content: "binary patches are not supported", IsError: true}, nil
	}

	wantRel := patchDiffRelPath(f)
	userRel := filepath.ToSlash(filepath.Clean(in.Path))
	if wantRel == "" {
		return Result{Content: "could not determine file path from diff headers", IsError: true}, nil
	}
	if wantRel != userRel {
		return Result{
			Content: fmt.Sprintf("path %q does not match diff target %q", userRel, wantRel),
			IsError: true,
		}, nil
	}

	var (
		resolved string
		src      []byte
	)
	switch {
	case f.IsNew:
		var resolveErr error
		resolved, resolveErr = resolveWriteTargetUnderRoot(t.root, in.Path)
		if resolveErr != nil {
			return Result{Content: resolveErr.Error(), IsError: true}, nil
		}
		src = nil
	case f.IsDelete:
		var resolveErr error
		resolved, resolveErr = resolveExistingPathUnderRoot(t.root, in.Path)
		if resolveErr != nil {
			return Result{Content: resolveErr.Error(), IsError: true}, nil
		}
		raw, readErr := os.ReadFile(resolved)
		if readErr != nil {
			return Result{Content: fmt.Sprintf("read file: %v", readErr), IsError: true}, nil
		}
		src = raw
	default:
		var resolveErr error
		resolved, resolveErr = resolveExistingPathUnderRoot(t.root, in.Path)
		if resolveErr != nil {
			return Result{Content: resolveErr.Error(), IsError: true}, nil
		}
		raw, readErr := os.ReadFile(resolved)
		if readErr != nil {
			return Result{Content: fmt.Sprintf("read file: %v", readErr), IsError: true}, nil
		}
		src = raw
	}

	var out bytes.Buffer
	applyErr := gitdiff.Apply(&out, bytes.NewReader(src), f)
	if applyErr != nil {
		if errors.Is(applyErr, &gitdiff.Conflict{}) {
			return Result{Content: fmt.Sprintf("patch conflict: %v", applyErr), IsError: true}, nil
		}
		return Result{Content: fmt.Sprintf("apply patch: %v", applyErr), IsError: true}, nil
	}

	outBytes := out.Bytes()
	if len(outBytes) > MaxWriteFileBytes {
		return Result{
			Content: fmt.Sprintf("patched result too large: %d bytes exceeds %d-byte limit", len(outBytes), MaxWriteFileBytes),
			IsError: true,
		}, nil
	}

	if f.IsDelete {
		if len(outBytes) > 0 {
			return Result{Content: "internal error: delete patch produced non-empty output", IsError: true}, nil
		}
		if err := os.Remove(resolved); err != nil {
			return Result{Content: fmt.Sprintf("remove file: %v", err), IsError: true}, nil
		}
		return Result{Content: fmt.Sprintf("deleted %s", in.Path)}, nil
	}

	perm := os.FileMode(0o600)
	if info, statErr := os.Stat(resolved); statErr == nil {
		perm = info.Mode().Perm()
	}

	if err := atomicWriteFile(resolved, outBytes, perm); err != nil {
		return Result{Content: fmt.Sprintf("write file: %v", err), IsError: true}, nil
	}

	action := "patched"
	if f.IsNew {
		action = "created"
	}
	return Result{Content: fmt.Sprintf("%s %s (%d bytes)", action, in.Path, len(outBytes))}, nil
}

func normalizeGitDiffPath(name string) string {
	name = strings.TrimSpace(name)
	if name == "/dev/null" || name == "dev/null" {
		return ""
	}
	for _, prefix := range []string{"a/", "b/"} {
		if strings.HasPrefix(name, prefix) {
			name = name[len(prefix):]
			break
		}
	}
	return filepath.ToSlash(filepath.Clean(name))
}

func patchDiffRelPath(f *gitdiff.File) string {
	switch {
	case f.IsNew:
		return normalizeGitDiffPath(f.NewName)
	case f.IsDelete:
		return normalizeGitDiffPath(f.OldName)
	default:
		o := normalizeGitDiffPath(f.OldName)
		if o != "" {
			return o
		}
		return normalizeGitDiffPath(f.NewName)
	}
}
