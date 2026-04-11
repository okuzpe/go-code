package orchestrator

import (
	"strings"

	"github.com/abadojack/whatlanggo"
)

// whatlanggoToTag maps a reliable whatlanggo detection to es|en|fr|de|pt, else "".
func whatlanggoToTag(text string) string {
	t := strings.TrimSpace(text)
	if len(t) < 8 {
		return ""
	}
	info := whatlanggo.Detect(t)
	if !info.IsReliable() {
		return ""
	}
	switch info.Lang {
	case whatlanggo.Spa:
		return "es"
	case whatlanggo.Eng:
		return "en"
	case whatlanggo.Fra:
		return "fr"
	case whatlanggo.Deu:
		return "de"
	case whatlanggo.Por:
		return "pt"
	default:
		return ""
	}
}
