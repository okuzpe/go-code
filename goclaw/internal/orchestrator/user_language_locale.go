package orchestrator

import (
	"os"
	"strings"
)

// localePrimaryTagFromEnv returns es|en|fr|de|pt from the first usable locale variable, else "".
// Order: LC_ALL, LC_MESSAGES, LANG (common on Unix and Git Bash on Windows).
func localePrimaryTagFromEnv() string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" || v == "C" || strings.EqualFold(v, "POSIX") {
			continue
		}
		base := strings.Split(v, ".")[0]
		base = strings.Split(base, "@")[0]
		if i := strings.IndexByte(base, '_'); i >= 0 {
			base = base[:i]
		}
		base = strings.ToLower(base)
		switch base {
		case "es", "en", "fr", "de", "pt":
			return base
		}
	}
	return ""
}
