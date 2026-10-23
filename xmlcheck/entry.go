// Compile reads a pattern from a table.

package xmlcheck

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Compile reads a pattern from a table. It matches without regard to case
// unless the entry is sensitive, so the XML needs no (?i) of its own.
func Compile(match string, sensitive bool) (*regexp.Regexp, error) {
	return regexp.Compile(Pattern(match, sensitive))
}

// HasPhrase reports whether text holds phrase, and ignores case unless the
// entry is sensitive.
func HasPhrase(text, phrase string, sensitive bool) bool {
	if !sensitive {
		text, phrase = strings.ToLower(text), strings.ToLower(phrase)
	}
	return strings.Contains(text, phrase)
}

// expansionRef finds a reference in a replacement: $$, ${name} and $name.
var expansionRef = regexp.MustCompile(`\$(?:\$|\{([A-Za-z0-9_]+)\}|([A-Za-z0-9_]+))`)

// PatternProblems answers what makes a pattern entry unusable: a match that
// does not compile, and a replacement that names a group the match lacks.
func PatternProblems(match, template string, sensitive bool) []string {
	re, err := Compile(match, sensitive)
	if err != nil {
		return []string{fmt.Sprintf("the match does not compile: %v", err)}
	}
	var out []string
	for _, ref := range expansionRef.FindAllStringSubmatch(template, -1) {
		name := ref[1]
		if name == "" {
			name = ref[2]
		}
		if ref[0] == "$$" || hasGroup(re, name) {
			continue
		}
		out = append(out, fmt.Sprintf("the replacement names the group %q, which the match does not have", name))
	}
	return out
}

func hasGroup(re *regexp.Regexp, name string) bool {
	if n, err := strconv.Atoi(name); err == nil {
		return n >= 0 && n <= re.NumSubexp()
	}
	return slices.Contains(re.SubexpNames(), name)
}

// RewriteProblems answers what makes a phrase swap pointless: it writes the
// phrase it found, or it writes a text that still holds it.
func RewriteProblems(from, to string, sensitive bool) []string {
	same := strings.EqualFold(from, to)
	if sensitive {
		same = from == to
	}
	if same {
		return []string{fmt.Sprintf("the phrase %q is rewritten to itself", from)}
	}
	if !sensitive {
		from, to = strings.ToLower(from), strings.ToLower(to)
	}
	if containsPhrase(to, from) {
		return []string{fmt.Sprintf("the replacement %q holds the phrase %q it replaces, so a second pass rewrites it again", to, from)}
	}
	return nil
}
