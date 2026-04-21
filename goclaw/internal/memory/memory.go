// Package memory manages persistent facts across sessions (user / feedback / project / reference).
package memory

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/okuzpe/goclaw/internal/text"
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
	Filename    string    // base name on disk, set by Load/List
	ExpiresAt   time.Time // zero = never expires; set via "expires_at: YYYY-MM-DD" in frontmatter
}

// IsExpired reports whether the entry has a non-zero expiry that is in the past.
func (e Entry) IsExpired() bool {
	return !e.ExpiresAt.IsZero() && time.Now().After(e.ExpiresAt)
}

// entryWithMtime pairs an entry with the file's modification time for decay scoring.
type entryWithMtime struct {
	entry Entry
	mtime time.Time
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

// List returns all non-expired entries sorted by modification time (newest first).
func (st *Store) List() ([]Entry, error) {
	withTimes, err := st.listWithMtime()
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(withTimes))
	for _, w := range withTimes {
		out = append(out, w.entry)
	}
	return out, nil
}

// listWithMtime returns all non-expired entries paired with their file mtime, newest first.
func (st *Store) listWithMtime() ([]entryWithMtime, error) {
	dirEntries, err := os.ReadDir(st.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("memory store: readdir: %w", err)
	}
	tmp := make([]entryWithMtime, 0, len(dirEntries))
	for _, de := range dirEntries {
		name := de.Name()
		if !strings.HasSuffix(name, ".md") || name == "MEMORY.md" {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(st.dir, name))
		if err != nil {
			continue
		}
		e, err := parseEntryFile(string(raw))
		if err != nil {
			continue
		}
		if e.IsExpired() {
			continue
		}
		e.Filename = name
		tmp = append(tmp, entryWithMtime{entry: e, mtime: info.ModTime()})
	}
	sort.Slice(tmp, func(i, j int) bool {
		return tmp[i].mtime.After(tmp[j].mtime)
	})
	return tmp, nil
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

// RelevantContext returns up to max entries ranked by BM25 against query,
// with an age-decay penalty applied to older entries.
// Only entries scoring at or above minScore (0.0–1.0, normalized) after decay are included.
// When query is empty the function falls back to recency order (same as RecentContext).
func (st *Store) RelevantContext(max int, query string, minScore float64) (string, error) {
	if max <= 0 {
		return "", nil
	}
	withTimes, err := st.listWithMtime()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(query) == "" {
		return formatEntryBullets(recencySlice(withTimes, max)), nil
	}

	// Build BM25 corpus from all entry texts.
	docTokens := make([][]string, len(withTimes))
	for i, w := range withTimes {
		entryText := w.entry.Name + " " + w.entry.Description + " " + w.entry.Body
		docTokens[i] = tokenizeTerms(entryText)
	}
	corpus := newBM25Corpus(docTokens)
	queryTerms := uniqueTerms(tokenizeTerms(query))
	if len(queryTerms) == 0 {
		return formatEntryBullets(recencySlice(withTimes, max)), nil
	}

	// Score every document; track the max to normalize scores to [0, 1].
	rawScores := make([]float64, len(withTimes))
	maxRaw := 0.0
	for i := range withTimes {
		s := corpus.score(i, queryTerms)
		rawScores[i] = s
		if s > maxRaw {
			maxRaw = s
		}
	}

	now := time.Now()
	type scoredEntry struct {
		entry Entry
		score float64
	}
	scored := make([]scoredEntry, 0, len(withTimes))
	for i, w := range withTimes {
		if rawScores[i] == 0 || maxRaw == 0 {
			continue
		}
		normalized := rawScores[i] / maxRaw // [0, 1] — best match is always 1.0
		decayed := applyAgeDecay(normalized, w.mtime, now, w.entry.Type)
		if decayed >= minScore {
			scored = append(scored, scoredEntry{entry: w.entry, score: decayed})
		}
	}
	// Stable sort by score descending; recency (list order) breaks ties.
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

// recencySlice returns at most max entries from withTimes in recency order (newest first).
// It is the fallback path when a query is empty or reduces to only stop words.
func recencySlice(withTimes []entryWithMtime, max int) []Entry {
	out := make([]Entry, 0, len(withTimes))
	for _, w := range withTimes {
		out = append(out, w.entry)
	}
	if len(out) > max {
		out = out[:max]
	}
	return out
}

// applyAgeDecay multiplies a relevance score by a decay factor based on how old the entry is.
// Decay rate varies by type: project entries decay faster (2-week half-life) since they go stale,
// while feedback and user entries decay slowly (12-week half-life) as they remain useful longer.
// Reference entries do not decay (they point to external systems that don't expire).
func applyAgeDecay(score float64, mtime, now time.Time, entryType Type) float64 {
	if score == 0 {
		return 0
	}
	age := now.Sub(mtime)
	if age <= 0 {
		return score
	}
	weeks := age.Hours() / (7 * 24)

	var halfLifeWeeks float64
	switch entryType {
	case TypeProject:
		halfLifeWeeks = 2 // project facts go stale quickly
	case TypeReference:
		return score // reference entries point to external systems; no decay
	case TypeUser:
		halfLifeWeeks = 16 // user preferences change slowly
	default: // TypeFeedback and unknown
		halfLifeWeeks = 12
	}

	// Exponential decay: score * 0.5^(age/halfLife). Floor at 0.1 so old entries are
	// never completely invisible — they can still surface if they're the only match.
	decay := math.Pow(0.5, weeks/halfLifeWeeks)
	if decay < 0.1 {
		decay = 0.1
	}
	return score * decay
}

var memoryStopWords = map[string]struct{}{
	// English — articles, pronouns, auxiliaries, prepositions, conjunctions
	"the": {}, "and": {}, "for": {}, "with": {}, "that": {},
	"this": {}, "are": {}, "was": {}, "has": {}, "have": {},
	"from": {}, "not": {}, "but": {}, "you": {}, "can": {},
	"its": {}, "will": {}, "would": {}, "should": {}, "could": {},
	"been": {}, "being": {}, "were": {}, "had": {}, "did": {},
	"does": {}, "into": {}, "over": {}, "also": {}, "just": {},
	"more": {}, "than": {}, "when": {}, "then": {}, "they": {},
	"them": {}, "their": {}, "there": {}, "what": {}, "which": {},
	"who": {}, "how": {}, "all": {}, "each": {}, "any": {},
	"some": {}, "such": {}, "use": {}, "used": {}, "using": {},
	"may": {}, "must": {}, "need": {}, "make": {}, "made": {},
	"only": {}, "very": {}, "well": {}, "even": {}, "here": {},
	"these": {}, "those": {}, "about": {}, "after": {}, "before": {},
	// Spanish function words — won't appear in normal English text
	"los": {}, "las": {}, "del": {}, "por": {}, "con": {},
	"para": {}, "una": {}, "como": {}, "sus": {}, "les": {},
	"hay": {}, "cuando": {}, "donde": {}, "entre": {}, "sobre": {},
	"hasta": {}, "desde": {}, "antes": {}, "también": {}, "porque": {},
	"aunque": {}, "cada": {}, "todos": {}, "todas": {}, "muy": {},
	"bien": {}, "entonces": {}, "ahora": {}, "este": {}, "esta": {},
	"ese": {}, "esa": {}, "eso": {}, "nos": {}, "más": {},
}

func isStopWord(word string) bool {
	_, ok := memoryStopWords[word]
	return ok
}

const entryBodySnippetMaxRunes = 220 // max runes of body shown per entry in context injection

// formatEntryBullets renders a bullet list from a slice of entries, including a truncated
// body snippet so the model sees the actual content (why / how to apply) not just the title.
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
		if body := strings.TrimSpace(e.Body); body != "" {
			snippet := text.TruncateRunes(body, entryBodySnippetMaxRunes)
			// Indent each body line by two spaces for readability.
			for line := range strings.SplitSeq(snippet, "\n") {
				sb.WriteString("  ")
				sb.WriteString(line)
				sb.WriteByte('\n')
			}
		}
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
	if nBytes <= 0 {
		return "deadbeef"
	}
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
	if !e.ExpiresAt.IsZero() {
		sb.WriteString("\nexpires_at: ")
		sb.WriteString(e.ExpiresAt.UTC().Format("2006-01-02"))
	}
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
	fm, body, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return Entry{}, fmt.Errorf("memory store: invalid frontmatter")
	}
	body = strings.TrimSpace(body)
	var e Entry
	for line := range strings.SplitSeq(fm, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, found := strings.Cut(line, ":")
		if !found {
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
		case "expires_at":
			if t, err := time.Parse("2006-01-02", v); err == nil {
				e.ExpiresAt = t.UTC()
			}
		}
	}
	e.Body = body
	return e, nil
}
