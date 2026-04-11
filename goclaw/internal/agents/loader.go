package agents

import (
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// customFrontmatter is the YAML structure parsed from a *.md agent file's frontmatter.
type customFrontmatter struct {
	Name          string   `yaml:"name"`
	Description   string   `yaml:"description"`
	ModelOverride string   `yaml:"model"`
	ToolAllowlist []string `yaml:"tool_allowlist"`
	ReadOnly      bool     `yaml:"read_only"`
	SystemPrompt  string   `yaml:"system_prompt"`
}

// validAgentName accepts lowercase letters, digits, and hyphens only.
var validAgentName = regexp.MustCompile(`^[a-z0-9-]+$`)

// LoadCustomProfiles scans dir for *.md files with YAML frontmatter and returns parsed Profile instances.
// Files without valid frontmatter are logged and skipped; errors from individual files do not abort the scan.
// Returns an empty slice (not an error) if dir does not exist.
func LoadCustomProfiles(dir string) ([]Profile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var profiles []Profile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		profile, ok := parseAgentFile(path)
		if !ok {
			continue
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

// parseAgentFile reads a single *.md file, extracts frontmatter, and returns a Profile.
// Returns false if the file cannot be parsed or the profile is invalid.
func parseAgentFile(path string) (Profile, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("custom agent: failed to read file", "path", path, "err", err)
		return Profile{}, false
	}

	content := string(raw)
	frontmatter, body, ok := splitFrontmatter(content)
	if !ok {
		slog.Warn("custom agent: no YAML frontmatter found (must start with ---)", "path", path)
		return Profile{}, false
	}

	var fm customFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
		slog.Warn("custom agent: invalid YAML frontmatter", "path", path, "err", err)
		return Profile{}, false
	}

	name := strings.TrimSpace(fm.Name)
	if name == "" {
		slog.Warn("custom agent: missing 'name' field in frontmatter", "path", path)
		return Profile{}, false
	}
	if !validAgentName.MatchString(name) {
		slog.Warn("custom agent: name must match [a-z0-9-]+", "path", path, "name", name)
		return Profile{}, false
	}

	// Combine system_prompt from frontmatter and body text.
	systemPrompt := strings.TrimSpace(fm.SystemPrompt)
	if bodyText := strings.TrimSpace(body); bodyText != "" {
		if systemPrompt != "" {
			systemPrompt += "\n\n" + bodyText
		} else {
			systemPrompt = bodyText
		}
	}

	profile := Profile{
		Name:          name,
		Description:   strings.TrimSpace(fm.Description),
		ModelOverride: strings.TrimSpace(fm.ModelOverride),
		ToolAllowlist: fm.ToolAllowlist,
		ReadOnly:      fm.ReadOnly,
		SystemPrompt:  systemPrompt,
	}
	slog.Debug("custom agent loaded", "name", name, "path", path)
	return profile, true
}

// splitFrontmatter splits a Markdown file into YAML frontmatter and body.
// The file must start with "---\n"; the second "---" closes the frontmatter block.
// Returns (frontmatter, body, true) on success; ("", "", false) if no valid frontmatter is found.
func splitFrontmatter(content string) (string, string, bool) {
	const delimiter = "---"
	// Must start with "---" (optionally followed by \r\n or \n).
	if !strings.HasPrefix(content, delimiter) {
		return "", "", false
	}
	// Find the end of the first "---" line.
	firstEnd := strings.IndexByte(content, '\n')
	if firstEnd < 0 {
		return "", "", false
	}
	rest := content[firstEnd+1:]

	// Find the closing "---".
	closing := strings.Index(rest, "\n"+delimiter)
	if closing < 0 {
		// Try at start of rest (edge case: frontmatter immediately closed).
		if !strings.HasPrefix(rest, delimiter) {
			return "", "", false
		}
		closing = 0
	}

	frontmatter := rest[:closing]
	afterDelimiter := rest[closing+1+len(delimiter):]
	// Skip the rest of the closing "---" line.
	bodyStart := strings.IndexByte(afterDelimiter, '\n')
	var body string
	if bodyStart >= 0 {
		body = afterDelimiter[bodyStart+1:]
	}
	return frontmatter, body, true
}

// AllWithCustom returns built-in profiles merged with user and project custom profiles.
// Priority: project > user > built-in (later wins for the same name).
func AllWithCustom(userAgentsDir, projectAgentsDir string) (map[string]Profile, error) {
	result := All()

	userProfiles, err := LoadCustomProfiles(userAgentsDir)
	if err != nil {
		slog.Warn("custom agent: failed to scan user agents dir", "dir", userAgentsDir, "err", err)
	}
	for _, profile := range userProfiles {
		result[profile.Name] = profile
	}

	projectProfiles, err := LoadCustomProfiles(projectAgentsDir)
	if err != nil {
		slog.Warn("custom agent: failed to scan project agents dir", "dir", projectAgentsDir, "err", err)
	}
	for _, profile := range projectProfiles {
		result[profile.Name] = profile
	}

	return result, nil
}
