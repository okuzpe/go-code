package app

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

func cloneHeaderMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	return maps.Clone(m)
}

func headerHasAuthorization(m map[string]string) bool {
	for k := range m {
		if strings.EqualFold(k, "Authorization") {
			return true
		}
	}
	return false
}

func readBearerTokenFile(workdir, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty bearer_token_file")
	}
	full := path
	if !filepath.IsAbs(path) {
		full = filepath.Join(workdir, path)
	}
	full = filepath.Clean(full)
	b, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("read bearer_token_file %s: %w", full, err)
	}
	return strings.TrimSpace(string(b)), nil
}
