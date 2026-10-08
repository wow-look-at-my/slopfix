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
	if strings.HasPrefix(text, "// +build") || isMetadataLine(text) {
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

// metadataLine is a line of a userscript metadata block, which the browser extension parses.
var metadataLine = regexp.MustCompile(`^//\s*(?:==/?User(?:Script|Style)==\s*$|@[A-Za-z][A-Za-z0-9:_-]*(?:\s|$))`)

// isMetadataLine reports a trimmed line of a userscript metadata block.
func isMetadataLine(text string) bool {
	return metadataLine.MatchString(text)
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
