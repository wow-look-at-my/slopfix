// refs.go finds the references a message hands the reader with nothing to
// click: an issue or PR number, a commit SHA, a branch slug, a bare GitHub URL.
//
// Each match carries the byte range it occupies, because the caller rewrites
// the reference in place rather than reporting it. Text that is already a link
// is blanked beforehand, so a correct reference is never rewritten twice.
package linkrefs

import (
	"regexp"
	"slices"
	"strings"
)

// Ref is an unlinked reference: what it is, and the token itself.
type Ref struct {
	Kind string
	Text string
	// Backticked is true when the token sits inside inline backticks; the rewrite keeps them INSIDE the link text.
	Backticked bool
}

// A markdown link, with an optional title. Both halves go: what survives removal is by definition unlinked.
var mdLinkRe = regexp.MustCompile(`\[[^\]\n]*\]\([^)\s]+(?:[ \t]+"[^"]*")?\)`)

// An angle-bracket autolink, which every markdown renderer makes clickable.
var autoLinkRe = regexp.MustCompile(`<https?://[^>\s]+>`)

var (
	// An issue number, with an optional owner/repo in front, so the boundary check sees the start of the slug.
	numberRe = regexp.MustCompile(`(?:[A-Za-z0-9._-]+/[A-Za-z0-9._-]+)?#[0-9]{1,7}`)
	// A commit SHA, in hex. validSHA checks further: hex alone also describes a decimal number and a word.
	shaRe = regexp.MustCompile(`[0-9a-f]{7,40}`)
	// A branch slug, matched on a conventional prefix and never ending in '.' or '/', which keeps it off file paths.
	branchRe = regexp.MustCompile(`(?i)(?:claude|feature|feat|fix|bugfix|hotfix|release|chore|refactor|wip|renovate|dependabot)/[A-Za-z0-9._/-]*[A-Za-z0-9_-]`)
	// A GitHub URL that survived link removal is a URL the reader must select
	// and paste.
	urlRe = regexp.MustCompile(`https?://(?:www\.)?github\.com/[^\s)\]<>"']+`)
)

// Located is an unlinked reference with the byte range it occupies, so a
// caller can splice a link in over it.
type Located struct {
	Ref
	Start int
	End   int
}

// FindUnlinkedInLine returns every unlinked reference in a line, with offsets into that line.
func FindUnlinkedInLine(line string) []Located {
	return candidates(blankLinks(line))
}

// blankLinks replaces every markdown link, autolink and character reference
// with spaces, preserving the length of the text exactly.
func blankLinks(text string) string {
	b := []byte(text)
	for _, re := range []*regexp.Regexp{mdLinkRe, autoLinkRe, charRefRe} {
		for _, loc := range re.FindAllStringIndex(text, -1) {
			for i := loc[0]; i < loc[1]; i++ {
				b[i] = ' '
			}
		}
	}
	return string(b)
}

// candidates runs every matcher and keeps hits that survive validation. Overlapping hits are dropped.
func candidates(text string) []Located {
	type matcher struct {
		kind  string
		re    *regexp.Regexp
		valid func(string) bool
	}
	// URL leads: it is the longest match, and claiming its span stops the branch matcher firing inside it.
	matchers := []matcher{
		{"a bare GitHub URL", urlRe, nil},
		{"an issue or pull request number", numberRe, nil},
		{"a commit SHA", shaRe, validSHA},
		{"a branch", branchRe, validBranch},
	}
	var out []Located
	for _, m := range matchers {
		for _, loc := range m.re.FindAllStringIndex(text, -1) {
			token := text[loc[0]:loc[1]]
			if !boundedToken(text, loc[0], loc[1]) {
				continue
			}
			if m.valid != nil && !m.valid(token) {
				continue
			}
			if slices.ContainsFunc(out, func(o Located) bool { return loc[0] < o.End && o.Start < loc[1] }) {
				continue
			}
			start, end, backticked := backtickWrapped(text, loc[0], loc[1])
			out = append(out, Located{
				Ref:   Ref{Kind: m.kind, Text: token, Backticked: backticked},
				Start: start,
				End:   end,
			})
		}
	}
	slices.SortStableFunc(out, func(a, b Located) int { return a.Start - b.Start })
	return out
}

// boundedToken reports whether a match stands on its own rather than sitting inside a longer word or path.
func boundedToken(text string, start, end int) bool {
	if start > 0 && continuesToken(text, start-1, -1) {
		return false
	}
	if end < len(text) && continuesToken(text, end, +1) {
		return false
	}
	return true
}

// continuesToken reports whether the byte at i extends the match into a longer word, reading toward dir.
// A '.' counts only when an alphanumeric follows: `6884dd2.` is a commit, `6884dd2abc.log` is a filename.
func continuesToken(text string, i, dir int) bool {
	b := text[i]
	if b == '.' {
		next := i + dir
		return next >= 0 && next < len(text) && isAlnum(text[next])
	}
	return isAlnum(b) || b == '_' || b == '/' || b == '-'
}

// backtickWrapped reports whether the match at [start,end) sits inside a pair of inline backticks, and returns
// the widened range that swallows them. Only a LONE backtick on each side counts, since ``x`` is an escape.
func backtickWrapped(text string, start, end int) (int, int, bool) {
	if start == 0 || end >= len(text) || text[start-1] != '`' || text[end] != '`' {
		return start, end, false
	}
	if start >= 2 && text[start-2] == '`' {
		return start, end, false
	}
	if end+1 < len(text) && text[end+1] == '`' {
		return start, end, false
	}
	return start - 1, end + 1, true
}

func isAlnum(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// validSHA requires both a digit and a hex letter, which is what separates a
// commit from a decimal number (a timestamp, a byte count) and from an
// ordinary word spelled in a-f ("defaced", "cabbage").
func validSHA(token string) bool {
	hasDigit, hasLetter := false, false
	for _, r := range token {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r >= 'a' && r <= 'f':
			hasLetter = true
		}
	}
	return hasDigit && hasLetter
}

// validBranch requires a name after the prefix, so the bare word "fix/" is not
// a branch.
func validBranch(token string) bool {
	i := strings.Index(token, "/")
	return i > 0 && i < len(token)-1
}

// issueURLRe matches a GitHub pull request or issue URL, with a trailing path or anchor allowed and ignored.
var issueURLRe = regexp.MustCompile(`^https?://(?:www\.)?github\.com/([A-Za-z0-9._-]+)/([A-Za-z0-9._-]+)/(?:pull|issues)/([0-9]{1,7})(?:[/#?].*)?$`)

// IssueRef reads the repository and number out of a GitHub pull request or issue
// URL, and reports false for every other URL.
func IssueRef(url string) (Repo, string, bool) {
	m := issueURLRe.FindStringSubmatch(strings.TrimRight(url, "."))
	if m == nil {
		return Repo{}, "", false
	}
	return Repo{Owner: m[1], Name: m[2]}, m[3], true
}

// charRefRe is a `&#N;` character reference: XML rather than an issue number, so it is blanked alongside links.
var charRefRe = regexp.MustCompile(`&#x?[0-9a-fA-F]{1,7};`)

// fenceMarker returns "`" or "~" when a line opens or closes a code fence.
func fenceMarker(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	for _, m := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, m) {
			return m[:1]
		}
	}
	return ""
}
