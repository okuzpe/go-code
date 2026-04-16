// Package replhistory persists REPL input lines for the fullscreen TUI (↑/↓ recall).
package replhistory

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MaxLines is the maximum number of entries kept on disk (oldest dropped).
const MaxLines = 500

// File returns the path to the newline-delimited history file under userConfigDir.
func File(userConfigDir string) string {
	return filepath.Join(strings.TrimSpace(userConfigDir), "history")
}

// Load returns up to MaxLines non-empty lines in submission order (oldest first, newest last).
func Load(userConfigDir string) ([]string, error) {
	path := File(userConfigDir)
	if path == "history" || userConfigDir == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var lines []string
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(lines) > MaxLines {
		lines = lines[len(lines)-MaxLines:]
	}
	return lines, nil
}

// Append appends one line to the history file (creates parent dirs). Empty lines are skipped.
func Append(userConfigDir, line string) error {
	line = strings.TrimSpace(line)
	if line == "" || strings.TrimSpace(userConfigDir) == "" {
		return nil
	}
	if err := os.MkdirAll(userConfigDir, 0o755); err != nil {
		return fmt.Errorf("repl history mkdir: %w", err)
	}
	path := File(userConfigDir)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("repl history open: %w", err)
	}
	if _, err := fmt.Fprintf(f, "%s\n", line); err != nil {
		return fmt.Errorf("repl history write: %w", err)
	}
	if err := f.Close(); err != nil { // close before rotate re-reads the file
		return err
	}
	return rotateTail(path, MaxLines)
}

func rotateTail(path string, max int) error {
	if max < 1 {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return nil
	}
	var lines []string
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		t := strings.TrimSpace(sc.Text())
		if t != "" {
			lines = append(lines, t)
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if len(lines) <= max {
		return nil
	}
	keep := lines[len(lines)-max:]
	var sb strings.Builder
	for _, ln := range keep {
		sb.WriteString(ln)
		sb.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(sb.String()), 0o600)
}
