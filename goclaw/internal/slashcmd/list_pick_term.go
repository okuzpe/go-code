package slashcmd

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"golang.org/x/term"
)

// ttyListPickResult is the outcome of pickListTTY.
type ttyListPickResult int

const (
	ttyListPickNone ttyListPickResult = iota
	ttyListPickChosen
	ttyListPickCancelled
)

// pickListTTY runs a minimal arrow-key menu on stdin/stdout when fd is a terminal.
// header is printed once above the list (e.g. "Choose — ↑↓ · Enter · Esc").
func pickListTTY(fd int, in io.Reader, out io.Writer, header string, items []string, startIdx int) (chosen string, result ttyListPickResult, err error) {
	if !term.IsTerminal(fd) || len(items) == 0 {
		return "", ttyListPickNone, nil
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		return "", ttyListPickNone, err
	}
	defer func() {
		_ = term.Restore(fd, old)
		_, _ = fmt.Fprintln(out)
	}()

	cursor := startIdx
	if cursor < 0 || cursor >= len(items) {
		cursor = 0
	}
	refresh := func() {
		_, _ = fmt.Fprint(out, "\r\x1b[2K\x1b[J")
		if strings.TrimSpace(header) != "" {
			_, _ = fmt.Fprintln(out, header)
		}
		for i, name := range items {
			if i == cursor {
				_, _ = fmt.Fprintf(out, "  ▸ %s\r\n", name)
			} else {
				_, _ = fmt.Fprintf(out, "    %s\r\n", name)
			}
		}
	}
	refresh()

	br := bufio.NewReader(in)
	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return "", ttyListPickCancelled, nil
			}
			return "", ttyListPickNone, err
		}
		switch b {
		case 3: // Ctrl+C
			return "", ttyListPickCancelled, nil
		case 13, 10:
			return items[cursor], ttyListPickChosen, nil
		case 27:
			next, err := br.ReadByte()
			if err != nil {
				if err == io.EOF {
					return "", ttyListPickCancelled, nil
				}
				return "", ttyListPickNone, err
			}
			if next == '[' {
				dir, err := br.ReadByte()
				if err != nil {
					if err == io.EOF {
						return "", ttyListPickCancelled, nil
					}
					return "", ttyListPickNone, err
				}
				switch dir {
				case 'A':
					cursor = (cursor - 1 + len(items)) % len(items)
					refresh()
				case 'B':
					cursor = (cursor + 1) % len(items)
					refresh()
				}
				continue
			}
			return "", ttyListPickCancelled, nil
		}
	}
}
