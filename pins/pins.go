// Package pins finds a download URL that names an exact release.
//
// dl.pazer.build serves the newest published build on a project's default
// branch when the URL names no version.
package pins

import (
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/ste"
)

// ID names the rule.
const ID = "pins/download-version"

// AllIDs names every rule in the category.
var AllIDs = set.Of(ID)

// downloadURL finds a dl.pazer.build URL in text. Braces stay in, for a template such as `${OS}`.
var downloadURL = regexp.MustCompile("dl\\.pazer\\.build/[^\\s\"'`<>()\\[\\]\\\\]*")

// found is a dl.pazer.build URL in a text.
type found struct {
	raw        string
	line       int
	start, end int
	parsed     *url.URL
}

// scan answers every dl.pazer.build URL in text that net/url parses, with byte offsets into text.
func scan(text string) []found {
	var out []found
	lineStart := 0
	for index, line := range strings.Split(text, "\n") {
		for _, at := range downloadURL.FindAllStringIndex(line, -1) {
			raw := line[at[0]:at[1]]
			parsed, err := url.Parse("https://" + html.UnescapeString(raw))
			if err != nil {
				continue
			}
			out = append(out, found{raw: raw, line: index + 1, start: lineStart + at[0], end: lineStart + at[1], parsed: parsed})
		}
		lineStart += len(line) + 1
	}
	return out
}

// pinned reports whether the URL's query names the parameter v.
func (f found) pinned() bool { return f.parsed.Query().Has("v") }

// unpinned answers the URL with v deleted, or false when re-encoding its query
// would change it past the deletion. Encode escapes a template such as `${OS}`,
// and writes `&` where the text held `&amp;`.
func (f found) unpinned() (string, bool) {
	if strings.ContainsAny(f.parsed.RawQuery, "${}") || strings.Contains(f.raw, "&amp;") {
		return "", false
	}
	query := f.parsed.Query()
	query.Del("v")
	u := *f.parsed
	u.RawQuery = query.Encode()
	return strings.TrimPrefix(u.String(), "https://"), true
}

// Check reports each dl.pazer.build URL in text that carries a `v` parameter.
func Check(text string) []ste.Finding {
	var findings []ste.Finding
	for _, f := range scan(text) {
		if !f.pinned() {
			continue
		}
		findings = append(findings, ste.Finding{
			Line:   f.line,
			ID:     ID,
			Rule:   "a dl.pazer.build URL pins a release with v=",
			Detail: f.raw,
			Fix:    "Drop the v parameter. The URL then serves the newest build on the default branch, and branch= names another branch",
		})
	}
	return findings
}

// Edits answers the edit that rewrites each pinned URL without its v parameter.
func Edits(text string) []edit.Edit {
	var out []edit.Edit
	for _, f := range scan(text) {
		if !f.pinned() {
			continue
		}
		unpinned, ok := f.unpinned()
		if !ok {
			continue
		}
		out = append(out, edit.Edit{Start: f.start, End: f.end, Text: unpinned, Cut: []string{f.raw}})
	}
	return out
}

// Gate writes an edit only when it is one Edits derives from src, and holds only
// when every URL survives it.
func Gate(src string, edits []edit.Edit, scope edit.Scope) edit.Result {
	own := Edits(src)
	urls := len(scan(src))
	reach := func(e edit.Edit) string {
		for _, o := range own {
			if o.Start == e.Start && o.End == e.End && o.Text == e.Text {
				return ""
			}
		}
		return "it is not the rewrite of a dl.pazer.build URL without its v parameter"
	}
	holds := func(text string) bool { return len(scan(text)) == urls }
	return edit.Gate(src, edits, scope, reach, holds)
}

func init() {
	fixer.Register(fixer.Spec{
		Label:    ID,
		Families: []string{"pins"},
		Rules:    []string{ID},
		Files:    []fixer.Kind{fixer.Source, fixer.Document, fixer.Workflow},
		Place:    1000,
		Repair: func(f *fixer.File) {
			f.ApplyThrough(Gate, Edits(f.Text()))
		},
	})
}
