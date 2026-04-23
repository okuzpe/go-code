package orchestrator

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/tools"
)

type checkpointFile struct {
	DisplayPath string
	Resolved    string
	Existed     bool
	BackupPath  string
	Perm        os.FileMode
}

type checkpointEntry struct {
	ToolName  string
	CreatedAt time.Time
	Files     []checkpointFile
}

var checkpointableTools = map[string]struct{}{
	"write_file":  {},
	"write_files": {},
	"edit_file":   {},
	"patch":       {},
}

func (o *Orchestrator) toolPathScope() tools.PathScope {
	return tools.NormalizePathScope(tools.PathScope{
		Root:         o.workdir,
		RelativeBase: o.launchDir,
	})
}

func (o *Orchestrator) capturePendingCheckpoint(toolName, input string) (*checkpointEntry, error) {
	if strings.TrimSpace(o.scratchDir) == "" {
		return nil, nil
	}
	if _, ok := checkpointableTools[toolName]; !ok {
		return nil, nil
	}
	displayPaths, err := checkpointDisplayPaths(toolName, input)
	if err != nil {
		return nil, nil
	}
	scope := o.toolPathScope()
	entry := &checkpointEntry{
		ToolName:  toolName,
		CreatedAt: time.Now(),
	}
	for _, displayPath := range displayPaths {
		resolved, err := tools.CandidateWriteTargetPath(scope, displayPath)
		if err != nil {
			return nil, nil
		}
		entry.Files = append(entry.Files, checkpointFile{
			DisplayPath: displayPath,
			Resolved:    resolved,
		})
	}
	if err := o.populateCheckpointBacking(entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func checkpointDisplayPaths(toolName, input string) ([]string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		return nil, err
	}
	if toolName == "write_files" {
		var payload struct {
			Files []struct {
				Path string `json:"path"`
			} `json:"files"`
		}
		if err := json.Unmarshal([]byte(input), &payload); err != nil {
			return nil, err
		}
		paths := make([]string, 0, len(payload.Files))
		for _, file := range payload.Files {
			path := strings.TrimSpace(file.Path)
			if path == "" {
				continue
			}
			paths = append(paths, path)
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("files paths are required")
		}
		return paths, nil
	}
	var path string
	if err := json.Unmarshal(raw["path"], &path); err != nil {
		return nil, err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}
	return []string{path}, nil
}

func (o *Orchestrator) populateCheckpointBacking(entry *checkpointEntry) error {
	checkpointDir := filepath.Join(o.scratchDir, "undo")
	if err := os.MkdirAll(checkpointDir, 0o700); err != nil {
		return fmt.Errorf("checkpoint dir: %w", err)
	}
	for index := range entry.Files {
		current := &entry.Files[index]
		info, err := os.Stat(current.Resolved)
		if err != nil {
			if os.IsNotExist(err) {
				current.Existed = false
				continue
			}
			return fmt.Errorf("checkpoint stat %s: %w", current.DisplayPath, err)
		}
		current.Existed = true
		current.Perm = info.Mode().Perm()
		data, err := os.ReadFile(current.Resolved)
		if err != nil {
			return fmt.Errorf("checkpoint read %s: %w", current.DisplayPath, err)
		}
		backup, err := os.CreateTemp(checkpointDir, "file-*")
		if err != nil {
			return fmt.Errorf("checkpoint temp %s: %w", current.DisplayPath, err)
		}
		if _, err := backup.Write(data); err != nil {
			_ = backup.Close()
			_ = os.Remove(backup.Name())
			return fmt.Errorf("checkpoint write %s: %w", current.DisplayPath, err)
		}
		if err := backup.Close(); err != nil {
			_ = os.Remove(backup.Name())
			return fmt.Errorf("checkpoint close %s: %w", current.DisplayPath, err)
		}
		current.BackupPath = backup.Name()
	}
	return nil
}

func (o *Orchestrator) commitCheckpoint(entry *checkpointEntry) {
	if entry == nil {
		return
	}
	o.checkpointMu.Lock()
	defer o.checkpointMu.Unlock()
	o.checkpoints = append(o.checkpoints, *entry)
}

func (o *Orchestrator) discardCheckpoint(entry *checkpointEntry) {
	if entry == nil {
		return
	}
	for _, file := range entry.Files {
		if strings.TrimSpace(file.BackupPath) == "" {
			continue
		}
		if err := os.Remove(file.BackupPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("checkpoint discard: remove backup failed", "path", file.BackupPath, "err", err)
		}
	}
}

func (o *Orchestrator) clearCheckpoints() {
	o.checkpointMu.Lock()
	defer o.checkpointMu.Unlock()
	for _, entry := range o.checkpoints {
		for _, file := range entry.Files {
			if strings.TrimSpace(file.BackupPath) == "" {
				continue
			}
			if err := os.Remove(file.BackupPath); err != nil && !os.IsNotExist(err) {
				slog.Warn("checkpoint clear: remove backup failed", "path", file.BackupPath, "err", err)
			}
		}
	}
	o.checkpoints = nil
}

// UndoLastCheckpoint restores the latest successful write/edit/patch tool change in this session.
func (o *Orchestrator) UndoLastCheckpoint() (string, error) {
	o.checkpointMu.Lock()
	if len(o.checkpoints) == 0 {
		o.checkpointMu.Unlock()
		return "", fmt.Errorf("no undo checkpoint available in this session")
	}
	entry := o.checkpoints[len(o.checkpoints)-1]
	o.checkpoints = o.checkpoints[:len(o.checkpoints)-1]
	o.checkpointMu.Unlock()

	for index := len(entry.Files) - 1; index >= 0; index-- {
		file := entry.Files[index]
		if !file.Existed {
			if err := os.Remove(file.Resolved); err != nil && !os.IsNotExist(err) {
				return "", fmt.Errorf("undo remove %s: %w", file.DisplayPath, err)
			}
			continue
		}
		data, err := os.ReadFile(file.BackupPath)
		if err != nil {
			return "", fmt.Errorf("undo read backup %s: %w", file.DisplayPath, err)
		}
		if err := atomicRestoreFile(file.Resolved, data, file.Perm); err != nil {
			return "", fmt.Errorf("undo restore %s: %w", file.DisplayPath, err)
		}
	}
	o.discardCheckpoint(&entry)

	paths := make([]string, 0, len(entry.Files))
	for _, file := range entry.Files {
		paths = append(paths, file.DisplayPath)
	}
	return fmt.Sprintf("undid %s for %s", entry.ToolName, strings.Join(paths, ", ")), nil
}

func atomicRestoreFile(targetPath string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(targetPath), ".goclaw-undo-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	if err := os.Rename(tmpName, targetPath); err != nil {
		return err
	}
	ok = true
	return nil
}
