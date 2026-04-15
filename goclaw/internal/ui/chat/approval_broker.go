package chat

import (
	"context"

	"github.com/okuzpe/goclaw/internal/orchestrator"
	"github.com/okuzpe/goclaw/internal/text"
)

const tuiToolApprovalPreviewMaxRunes = 400

// ApprovalRequest is a pending tool approval prompt in the fullscreen TUI.
type ApprovalRequest struct {
	ToolName string
	Preview  string
	Resp     chan bool
}

// ApprovalBroker bridges orchestrator.ToolApprover to Bubble Tea via a channel.
type ApprovalBroker struct {
	Requests chan ApprovalRequest
}

// NewApprovalBroker creates a broker with a bounded request queue.
func NewApprovalBroker() *ApprovalBroker {
	return &ApprovalBroker{Requests: make(chan ApprovalRequest, 8)}
}

// ToolApprover returns the orchestrator callback that enqueues approval UI work.
func (b *ApprovalBroker) ToolApprover() orchestrator.ToolApprover {
	return func(ctx context.Context, toolName, toolInput string) (bool, error) {
		preview := text.TruncateRunes(
			orchestrator.FormatToolUsePreview(toolName, toolInput),
			tuiToolApprovalPreviewMaxRunes,
		)
		req := ApprovalRequest{
			ToolName: toolName,
			Preview:  preview,
			Resp:     make(chan bool, 1),
		}
		select {
		case b.Requests <- req:
		case <-ctx.Done():
			return false, ctx.Err()
		}
		select {
		case ok := <-req.Resp:
			return ok, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
}
