package treecomments

import (
	"strings"
	"unicode"
)

// IsDirective reports a comment line that a tool reads, such as //go:nosplit.
// A space after the marker turns it into prose, so no repair may touch it.
func IsDirective(line string) bool {
	text := strings.TrimSpace(line)
	if strings.HasPrefix(text, "// +build") || tripleSlash(text) {
		return true
	}
	rest, found := strings.CutPrefix(text, "//")
	if !found || rest == "" || rest[0] == ' ' || rest[0] == '\t' {
		return false
	}
	name := rest
	if end := strings.IndexFunc(rest, func(char rune) bool {
		return !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '-' && char != '_'
	}); end >= 0 {
		name = rest[:end]
	}
	if name == "" {
		return false
	}
	after := rest[len(name):]
	if strings.HasPrefix(after, ":") {
		return true
	}
	switch name {
	case "line", "export", "extern", "sys", "sysnb":
		return after == "" || after[0] == ' ' || after[0] == '\t'
	}
	return false
}

// directiveLines answers every directive line in the comments of src, in order.
func directiveLines(filename, src string) []string {
	var out []string
	for _, comment := range Extract(filename, src) {
		for _, line := range strings.Split(comment.Text, "\n") {
			if IsDirective(line) {
				out = append(out, strings.TrimSpace(line))
			}
		}
	}
	return out
}

// tripleSlashTags are the XML tags of a TypeScript triple-slash directive.
var tripleSlashTags = []string{"<reference", "<amd-module", "<amd-dependency"}

// tripleSlash reports a TypeScript triple-slash directive, such as
// `/// <reference types="vite/client" />`. The compiler reads it.
func tripleSlash(text string) bool {
	rest, found := strings.CutPrefix(text, "///")
	if !found {
		return false
	}
	rest = strings.TrimLeft(rest, " \t")
	for _, tag := range tripleSlashTags {
		if after, ok := strings.CutPrefix(rest, tag); ok && after != "" && strings.ContainsRune(" \t/", rune(after[0])) {
			return true
		}
	}
	return false
}
