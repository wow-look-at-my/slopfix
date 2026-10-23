// IsDirective reports a comment line that a tool reads, such as //go:nosplit.

package treecomments

import (
	"regexp"
	"strings"
	"unicode"
)

// IsDirective reports a comment line that a tool reads, such as //go:nosplit.
// A space after the marker turns it into prose, so no repair may touch it.
func IsDirective(line string) bool {
	text := strings.TrimSpace(line)
	if strings.HasPrefix(text, "// +build") || metadataOpen.MatchString(text) || tripleSlash(text) {
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

// metadataOpen and metadataClose delimit a userscript or userstyle metadata
// block, which the browser extension parses line by line.
var (
	metadataOpen  = regexp.MustCompile(`^//\s*==User(?:Script|Style)==\s*$`)
	metadataClose = regexp.MustCompile(`^//\s*==/User(?:Script|Style)==\s*$`)
	metadataKey   = regexp.MustCompile(`^//\s*@[A-Za-z][A-Za-z0-9:_-]*(?:\s|$)`)
)

// DirectiveAt reports whether lines[i] is a directive: a line IsDirective
// reads, or a line inside a userscript metadata block. An "// @name" line
// anywhere else is prose.
func DirectiveAt(lines []string, i int) bool {
	if IsDirective(lines[i]) {
		return true
	}
	text := strings.TrimSpace(lines[i])
	if !metadataKey.MatchString(text) && !metadataClose.MatchString(text) {
		return false
	}
	for j := i - 1; j >= 0; j-- {
		prior := strings.TrimSpace(lines[j])
		if metadataOpen.MatchString(prior) {
			return true
		}
		if metadataClose.MatchString(prior) {
			return false
		}
	}
	return false
}

// DirectiveMask answers DirectiveAt for every line of lines.
func DirectiveMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	for i := range lines {
		mask[i] = DirectiveAt(lines, i)
	}
	return mask
}

// directiveLines answers every directive line in the comments of src, in order.
func directiveLines(filename, src string) []string {
	var out []string
	for _, comment := range Extract(filename, src) {
		lines := strings.Split(comment.Text, "\n")
		for i, line := range lines {
			if DirectiveAt(lines, i) {
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
