package commentfix

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/treecomments"
)

// isDirectiveLine reports a line a tool reads rather than a reader. The C family
// spells it with no space after the marker, and the hash family carries the
// interpreter line and the linter pragma.
func isDirectiveLine(line string) bool {
	if treecomments.IsDirective(line) || isLintPragma(line) {
		return true
	}
	t := strings.TrimSpace(line)
	for _, marker := range []string{"//", "#"} {
		rest, found := strings.CutPrefix(t, marker)
		if !found {
			continue
		}
		if marker == "#" && strings.HasPrefix(rest, "!") {
			return true // an interpreter line
		}
		name, _, hasColon := strings.Cut(rest, ":")
		if !hasColon || name == "" || strings.ContainsAny(name, " \t") {
			continue
		}
		// `//go:build` and `# shellcheck:` carry no space before the colon.
		return true
	}
	return false
}

// isLintPragma reports a line comment that a linter reads. A cut that drops one
// turns a suppressed warning back on.
func isLintPragma(line string) bool {
	_, ok := pragmaText(line)
	return ok
}

// pragmaWords are the pragmas that a linter or a type checker reads. Every word after the pragma is free text a person wrote.
var pragmaWords = []string{"eslint-disable-next-line", "eslint-disable-line", "eslint-disable", "eslint-enable", "@ts-ignore", "@ts-expect-error", "@ts-nocheck", "prettier-ignore"}

// pragmaPairs are the pragmas whose next word belongs to them.
var pragmaPairs = map[string][]string{"istanbul ignore": {"next", "if", "else", "file"}, "c8 ignore": {"next", "start", "stop"}}

// shellcheckKeys are the keys a shellcheck directive sets, as key=value words.
var shellcheckKeys = []string{"disable=", "enable=", "source=", "shell=", "external-sources="}

// pragmaText reports whether line is a lint pragma, and the byte offset where
// its free text starts. The free text is prose, so the length rule reads it.
// The offset is len(line) when the pragma carries no free text.
func pragmaText(line string) (int, bool) {
	t := strings.TrimLeft(line, " \t")
	for _, marker := range []string{"//", "#"} {
		rest, found := strings.CutPrefix(t, marker)
		if !found {
			continue
		}
		rest = strings.TrimLeft(rest, " \t")
		at := len(line) - len(rest)
		if end, ok := pragmaEnd(rest); ok {
			return at + end, true
		}
	}
	return 0, false
}

// pragmaEnd answers where the pragma that opens rest stops.
func pragmaEnd(rest string) (int, bool) {
	for _, word := range pragmaWords {
		if !opensWith(rest, word) {
			continue
		}
		// An eslint rule list runs up to the "--" that opens its description.
		if strings.HasPrefix(word, "eslint-") {
			if i := strings.Index(rest, " --"); i >= 0 {
				return i, true
			}
			return len(rest), true
		}
		return len(word), true
	}
	for lead, nexts := range pragmaPairs {
		for _, next := range nexts {
			if word := lead + " " + next; opensWith(rest, word) {
				return len(word), true
			}
		}
	}
	if after, ok := strings.CutPrefix(rest, "shellcheck "); ok {
		end := len(rest) - len(after)
		for _, field := range strings.Fields(after) {
			if !hasAnyPrefix(field, shellcheckKeys) {
				break
			}
			end = strings.Index(rest, field) + len(field)
		}
		return end, end > len("shellcheck ")
	}
	return 0, false
}

// opensWith reports whether rest opens with word, followed by a space or the end.
func opensWith(rest, word string) bool {
	after, ok := strings.CutPrefix(rest, word)
	return ok && (after == "" || after[0] == ' ' || after[0] == '\t')
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// pragmaProse answers the free text that each lint pragma line in text carries.
func pragmaProse(text []string) []string {
	var out []string
	for _, line := range text {
		if at, ok := pragmaText(line); ok && strings.TrimSpace(line[at:]) != "" {
			out = append(out, line[at:])
		}
	}
	return out
}

// withoutPragmaProse cuts the free text off each lint pragma line, and keeps the pragma.
func withoutPragmaProse(text []string) []string {
	out := make([]string, len(text))
	for i, line := range text {
		out[i] = line
		if at, ok := pragmaText(line); ok {
			out[i] = strings.TrimRight(line[:at], " \t")
		}
	}
	return out
}
