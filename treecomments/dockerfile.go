package treecomments

import (
	"path/filepath"
	"regexp"
	"strings"
)

// parserDirective is a line BuildKit reads as a parser directive when it heads a Dockerfile.
var parserDirective = regexp.MustCompile(`(?i)^#\s*(syntax|escape|check)\s*=\s*\S`)

// dropParserDirectives removes the parser directives a Dockerfile opens on.
func dropParserDirectives(filename string, comments []Comment) []Comment {
	if !isDockerfile(filename) {
		return comments
	}
	next := 1
	for i, c := range comments {
		if c.Line != next || c.Col != 0 || c.Lines != 1 || !parserDirective.MatchString(c.Text) {
			return comments[i:]
		}
		next++
	}
	return nil
}

// isDockerfile reports whether filename names a Dockerfile, in the spellings
// docker build and podman build look for.
func isDockerfile(filename string) bool {
	base := strings.ToLower(filepath.Base(filename))
	for _, name := range []string{"dockerfile", "containerfile"} {
		if base == name || strings.HasPrefix(base, name+".") || strings.HasSuffix(base, "."+name) {
			return true
		}
	}
	return false
}
