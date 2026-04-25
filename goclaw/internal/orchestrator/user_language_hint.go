package orchestrator

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/okuzpe/goclaw/internal/config"
	"github.com/okuzpe/goclaw/internal/llm"
)

const runtimeUserLanguageHintHeader = "\n\n## Runtime user-language hint\n"

// LastUserNaturalText returns the most recent user-authored plain text turn (not a tool-result
// payload), walking backward from the end of the session. Synthetic runtime nudge lines
// injected by the orchestrator are skipped so callers see the real user request.
func LastUserNaturalText(msgs []llm.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if !strings.EqualFold(m.Role, "user") {
			continue
		}
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		if len(m.ToolResults) > 0 {
			continue
		}
		if isSyntheticRuntimeNudgeContent(m.Content) {
			continue
		}
		return m.Content
	}
	return ""
}

func isSyntheticRuntimeNudgeContent(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "[goclaw] The user asked for concrete code improvements") ||
		strings.HasPrefix(t, "[goclaw] The user asked for code or repository changes") ||
		strings.HasPrefix(t, "[goclaw] Action nudges were exhausted without native tool calls") ||
		strings.HasPrefix(t, "[goclaw] You made workspace changes.") ||
		strings.HasPrefix(t, "[goclaw] edit_file failed: old_string not found.") ||
		strings.HasPrefix(t, "[goclaw] Path recovery is pending") ||
		strings.HasPrefix(t, "[goclaw] Reflection checkpoint:")
}

func lastUserNaturalText(msgs []llm.Message) string {
	return LastUserNaturalText(msgs)
}

// userLanguageSystemSuffix appends a short English-meta hint so local models align reply language.
// cfg.PreferredResponseLanguage: auto | from_os | es | en | fr | de | pt (see config.NormalizePreferredResponseLanguage).
func userLanguageSystemSuffix(latestUserText string, cfg config.Config) string {
	latestUserText = strings.TrimSpace(latestUserText)
	pref := config.NormalizePreferredResponseLanguage(cfg.PreferredResponseLanguage)

	if latestUserText != "" && looksLikeStructuredToolInput(latestUserText) {
		return ""
	}

	if pref != "auto" && pref != "from_os" {
		if latestUserText == "" {
			return ""
		}
		return hintForPreferredTag(pref)
	}

	if latestUserText == "" {
		return ""
	}

	tag := classifyUserLanguage(latestUserText)
	if tag == "" {
		tag = whatlanggoToTag(latestUserText)
	}
	if tag == "" && pref == "from_os" {
		tag = localePrimaryTagFromEnv()
	}

	return hintForDetectedTag(tag)
}

func hintForPreferredTag(tag string) string {
	switch tag {
	case "es":
		return runtimeUserLanguageHintHeader +
			"Preferred response language (goclaw settings): Spanish. Use Spanish for conversational prose unless RESPONSE LANGUAGE says otherwise."
	case "en":
		return runtimeUserLanguageHintHeader +
			"Preferred response language (goclaw settings): English. Use English for conversational prose unless RESPONSE LANGUAGE says otherwise."
	case "fr":
		return runtimeUserLanguageHintHeader +
			"Preferred response language (goclaw settings): French. Use French for conversational prose unless RESPONSE LANGUAGE says otherwise."
	case "de":
		return runtimeUserLanguageHintHeader +
			"Preferred response language (goclaw settings): German. Use German for conversational prose unless RESPONSE LANGUAGE says otherwise."
	case "pt":
		return runtimeUserLanguageHintHeader +
			"Preferred response language (goclaw settings): Portuguese. Use Portuguese for conversational prose unless RESPONSE LANGUAGE says otherwise."
	default:
		return ""
	}
}

func hintForDetectedTag(tag string) string {
	switch tag {
	case "es":
		return runtimeUserLanguageHintHeader +
			"The latest user-written message reads as Spanish. Reply in Spanish for all conversational prose (per RESPONSE LANGUAGE)."
	case "en":
		return runtimeUserLanguageHintHeader +
			"The latest user-written message reads as English. Reply in English for all conversational prose (per RESPONSE LANGUAGE)."
	case "fr":
		return runtimeUserLanguageHintHeader +
			"The latest user-written message reads as French. Reply in French for all conversational prose (per RESPONSE LANGUAGE)."
	case "de":
		return runtimeUserLanguageHintHeader +
			"The latest user-written message reads as German. Reply in German for all conversational prose (per RESPONSE LANGUAGE)."
	case "pt":
		return runtimeUserLanguageHintHeader +
			"The latest user-written message reads as Portuguese. Reply in Portuguese for all conversational prose (per RESPONSE LANGUAGE)."
	default:
		return ""
	}
}

// langReplyHint returns the system-prompt language hint to inject.
// When turnInputLang is non-empty (set before input translation), it takes priority over
// detecting the language from session text — which would yield "en" after translation.
// Falls back to the full userLanguageSystemSuffix logic when no pre-detected tag is available.
func langReplyHint(turnInputLang string, cfg config.Config, msgs []llm.Message) string {
	if turnInputLang != "" {
		pref := config.NormalizePreferredResponseLanguage(cfg.PreferredResponseLanguage)
		if pref != "auto" && pref != "from_os" {
			return hintForPreferredTag(pref)
		}
		return hintForDetectedTag(turnInputLang)
	}
	return userLanguageSystemSuffix(lastUserNaturalText(msgs), cfg)
}

func looksLikeStructuredToolInput(text string) bool {
	t := strings.TrimSpace(text)
	if len(t) > 800 {
		return false
	}
	return strings.HasPrefix(t, "{") && strings.Contains(t, `"`)
}

// classifyUserLanguage returns a BCP-47-style short tag when heuristic confidence is enough, else "".
func classifyUserLanguage(text string) string {
	lower := strings.ToLower(text)
	norm := normalizeWordSpans(lower)

	scores := map[string]int{
		"es": scoreSpanish(text, lower, norm),
		"fr": scoreFrench(lower, norm),
		"de": scoreGerman(lower, norm),
		"pt": scorePortuguese(lower, norm),
	}

	maxTag := ""
	maxVal := 0
	for t, v := range scores {
		if v > maxVal {
			maxVal = v
			maxTag = t
		}
	}
	if maxVal < 3 {
		return ""
	}
	tied := 0
	for _, v := range scores {
		if v == maxVal {
			tied++
		}
	}
	if tied > 1 {
		return ""
	}
	return maxTag
}

func normalizeWordSpans(lower string) string {
	var b strings.Builder
	b.WriteByte(' ')
	for _, r := range lower {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	b.WriteByte(' ')
	fields := strings.Fields(b.String())
	return " " + strings.Join(fields, " ") + " "
}

func scoreSpanish(text, lower, norm string) int {
	n := 0
	for _, r := range text {
		switch r {
		case 'ñ', 'Ñ':
			n += 4
		case '¿', '¡':
			n += 4
		case 'á', 'é', 'í', 'ó', 'ú', 'ü', 'Á', 'É', 'Í', 'Ó', 'Ú', 'Ü':
			n += 2
		}
	}
	spanishWords := []string{
		"hola", "gracias", "buenos", "buenas", "adiós", "por favor",
		"cómo", "qué", "dónde", "cuándo", "porque", "también", "está", "están", "esto", "esta",
		"señor", "señora", "mucho", "gusto", "hablar", "necesito", "puedes", "favor",
		"ayuda", "bienvenido", "usuario",
	}
	for _, w := range spanishWords {
		if strings.Contains(norm, " "+w+" ") {
			n += 2
		}
	}
	if strings.Contains(lower, "por favor") || strings.Contains(lower, "por qué") || strings.Contains(lower, "porque") {
		n += 2
	}
	// Common spoken Spanish without accents (models/users often omit tildes).
	if strings.Contains(norm, " que tal ") || strings.Contains(norm, " qué tal ") {
		n += 4
	}
	if strings.Contains(norm, " como estas ") || strings.Contains(norm, " cómo estás ") {
		n += 3
	}
	if utf8.RuneCountInString(strings.TrimSpace(lower)) <= 6 && norm == " hola " {
		n += 4
	}
	return n
}

func scoreFrench(lower, norm string) int {
	n := 0
	for _, r := range lower {
		switch r {
		case 'à', 'â', 'ç', 'è', 'é', 'ê', 'ë', 'î', 'ï', 'ô', 'ù', 'û', 'œ':
			n += 2
		}
	}
	words := []string{"bonjour", "merci", "madame", "monsieur", "salut", "français", "comment", "aujourd"}
	for _, w := range words {
		if strings.Contains(norm, " "+w+" ") {
			n += 2
		}
	}
	if strings.Contains(lower, "s'il") || strings.Contains(lower, "s il") {
		n += 2
	}
	return n
}

func scoreGerman(lower, norm string) int {
	n := 0
	for _, r := range lower {
		switch r {
		case 'ä', 'ö', 'ü', 'ß':
			n += 3
		}
	}
	words := []string{"hallo", "danke", "bitte", "guten", "schön", "können", "müssen", "über"}
	for _, w := range words {
		if strings.Contains(norm, " "+w+" ") {
			n += 2
		}
	}
	return n
}

func scorePortuguese(lower, norm string) int {
	n := 0
	for _, r := range lower {
		switch r {
		case 'ã', 'õ':
			n += 4
		case 'ç':
			n += 1
		}
	}
	words := []string{"olá", "obrigado", "obrigada", "não", "você", "também", "português"}
	for _, w := range words {
		if strings.Contains(norm, " "+w+" ") {
			n += 3
		}
	}
	if strings.Contains(norm, " ola ") && !strings.Contains(norm, " hola ") {
		n += 2
	}
	return n
}
