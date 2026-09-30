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
	if strings.HasPrefix(text, "// +build") {
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

// licenseMark is the notice that makes a comment run a license header.
var licenseMark = regexp.MustCompile(`(?i)\bcopyright\b|SPDX-License-Identifier:`)

// LicenseHeader answers where the comment run that opens the file sits, when
// that run carries a license notice. A legal notice stays byte for byte.
func LicenseHeader(filename, src string) (start, end int, ok bool) {
	runs := Runs(filename, src)
	if len(runs) == 0 {
		return 0, 0, false
	}
	head := runs[0]
	start = head[0].Offset
	before := strings.TrimSpace(src[:start])
	if before != "" && (!strings.HasPrefix(before, "#!") || strings.Contains(before, "\n")) {
		return 0, 0, false
	}
	last := head[len(head)-1]
	end = last.Offset + len(last.Text)
	if !licenseMark.MatchString(src[start:end]) {
		return 0, 0, false
	}
	return start, end, true
}

// licenseText answers the license header of src, or "" when it has none.
func licenseText(filename, src string) string {
	start, end, ok := LicenseHeader(filename, src)
	if !ok {
		return ""
	}
	return src[start:end]
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
