package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/okuzpe/goclaw/internal/orchestrator"
)

func automationOutputToolApprover(_ context.Context, toolName, _ string) (bool, error) {
	return false, fmt.Errorf(
		"automation output mode cannot prompt for tool approval; set \"tool_permissions\" for %q to \"allow\" in settings (or use --no-tools)",
		toolName,
	)
}

func withAutomationOutputToolApprover(base []orchestrator.Option) []orchestrator.Option {
	return append(append([]orchestrator.Option(nil), base...), orchestrator.WithToolApprover(automationOutputToolApprover))
}

func readSingleLineStdin(r io.Reader) (string, error) {
	br := bufio.NewReader(r)
	line, err := br.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// RunChatJSONOutput runs one user message from stdin and prints JSON to stdout.
func RunChatJSONOutput(ctx context.Context, rt *ChatRuntime) error {
	line, err := readSingleLineStdin(os.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	if line == "" {
		return errors.New("automation output: empty line on stdin; pipe one non-empty line (example: echo \"hi\" | goclaw --output-format json)")
	}
	return RunChatJSONOutputFromLine(ctx, rt, line)
}

// RunChatJSONOutputFromLine runs one user message and prints JSON {"response","toolCalls"} to stdout.
func RunChatJSONOutputFromLine(ctx context.Context, rt *ChatRuntime, line string) error {
	if strings.TrimSpace(line) == "" {
		return errors.New("automation output: empty user message")
	}

	orch := newAutomationOrchestrator(rt)
	var trace []orchestrator.JSONToolCall
	resp, err := runSessionTurn(ctx, rt, orch, line, NopStreamSink{}, sessionTurnOptions{
		ApplyAutoProfile: true,
		ToolTrace:        &trace,
	})
	if err != nil {
		return err
	}

	out := orchestrator.JSONTurnResult{Response: resp, ToolCalls: trace}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		return err
	}
	if err := rt.SaveSession(); err != nil {
		slog.Warn("failed to save session", "err", err)
	}
	return nil
}

// RunChatTextOutputFromLine runs one user message and prints the final assistant text to stdout (no REPL).
func RunChatTextOutputFromLine(ctx context.Context, rt *ChatRuntime, line string) error {
	if strings.TrimSpace(line) == "" {
		return errors.New("automation output: empty user message")
	}

	orch := newAutomationOrchestrator(rt)
	resp, err := runSessionTurn(ctx, rt, orch, line, NopStreamSink{}, sessionTurnOptions{
		ApplyAutoProfile: true,
	})
	if err != nil {
		return err
	}
	fmt.Print(resp)
	if !strings.HasSuffix(resp, "\n") {
		fmt.Println()
	}
	if err := rt.SaveSession(); err != nil {
		slog.Warn("failed to save session", "err", err)
	}
	return nil
}
