package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func cloneHeaderMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
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
	b, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
