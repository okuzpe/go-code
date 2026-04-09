# Convenience targets for goclaw (macOS, Linux, Git Bash on Windows).
# Without make: use the same `go` commands shown under each target.

.PHONY: build test vet fmt lint race cover clean parity run run-tui run-chat run-chat-cmd

CMD := ./cmd/goclaw

build:
	go build -o goclaw $(CMD)

test:
	go test ./...

# Mock Anthropic parity bundle (no API keys; see scripts/MOCK_PARITY_HARNESS.md).
parity:
	go test ./internal/orchestrator ./internal/coordinator -count=1 -run '^TestMockParityHarness$$'

vet:
	go vet ./...

fmt:
	go fmt ./...

race:
	go test -race ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

clean:
	rm -f goclaw coverage.out

# Requires: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
lint:
	golangci-lint run ./...

run:
	go run $(CMD)

# Fullscreen Bubble Tea TUI (same as --tui / GOCLAW_USE_TUI=1).
run-tui:
	go run $(CMD) --tui

# Same interactive session as `run`; explicit subcommand for Claude Code–style muscle memory.
run-chat-cmd:
	go run $(CMD) chat

# Chat-only: no tools registered (same as --no-tools).
run-chat:
	go run $(CMD) --no-tools
