package gitread

import (
	"regexp"
	"strings"
)

// globMatch reports whether a gitignore-style pattern matches a slash-spelled
// path. A pattern with no slash matches the path's base name at any depth.
func globMatch(pattern, path string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	anchored := strings.HasPrefix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")
	if !strings.Contains(pattern, "/") {
		anchored = false
	}
	subject := path
	if !anchored {
		if slash := strings.LastIndexByte(path, '/'); slash >= 0 && !strings.Contains(pattern, "/") {
			subject = path[slash+1:]
		}
	}
	re, err := regexp.Compile(globRegexp(pattern))
	if err != nil {
		return false
	}
	return re.MatchString(subject)
}

// globRegexp turns one gitignore glob into an anchored regular expression.
func globRegexp(pattern string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				// A leading "**/" matches zero or more directories.
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '[':
			end := strings.IndexByte(pattern[i:], ']')
			if end < 0 {
				b.WriteString(regexp.QuoteMeta(string(c)))
				continue
			}
			class := pattern[i : i+end+1]
			if strings.HasPrefix(class, "[!") {
				class = "[^" + class[2:]
			}
			b.WriteString(class)
			i += end
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return b.String()
}

// relTo answers path relative to root in slash form, or false when outside.
func relTo(root, path string) (string, bool) {
	rel, err := relPath(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return rel, true
}
