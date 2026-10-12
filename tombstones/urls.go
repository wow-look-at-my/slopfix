package tombstones

import (
	"regexp"
	"strings"
)

// urlPattern matches a URL: a scheme, "://" and the run of bytes up to a blank or a closing bracket or quote.
var urlPattern = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^\s<>()\[\]"'` + "`" + `]+`)

// withoutURLs writes blanks over each URL of text. A token inside a URL, such
// as the id in "https://host/session_01Abc", names a resource on that host.
func withoutURLs(text string) string {
	return urlPattern.ReplaceAllStringFunc(text, func(url string) string {
		return strings.Repeat(" ", len(url))
	})
}
