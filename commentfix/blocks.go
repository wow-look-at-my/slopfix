// blocks.go decides which files this rule reads, and which it skips.
//
// The measuring is in treeblocks.go, off a real syntax tree. What is left here
// is what comes before a parse: does a grammar cover this file, and did a
// generator write it.
package commentfix

import (
	"regexp"
	"strings"
)

// generatedLine is the canonical generated-file header. commentspan skips a
var generatedLine = regexp.MustCompile(`^\s*(?://+|#+)\s*Code generated .* DO NOT EDIT\.$`)

// isGeneratedLines reports the marker in the file's header, above any code.
// commentspan looks for it above the package clause, which is that region for a
// Go file.
func isGeneratedLines(lines []string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !startsComment(trimmed) {
			return false
		}
		if generatedLine.MatchString(line) {
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
	if isGeneratedLines(splitLines(src)) {
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
