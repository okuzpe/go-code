package ui

import "strings"

var demoCommands = []string{"clear", "help", "quit"}

// TabExpand replaces the trimmed whole-buffer text when it is a strict prefix of exactly one demo command.
func TabExpand(buffer string) (newText string, ok bool) {
	t := strings.TrimSpace(buffer)
	if t == "" || strings.Contains(buffer, "\n") {
		return "", false
	}
	var match string
	n := 0
	for _, c := range demoCommands {
		if strings.HasPrefix(c, t) {
			match = c
			n++
		}
	}
	if n != 1 || match == t {
		return "", false
	}
	return match, true
}
