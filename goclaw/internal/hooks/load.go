package hooks

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// LoadHooksFile reads `.goclaw/hooks.json` (array of hook entries) and registers them.
func LoadHooksFile(reg *Registry, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read hooks file: %w", err)
	}
	var entries []struct {
		Event   string   `json:"event"`
		Command string   `json:"command"`
		Args    []string `json:"args,omitempty"`
		URL     string   `json:"url,omitempty"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("parse hooks file: %w", err)
	}
	for _, e := range entries {
		et, err := ParseEventType(e.Event)
		if err != nil {
			slog.Warn("skip hook entry", "err", err)
			continue
		}
		if strings.TrimSpace(e.URL) != "" {
			reg.OnHTTP(et, strings.TrimSpace(e.URL), defaultHookHTTPTimeout)
			continue
		}
		if strings.TrimSpace(e.Command) != "" {
			reg.OnCommand(et, e.Command, e.Args...)
		}
	}
	return nil
}
