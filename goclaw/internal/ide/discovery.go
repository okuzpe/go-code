// Package ide holds optional local editor integration (D21).
package ide

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/okuzpe/goclaw/internal/mcp"
)

// Lockfile is a minimal JSON descriptor for an IDE MCP endpoint.
// Convention: place files under ~/.goclaw/ide/*.json (see docs/reference/ide-bridge.md §6).
type Lockfile struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// DiscoverMCPEndpoint reads sorted *.json files in dir and returns the first
// entry with a loopback MCP URL. Non-loopback URLs are skipped unless you
// configure the server manually in mcp_servers with mcp_allow_remote_urls.
func DiscoverMCPEndpoint(dir string) (string, map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil, fmt.Errorf("ide discovery: %w", err)
		}
		return "", nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(strings.ToLower(n), ".json") {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return "", nil, fmt.Errorf("ide discovery: no json lockfiles in %s", dir)
	}
	sort.Strings(names)
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var lf Lockfile
		if err := json.Unmarshal(raw, &lf); err != nil {
			continue
		}
		uStr := strings.TrimSpace(lf.URL)
		if uStr == "" {
			continue
		}
		u, err := url.Parse(uStr)
		if err != nil {
			continue
		}
		if err := mcp.ValidateHTTPURL(u, false); err != nil {
			continue
		}
		return uStr, lf.Headers, nil
	}
	return "", nil, fmt.Errorf("ide discovery: no valid lockfile in %s", dir)
}
