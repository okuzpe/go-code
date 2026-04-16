package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const getChatHTTPTimeout = 20 * time.Second

// ParseAllowlistEntries parses comma-separated allowlist values: decimal user ids, @digits (same as id),
// or @PublicUsername resolved via Bot API getChat (requires a valid bot token on the client).
// Resolving your bot's own @username is rejected (that id would never match message.from.id for your account).
func ParseAllowlistEntries(ctx context.Context, client *Client, csv string) ([]int64, error) {
	botUserID, err := client.getMeBotUserID(ctx)
	if err != nil {
		return nil, fmt.Errorf("telegram getMe: %w", err)
	}
	parts := strings.Split(csv, ",")
	var out []int64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := parseOneAllowlistEntry(ctx, client, botUserID, p)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func parseOneAllowlistEntry(ctx context.Context, client *Client, botUserID int64, part string) (int64, error) {
	// Some clients show user ids as "#123456789"; strip leading # before parsing.
	trimmed := strings.TrimSpace(part)
	trimmed = strings.TrimLeft(trimmed, "#")
	trimmed = strings.TrimSpace(trimmed)
	afterAt := strings.TrimPrefix(trimmed, "@")
	if isUnsignedDecimalString(afterAt) {
		n, err := strconv.ParseInt(afterAt, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("allowlist entry %q: %w", part, err)
		}
		if n == botUserID {
			return 0, fmt.Errorf("allowlist entry %q: that is this bot's Telegram user id — use your personal account id or @YourUsername, not the bot", part)
		}
		return n, nil
	}
	if strings.HasPrefix(trimmed, "@") {
		id, err := client.getChatNumericID(ctx, trimmed)
		if err != nil {
			return 0, fmt.Errorf("allowlist entry %q (getChat): %w", part, err)
		}
		if id == botUserID {
			return 0, fmt.Errorf("allowlist entry %q: that resolves to this bot's id — use your personal Telegram @username (or numeric id), not the bot's name", part)
		}
		return id, nil
	}
	return 0, fmt.Errorf("allowlist entry %q: use a numeric user id or @YourPublicUsername (not a name without @)", part)
}

func isUnsignedDecimalString(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func (c *Client) getMeBotUserID(ctx context.Context) (int64, error) {
	callCtx, cancel := context.WithTimeout(ctx, getChatHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, c.baseURL()+"/getMe", nil)
	if err != nil {
		return 0, fmt.Errorf("new request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("http %d: %s", resp.StatusCode, truncateForLog(body, 256))
	}
	var env struct {
		OK     bool `json:"ok"`
		Result struct {
			ID int64 `json:"id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return 0, fmt.Errorf("decode: %w", err)
	}
	if !env.OK {
		return 0, fmt.Errorf("ok=false: %s", truncateForLog(body, 256))
	}
	return env.Result.ID, nil
}

func (c *Client) getChatNumericID(ctx context.Context, chatRef string) (int64, error) {
	chatRef = strings.TrimSpace(chatRef)
	if chatRef == "" {
		return 0, fmt.Errorf("empty chat reference")
	}
	payload := map[string]any{"chat_id": chatRef}
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal: %w", err)
	}
	callCtx, cancel := context.WithTimeout(ctx, getChatHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, c.baseURL()+"/getChat", bytes.NewReader(raw))
	if err != nil {
		return 0, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("http %d: %s", resp.StatusCode, truncateForLog(body, 256))
	}
	var env struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		Result      struct {
			ID int64 `json:"id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return 0, fmt.Errorf("decode: %w", err)
	}
	if !env.OK {
		desc := strings.TrimSpace(env.Description)
		if desc != "" {
			return 0, fmt.Errorf("%s (username must be public, or use your numeric id from Telegram / @userinfobot)", desc)
		}
		return 0, fmt.Errorf("ok=false: %s", truncateForLog(body, 256))
	}
	return env.Result.ID, nil
}
