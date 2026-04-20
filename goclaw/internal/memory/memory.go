// Package memory manages persistent facts across sessions (user / feedback / project / reference).
package memory

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Type classifies a memory entry.
type Type string

const (
	TypeUser      Type = "user"
	TypeFeedback  Type = "feedback"
	TypeProject   Type = "project"
	TypeReference Type = "reference"

	memorySanitizedNameMaxRunes = 48
	memoryRandomSuffixBytes     = 8 // hex length 2*bytes in Save() basename
)

// Entry is a single memory record stored as a markdown file with YAML frontmatter.
type Entry struct {
	Name        string
	Description string
	Type        Type
	Body        string
	Filename    string // base name on disk, set by Load/List
}

// Store is a filesystem-backed memory store rooted at a directory.
type Store struct {
	dir string
}

var safeNameRE = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// New returns a Store pointing at dir. Callers should create dir with 0o700 if needed.
func New(dir string) *Store {
	return &Store{dir: dir}
}

// Save writes entry as a new file under the store directory (0o600). Returns the basename used.
func (st *Store) Save(e Entry) (string, error) {
	if err := os.MkdirAll(st.dir, 0o700); err != nil {
		return "", fmt.Errorf("memory store: mkdir: %w", err)
	}
	base := sanitizeBaseName(e.Name) + "_" + randomHex(memoryRandomSuffixBytes) + ".md"
	path := filepath.Join(st.dir, base)
	data := formatEntryFile(e)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		return "", fmt.Errorf("memory store: write %s: %w", base, err)
	}
	if err := WriteIndex(st); err != nil {
		slog.Warn("memory: index write failed", "err", err)
	}
	return base, nil
}

func invalidMemoryBasename(basename string) bool {
	return strings.Contains(basename, string(os.PathSeparator)) || basename == "." || basename == ".."
}

// Load reads one entry by file basename (e.g. from List).
func (st *Store) Load(basename string) (*Entry, error) {
	if invalidMemoryBasename(basename) {
		return nil, fmt.Errorf("memory store: invalid basename")
	}
	path := filepath.Join(st.dir, basename)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("memory store: read %s: %w", basename, err)
	}
	e, err := parseEntryFile(string(raw))
	if err != nil {
		return nil, err
	}
	e.Filename = basename
	return &e, nil
}

// List returns all entries sorted by modification time (newest first).
func (st *Store) List() ([]Entry, error) {
	entries, err := os.ReadDir(st.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("memory store: readdir: %w", err)
	}
	type withTime struct {
		e Entry
		t int64
	}
	tmp := make([]withTime, 0, len(entries))
	for _, ent := range entries {
		name := ent.Name()
		if !strings.HasSuffix(name, ".md") || name == "MEMORY.md" {
			continue
		}
		path := filepath.Join(st.dir, name)
		info, err := ent.Info()
		if err != nil {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		e, err := parseEntryFile(string(raw))
		if err != nil {
			continue
		}
		e.Filename = name
		tmp = append(tmp, withTime{e: e, t: info.ModTime().UnixNano()})
	}
	sort.Slice(tmp, func(i, j int) bool { return tmp[i].t > tmp[j].t })
	out := make([]Entry, 0, len(tmp))
	for _, w := range tmp {
		out = append(out, w.e)
	}
	return out, nil
}

// Delete removes one entry file by basename.
func (st *Store) Delete(basename string) error {
	if invalidMemoryBasename(basename) {
		return fmt.Errorf("memory store: invalid basename")
	}
	path := filepath.Join(st.dir, basename)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("memory store: delete %s: %w", basename, err)
	}
	if err := WriteIndex(st); err != nil {
		slog.Warn("memory: index write failed", "err", err)
	}
	return nil
}

// RecentContext returns a short bullet list of the newest entries for system-prompt injection.
func (st *Store) RecentContext(max int) (string, error) {
	if max <= 0 {
		return "", nil
	}
	list, err := st.List()
	if err != nil {
		return "", err
	}
	if len(list) > max {
		list = list[:max]
	}
	return formatEntryBullets(list), nil
}

// RelevantContext returns up to max entries ranked by keyword overlap with query.
// Only entries scoring at or above minScore (0.0–1.0) are included.
// When query is empty the function falls back to recency order (same as RecentContext).
func (st *Store) RelevantContext(max int, query string, minScore float64) (string, error) {
	if max <= 0 {
		return "", nil
	}
	list, err := st.List()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(query) == "" {
		if len(list) > max {
			list = list[:max]
		}
		return formatEntryBullets(list), nil
	}
	queryTokens := tokenizeQuery(query)
	type scoredEntry struct {
		entry Entry
		score float64
	}
	scored := make([]scoredEntry, 0, len(list))
	for _, e := range list {
		s := entryRelevanceScore(e, queryTokens)
		if s >= minScore {
			scored = append(scored, scoredEntry{entry: e, score: s})
		}
	}
	// Stable sort by score descending; recency (from List) breaks ties.
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
	if len(scored) > max {
		scored = scored[:max]
	}
	out := make([]Entry, len(scored))
	for i, se := range scored {
		out[i] = se.entry
	}
	return formatEntryBullets(out), nil
}

// tokenizeQuery lowercases and splits s into unique tokens (3+ chars, stop words removed).
func tokenizeQuery(s string) map[string]struct{} {
	tokens := make(map[string]struct{})
	for _, word := range strings.Fields(strings.ToLower(s)) {
		word = strings.Trim(word, ".,!?;:\"'`()[]{}")
		if len(word) >= 3 && !isStopWord(word) {
			tokens[word] = struct{}{}
		}
	}
	return tokens
}

// entryRelevanceScore returns the fraction of query tokens found anywhere in the entry text.
func entryRelevanceScore(e Entry, queryTokens map[string]struct{}) float64 {
	if len(queryTokens) == 0 {
		return 0
	}
	corpus := strings.ToLower(e.Name + " " + e.Description + " " + e.Body)
	hits := 0
	for token := range queryTokens {
		if strings.Contains(corpus, token) {
			hits++
		}
	}
	return float64(hits) / float64(len(queryTokens))
}

var memoryStopWords = map[string]struct{}{
	"the": {}, "and": {}, "for": {}, "with": {}, "that": {},
	"this": {}, "are": {}, "was": {}, "has": {}, "have": {},
	"from": {}, "not": {}, "but": {}, "you": {}, "can": {},
	"its": {}, "will": {}, "would": {}, "should": {}, "could": {},
}

func isStopWord(word string) bool {
	_, ok := memoryStopWords[word]
	return ok
}

// formatEntryBullets renders a bullet list from a slice of entries.
// Extracted from RecentContext so both functions share the same rendering logic.
func formatEntryBullets(list []Entry) string {
	var sb strings.Builder
	for _, e := range list {
		sb.WriteString("- ")
		sb.WriteString(string(e.Type))
		sb.WriteString(" ")
		sb.WriteString(e.Name)
		if e.Description != "" {
			sb.WriteString(": ")
			sb.WriteString(e.Description)
		}
		sb.WriteByte('\n')
	}
	return strings.TrimSpace(sb.String())
}

func sanitizeBaseName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "note"
	}
	s = safeNameRE.ReplaceAllString(s, "_")
	r := []rune(s)
	if len(r) > memorySanitizedNameMaxRunes {
		s = string(r[:memorySanitizedNameMaxRunes])
	}
	return s
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "deadbeef"
	}
	return hex.EncodeToString(b)
}

func formatEntryFile(e Entry) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("type: ")
	sb.WriteString(string(e.Type))
	sb.WriteString("\nname: ")
	sb.WriteString(escapeYAMLLine(e.Name))
	sb.WriteString("\ndescription: ")
	sb.WriteString(escapeYAMLLine(e.Description))
	sb.WriteString("\n---\n")
	sb.WriteString(strings.TrimSpace(e.Body))
	if e.Body != "" && !strings.HasSuffix(e.Body, "\n") {
		sb.WriteString("\n")
	}
	return sb.String()
}

func escapeYAMLLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func parseEntryFile(data string) (Entry, error) {
	data = strings.TrimPrefix(data, "\ufeff")
	if !strings.HasPrefix(data, "---\n") {
		return Entry{}, fmt.Errorf("memory store: missing frontmatter")
	}
	rest := strings.TrimPrefix(data, "---\n")
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return Entry{}, fmt.Errorf("memory store: invalid frontmatter")
	}
	fm := rest[:end]
	body := strings.TrimSpace(rest[end+len("\n---\n"):])
	var e Entry
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch k {
		case "type":
			e.Type = Type(v)
		case "name":
			e.Name = v
		case "description":
			e.Description = v
		}
	}
	e.Body = body
	return e, nil
}
