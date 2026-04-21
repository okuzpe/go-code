package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ChatMessage is one turn for Ollama /api/chat.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string         `json:"model"`
	Messages []ChatMessage  `json:"messages"`
	Stream   bool           `json:"stream"`
	Options  map[string]any `json:"options,omitempty"`
}

type chatStreamLine struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// StreamChat calls Ollama POST /api/chat with stream:true and forwards assistant content deltas.
func StreamChat(ctx context.Context, baseURL, model string, messages []ChatMessage, onDelta func(string) error) error {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	if strings.TrimSpace(model) == "" {
		return fmt.Errorf("ollama: empty model")
	}
	body, err := json.Marshal(chatRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("ollama: %s: %s", resp.Status, strings.TrimSpace(string(slurp)))
	}

	sc := bufio.NewScanner(resp.Body)
	// Lines can be large; raise buffer for big JSON payloads.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		content, _, err := ParseChatStreamLine(line)
		if err != nil {
			continue
		}
		if content != "" {
			if err := onDelta(content); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}

// ParseChatStreamLine extracts assistant delta text from one NDJSON line of /api/chat.
func ParseChatStreamLine(line []byte) (content string, done bool, err error) {
	var row chatStreamLine
	if err := json.Unmarshal(line, &row); err != nil {
		return "", false, err
	}
	return row.Message.Content, row.Done, nil
}

// PingOllama checks that the server responds (HEAD / or GET /api/tags with short timeout).
func PingOllama(ctx context.Context, baseURL string) error {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ollama: %s", resp.Status)
	}
	return nil
}
