package tools

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const minHTMLPlainTextWords = 5

// isLikelyHTMLResponse returns true when the Content-Type or body sniff suggests HTML.
func isLikelyHTMLResponse(contentType string, body []byte) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml") {
		return true
	}
	snip := body
	if len(snip) > 4096 {
		snip = snip[:4096]
	}
	low := strings.ToLower(string(snip))
	return strings.Contains(low, "<!doctype html") ||
		strings.Contains(low, "<html") ||
		(strings.Contains(low, "<head") && strings.Contains(low, "<body"))
}

// htmlResponseToPlainText parses HTML and returns condensed plain text suitable for LLM context.
// The second return is false when parsing fails or the extracted text is too short (caller should use raw body).
func htmlResponseToPlainText(body []byte) (string, bool) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", false
	}
	var out strings.Builder
	needSpaceBeforeText := false
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			t := strings.TrimSpace(n.Data)
			if t == "" {
				return
			}
			if needSpaceBeforeText {
				out.WriteByte(' ')
			}
			out.WriteString(t)
			needSpaceBeforeText = true
		case html.ElementNode:
			switch n.DataAtom {
			case atom.Script, atom.Style, atom.Noscript, atom.Template:
				return
			case atom.Br:
				out.WriteByte('\n')
				needSpaceBeforeText = false
				return
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			if isBlockHTMLAtom(n.DataAtom) {
				out.WriteByte('\n')
				needSpaceBeforeText = false
			}
		default:
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
	}
	walk(doc)
	plain := normalizeFetchedWhitespace(out.String())
	if len(strings.Fields(plain)) < minHTMLPlainTextWords {
		return "", false
	}
	return plain, true
}

func isBlockHTMLAtom(a atom.Atom) bool {
	switch a {
	case atom.P, atom.Div, atom.Li, atom.Tr, atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6,
		atom.Blockquote, atom.Pre, atom.Title, atom.Header, atom.Footer, atom.Section, atom.Article,
		atom.Nav, atom.Aside, atom.Ul, atom.Ol, atom.Table, atom.Tbody, atom.Thead, atom.Tfoot,
		atom.Hgroup, atom.Form, atom.Hr, atom.Dl, atom.Dd, atom.Dt, atom.Main, atom.Figcaption:
		return true
	default:
		return false
	}
}

func normalizeFetchedWhitespace(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.Join(strings.Fields(lines[i]), " ")
	}
	var b strings.Builder
	prevBlank := false
	for _, line := range lines {
		if line == "" {
			if !prevBlank {
				b.WriteByte('\n')
				prevBlank = true
			}
			continue
		}
		prevBlank = false
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	return strings.TrimSpace(b.String())
}
