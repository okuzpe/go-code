package slashcmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const editorBoilerplate = "# goclaw — write your message below. Save and close the editor to send.\n# Lines starting with # are ignored when building the prompt.\n\n"

// openPromptEditor creates a temp file, launches $EDITOR (or vi / notepad), returns non-comment body.
func openPromptEditor(ctx context.Context, workdir string) (string, error) {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "vi"
		}
	}
	dir := workdir
	if dir == "" {
		dir = os.TempDir()
	}
	f, err := os.CreateTemp(dir, "goclaw-prompt-*.md")
	if err != nil {
		return "", fmt.Errorf("temp file: %w", err)
	}
	path := f.Name()
	if _, err := f.WriteString(editorBoilerplate); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write temp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}

	cmd := exec.CommandContext(ctx, editor, path)
	cmd.Dir = workdir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("editor %q: %w", editor, err)
	}

	raw, err := os.ReadFile(path)
	_ = os.Remove(path)
	if err != nil {
		return "", fmt.Errorf("read temp: %w", err)
	}
	return stripEditorComments(string(raw)), nil
}

func stripEditorComments(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
