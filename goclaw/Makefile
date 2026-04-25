# Convenience targets for goclaw (macOS, Linux, Git Bash on Windows).
# Without make: use the same `go` commands shown under each target.

.PHONY: build install test vet fmt lint lint-tools lint-fmt staticcheck race cover clean parity run run-code run-hub run-tui run-chat run-chat-cmd telegram telegram-bridge

CMD := ./cmd/goclaw

# Long-running `go run` can exit with status 1 on Ctrl+C on some Windows/Git Bash setups even when the
# child handles SIGINT cleanly. Build a temp binary so signals go straight to goclaw.
TELEGRAM_RUN_BIN := $(CURDIR)/.goclaw-telegram-run$(shell go env GOEXE)

build:
	go build -o goclaw $(CMD)

# Installs to $(go env GOPATH)/bin (e.g. ~/go/bin or %USERPROFILE%\go\bin). Add that directory to PATH.
install:
	go install $(CMD)

test:
	go test ./...

# Mock LLM parity bundle (OpenAI-compat HTTP mock; no API keys; see scripts/MOCK_PARITY_HARNESS.md).
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
	rm -f goclaw coverage.out "$(TELEGRAM_RUN_BIN)"

# Install golangci-lint v2 + staticcheck into $(shell go env GOPATH)/bin (add that directory to PATH).
lint-tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest

# Requires: make lint-tools (or install the same binaries another way); golangci-lint on PATH.
lint:
	golangci-lint run ./...

# Standalone static analysis (subset overlaps golangci-lint staticcheck linter).
staticcheck:
	staticcheck ./...

# Apply gofmt + gofumpt via golangci-lint (requires lint-tools).
lint-fmt:
	golangci-lint fmt ./...

run:
	go run $(CMD)

# Core local-first flows.
run-code:
	go run $(CMD) --mode build

run-hub:
	go run $(CMD) --profile coordinator

# Line-at-a-time REPL (opt out of default TUI on a TTY).
# Fullscreen TUI (default on TTY; same as explicit --tui / GOCLAW_USE_TUI=1).
run-tui:
	go run $(CMD) --tui

# Same interactive session as `run`; explicit subcommand for Claude Code–style muscle memory.
run-chat-cmd:
	go run $(CMD) chat

# Chat-only: no tools registered (same as --no-tools).
run-chat:
	go run $(CMD) --no-tools

# Optional integrations.
# Telegram Bot API bridge: prompts for missing token/allowlist in a TTY, then long-polls (see docs/goclaw/telegram-bridge.md).
# Use `make telegram` only — `make telegram start` would also try a non-existent Make target named `start`.
# Uses a temp binary (not `go run`) so Ctrl+C often maps to a clean exit on Windows/Git Bash.
telegram:
	go build -o "$(TELEGRAM_RUN_BIN)" $(CMD) && "$(TELEGRAM_RUN_BIN)" telegram start; st=$$?; rm -f "$(TELEGRAM_RUN_BIN)"; exit $$st

# Same as telegram bridge: no prompts; fails if settings are incomplete (CI / scripts).
telegram-bridge:
	go build -o "$(TELEGRAM_RUN_BIN)" $(CMD) && "$(TELEGRAM_RUN_BIN)" telegram bridge; st=$$?; rm -f "$(TELEGRAM_RUN_BIN)"; exit $$st
