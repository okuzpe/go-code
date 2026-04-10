package skills

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// Meta holds optional YAML frontmatter from a SKILL.md (Claude-style skills).
type Meta struct {
	Name          string   `yaml:"name"`
	Description   string   `yaml:"description"`
	AllowedTools  []string `yaml:"allowed-tools"`
	AllowedTools2 []string `yaml:"allowed_tools"` // alternate key
}

// EffectiveAllowedTools merges allowed-tools and allowed_tools.
func (m Meta) EffectiveAllowedTools() []string {
	if len(m.AllowedTools) > 0 {
		return m.AllowedTools
	}
	return m.AllowedTools2
}

// ParseFrontmatter splits leading `---` / `---` YAML from the markdown body.
// If the file does not start with a frontmatter fence or YAML is invalid, it returns hasMeta=false
// and body is the full trimmed content (caller may still use it).
func ParseFrontmatter(raw []byte) (meta Meta, body string, hasMeta bool) {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return Meta{}, "", false
	}
	lines := strings.Split(s, "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" {
		return Meta{}, s, false
	}
	var yamlLines []string
	i := 1
	for ; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			break
		}
		yamlLines = append(yamlLines, lines[i])
	}
	if i >= len(lines) {
		return Meta{}, s, false
	}
	yamlBlock := strings.Join(yamlLines, "\n")
	if err := yaml.Unmarshal([]byte(yamlBlock), &meta); err != nil {
		return Meta{}, s, false
	}
	body = strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
	return meta, body, true
}

// formatSkillChunk builds the prompt fragment for one skill file.
func formatSkillChunk(path string, meta Meta, body string, maxBodyRunes int) string {
	var b strings.Builder
	b.WriteString("### ")
	b.WriteString(path)
	b.WriteByte('\n')
	if meta.Name != "" {
		b.WriteString("**name:** ")
		b.WriteString(meta.Name)
		b.WriteByte('\n')
	}
	if meta.Description != "" {
		b.WriteString("**description:** ")
		b.WriteString(meta.Description)
		b.WriteByte('\n')
	}
	if t := meta.EffectiveAllowedTools(); len(t) > 0 {
		b.WriteString("**allowed-tools:** ")
		b.WriteString(strings.Join(t, ", "))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	if body != "" {
		rs := []rune(body)
		if len(rs) > maxBodyRunes {
			body = string(rs[:maxBodyRunes]) + "\n…(truncated)"
		}
		b.WriteString(body)
	}
	b.WriteString("\n\n")
	return b.String()
}
