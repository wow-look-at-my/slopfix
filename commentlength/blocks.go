// blocks.go decides which files this rule reads, and which it skips.
//
// The measuring is in treeblocks.go, off a real syntax tree. What is left here
// is what comes before a parse: does a grammar cover this file, and did a
// generator write it.
package commentlength

import (
	"regexp"
	"strings"
)

// generatedMarker is the canonical generated-file header. commentspan skips a
// file carrying it, because nobody can act on a finding in generated code.
var generatedMarker = regexp.MustCompile(`^\s*(?://+|#+)\s*Code generated .* DO NOT EDIT\.$`)

// isGenerated reports the marker in the file's header, above any code.
// commentspan looks for it above the package clause, which is that region for a
// Go file.
func isGenerated(lines []string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !startsComment(trimmed) {
			return false
		}
		if generatedMarker.MatchString(line) {
			return true
		}
	}
	return false
}

// blocks returns every comment block in the file, each with the code it
// documents measured beside it.
func blocks(filename, src string) []block {
	language := languageFor(filename)
	if language == nil {
		return nil
	}
	if isGenerated(splitLines(src)) {
		return nil
	}
	parsed, ok := treeBlocks(language, src)
	if !ok {
		return nil
	}
	return parsed
}

// startsComment reports a line whose leading token opens a comment.
func startsComment(trimmed string) bool {
	for _, marker := range []string{"//", "/*", "#", "*/", "*"} {
		if strings.HasPrefix(trimmed, marker) {
			return true
		}
	}
	return false
}
