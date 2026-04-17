// Package telegram implements a minimal Telegram Bot API client for the optional goclaw bridge.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	apiHost               = "api.telegram.org"
	maxMessageRunes       = 4096
	getUpdatesTimeoutSec  = 50
	httpSendTimeout       = 60 * time.Second
	sendChatActionTimeout = 12 * time.Second
)

// Client calls only https://api.telegram.org with the given bot token.
type Client struct {
	token      string
	httpClient *http.Client
}

// NewClient returns a client for Bot API requests. token must be non-empty.
func NewClient(token string) *Client {
	return &Client{
		token: strings.TrimSpace(token),
		httpClient: &http.Client{
			Timeout: (getUpdatesTimeoutSec + 15) * time.Second,
		},
	}
}

func (c *Client) baseURL() string {
	// Bot tokens are path-safe (no slashes); do not URL-escape the colon.
	return "https://" + apiHost + "/bot" + c.token
}

// getUpdatesResponse is a partial Bot API envelope.
type getUpdatesResponse struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
}

// Update is a partial update object (private messages only for the bridge).
type Update struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		MessageID int64 `json:"message_id"`
		From      *struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Chat *struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

// GetUpdates performs long-polling getUpdates. offset is the next update_id to fetch after (use lastUpdateID+1).
func (c *Client) GetUpdates(ctx context.Context, offset int64) ([]Update, error) {
	query := url.Values{}
	if offset > 0 {
		query.Set("offset", fmt.Sprintf("%d", offset))
	}
	query.Set("timeout", fmt.Sprintf("%d", getUpdatesTimeoutSec))
	full := c.baseURL() + "/getUpdates?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, fmt.Errorf("telegram getUpdates: new request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram getUpdates: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("telegram getUpdates: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram getUpdates: http %d: %s", resp.StatusCode, truncateForLog(body, 512))
	}
	var env getUpdatesResponse
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("telegram getUpdates: decode envelope: %w", err)
	}
	if !env.OK {
		return nil, fmt.Errorf("telegram getUpdates: ok=false: %s", truncateForLog(body, 512))
	}
	var updates []Update
	if len(env.Result) == 0 || string(env.Result) == "null" {
		return nil, nil
	}
	if err := json.Unmarshal(env.Result, &updates); err != nil {
		return nil, fmt.Errorf("telegram getUpdates: decode result: %w", err)
	}
	return updates, nil
}

// SendTyping posts sendChatAction with action "typing" so Telegram shows a typing indicator (~5s; call periodically while waiting on the model).
func (c *Client) SendTyping(ctx context.Context, chatID int64) error {
	payload := map[string]any{
		"chat_id": chatID,
		"action":  "typing",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram sendChatAction: marshal: %w", err)
	}
	sendCtx, cancel := context.WithTimeout(ctx, sendChatActionTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(sendCtx, http.MethodPost, c.baseURL()+"/sendChatAction", bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("telegram sendChatAction: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram sendChatAction: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("telegram sendChatAction: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram sendChatAction: http %d: %s", resp.StatusCode, truncateForLog(body, 256))
	}
	var env struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("telegram sendChatAction: decode: %w", err)
	}
	if !env.OK {
		return fmt.Errorf("telegram sendChatAction: ok=false: %s", strings.TrimSpace(env.Description))
	}
	return nil
}

// SendMessageText sends plain text to chatID, splitting when longer than maxMessageRunes.
func (c *Client) SendMessageText(ctx context.Context, chatID int64, text string) error {
	chunks := SplitMessage(text, maxMessageRunes)
	if len(chunks) == 0 {
		chunks = []string{"(empty reply)"}
	}
	for _, chunk := range chunks {
		if err := c.sendMessageChunk(ctx, chatID, chunk); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) sendMessageChunk(ctx context.Context, chatID int64, text string) error {
	payload := map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram sendMessage: marshal: %w", err)
	}
	sendCtx, cancel := context.WithTimeout(ctx, httpSendTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(sendCtx, http.MethodPost, c.baseURL()+"/sendMessage", bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("telegram sendMessage: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram sendMessage: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("telegram sendMessage: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram sendMessage: http %d: %s", resp.StatusCode, truncateForLog(body, 512))
	}
	var env struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("telegram sendMessage: decode: %w", err)
	}
	if !env.OK {
		return fmt.Errorf("telegram sendMessage: ok=false: %s", strings.TrimSpace(env.Description))
	}
	return nil
}

// SplitMessage splits s into segments of at most maxRunes runes.
func SplitMessage(s string, maxRunes int) []string {
	if maxRunes <= 0 {
		maxRunes = maxMessageRunes
	}
	runes := []rune(strings.TrimSpace(s))
	if len(runes) == 0 {
		return nil
	}
	var out []string
	for len(runes) > 0 {
		if len(runes) <= maxRunes {
			return append(out, string(runes))
		}
		out = append(out, string(runes[:maxRunes]))
		runes = runes[maxRunes:]
	}
	return out
}

func truncateForLog(b []byte, max int) string {
	s := string(b)
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

// AllowedUserIDs builds a set for O(1) lookup.
func AllowedUserIDs(ids []int64) map[int64]struct{} {
	m := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return m
}

// UserAndChatFromUpdate returns sender user id and chat id when the update is a text message.
func UserAndChatFromUpdate(u Update) (userID int64, chatID int64, text string, ok bool) {
	if u.Message == nil {
		return 0, 0, "", false
	}
	if u.Message.From == nil || u.Message.Chat == nil {
		return 0, 0, "", false
	}
	t := strings.TrimSpace(u.Message.Text)
	if t == "" {
		return 0, 0, "", false
	}
	return u.Message.From.ID, u.Message.Chat.ID, t, true
}

// NextOffset returns update_id + 1 for acknowledged polling (or 0 if none).
func NextOffset(updates []Update) int64 {
	var max int64
	for _, u := range updates {
		if u.UpdateID > max {
			max = u.UpdateID
		}
	}
	if max == 0 {
		return 0
	}
	return max + 1
}
