package planfile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MetaFilename is the workspace-local approval record for plan execution gates.
const MetaFilename = "plan.meta.json"

// MetaPath returns <workdir>/.goclaw/plan.meta.json
func MetaPath(workdir string) string {
	return filepath.Join(workdir, Subdir, MetaFilename)
}

// PlanApprovalMeta records an explicit user approval for a specific plan file revision.
type PlanApprovalMeta struct {
	Version           int    `json:"version"`
	PlanPath          string `json:"plan_path"`      // absolute, filepath.Clean
	ContentSHA256     string `json:"content_sha256"` // hex of UTF-8 body
	ApprovedAtRFC3339 string `json:"approved_at"`    // RFC3339 UTC
}

// WriteApproval writes plan.meta.json after hashing planBody (UTF-8).
func WriteApproval(workdir, planAbsPath, planBody string) error {
	workdir = strings.TrimSpace(workdir)
	if workdir == "" {
		return fmt.Errorf("workspace directory not set")
	}
	planAbsPath = filepath.Clean(planAbsPath)
	if err := os.MkdirAll(filepath.Join(workdir, Subdir), 0o700); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	sum := sha256.Sum256([]byte(planBody))
	meta := PlanApprovalMeta{
		Version:           1,
		PlanPath:          planAbsPath,
		ContentSHA256:     hex.EncodeToString(sum[:]),
		ApprovedAtRFC3339: time.Now().UTC().Format(time.RFC3339),
	}
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(MetaPath(workdir), raw, 0o600)
}

// ClearApproval removes plan.meta.json if present.
func ClearApproval(workdir string) error {
	workdir = strings.TrimSpace(workdir)
	if workdir == "" {
		return fmt.Errorf("workspace directory not set")
	}
	p := MetaPath(workdir)
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ErrPlanNotApproved is returned when plan_require_apply_approval is on and meta is missing or stale.
var ErrPlanNotApproved = errors.New("plan not approved or plan file changed since approval")

// VerifyApproval returns nil if meta exists, paths match, and SHA256 matches planBody.
func VerifyApproval(workdir, planAbsPath, planBody string) error {
	workdir = strings.TrimSpace(workdir)
	if workdir == "" {
		return fmt.Errorf("workspace directory not set")
	}
	planAbsPath = filepath.Clean(planAbsPath)
	raw, err := os.ReadFile(MetaPath(workdir))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: run /plan approve after reviewing the plan", ErrPlanNotApproved)
		}
		return fmt.Errorf("read plan meta: %w", err)
	}
	var meta PlanApprovalMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return fmt.Errorf("plan meta: %w", err)
	}
	if meta.Version < 1 {
		return fmt.Errorf("%w: invalid meta", ErrPlanNotApproved)
	}
	gotPath := filepath.Clean(meta.PlanPath)
	if !strings.EqualFold(gotPath, planAbsPath) {
		return fmt.Errorf("%w: approved file %q does not match %q", ErrPlanNotApproved, gotPath, planAbsPath)
	}
	sum := sha256.Sum256([]byte(planBody))
	want := hex.EncodeToString(sum[:])
	if !strings.EqualFold(strings.TrimSpace(meta.ContentSHA256), want) {
		return fmt.Errorf("%w: plan content changed since /plan approve", ErrPlanNotApproved)
	}
	return nil
}

// ApprovalStatus returns a short human-readable line for /plan review (empty if none).
func ApprovalStatus(workdir, planAbsPath, planBody string) string {
	err := VerifyApproval(workdir, planAbsPath, planBody)
	if err == nil {
		return "Approval: OK — this plan revision matches /plan approve on disk."
	}
	if errors.Is(err, ErrPlanNotApproved) {
		return "Approval: none or stale — run /plan approve after review (required when plan_require_apply_approval is true)."
	}
	return "Approval: check failed — " + err.Error()
}
