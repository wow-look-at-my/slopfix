// english.go loads the prose table the repair applies, and the phrases it
// refuses to rewrite.
//
// The table is XML rather than Go, the same way autoallow carries its rules, so
// adding a word is a single-line edit somebody can make without reading Go.
package commentlength

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

//go:embed english.xml
var englishXML []byte

// xmlEnglish mirrors english.xml.
type xmlEnglish struct {
	Drops    []xmlDrop    `xml:"drop"`
	Rewrites []xmlRewrite `xml:"rewrite"`
	Patterns []xmlPattern `xml:"pattern"`
	Flags    []xmlFlag    `xml:"flag"`
}

// xmlPattern is a rewrite with captures, for a shape a whole-word swap cannot
// express. `to` uses $1, $2 for the groups, as Go's regexp expansion spells it.
type xmlPattern struct {
	Match  string `xml:"match,attr"`
	To     string `xml:"to,attr"`
	Where  string `xml:"where,attr"`
	Test   string `xml:"test,attr"`
	Expect string `xml:"expect,attr"`

	re *regexp.Regexp
}

type xmlDrop struct {
	Word  string `xml:"word,attr"`
	Where string `xml:"where,attr"`
	Test  string `xml:"test,attr"`
}

type xmlRewrite struct {
	From  string `xml:"from,attr"`
	To    string `xml:"to,attr"`
	Where string `xml:"where,attr"`
	Test  string `xml:"test,attr"`
}

// surface is where an entry applies. An empty where= means both, so an entry
// that says nothing about surface applies everywhere.
func appliesTo(where, surface string) bool {
	return where == "" || where == "both" || where == surface
}

type xmlFlag struct {
	Phrase string `xml:"phrase,attr"`
	Say    string `xml:"say,attr"`
	Test   string `xml:"test,attr"`
}

// english is the parsed table. It is parsed at start, and a malformed table
var english = mustLoadEnglish()

func mustLoadEnglish() xmlEnglish {
	var e xmlEnglish
	if err := xml.Unmarshal(englishXML, &e); err != nil {
		panic(fmt.Sprintf("commentlength: english.xml does not parse: %v", err))
	}
	if len(e.Drops) == 0 || len(e.Rewrites) == 0 {
		panic("commentlength: english.xml carries no drops or no rewrites")
	}
	for _, d := range e.Drops {
		if d.Word == "" || d.Test == "" {
			panic("commentlength: a <drop> is missing its word or its test")
		}
	}
	for _, r := range e.Rewrites {
		if r.From == "" || r.To == "" || r.Test == "" {
			panic("commentlength: a <rewrite> is missing from, to or test")
		}
	}
	for _, f := range e.Flags {
		if f.Phrase == "" || f.Say == "" || f.Test == "" {
			panic("commentlength: a <flag> is missing phrase, say or test")
		}
	}
	for i := range e.Patterns {
		p := &e.Patterns[i]
		if p.Match == "" || p.Test == "" || p.Expect == "" {
			panic("commentlength: a <pattern> is missing match, test or expect")
		}
		re, err := regexp.Compile(p.Match)
		if err != nil {
			panic(fmt.Sprintf("commentlength: <pattern match=%q> does not compile: %v", p.Match, err))
		}
		p.re = re
	}
	return e
}

// Suggestion is prose the rule knows is bad and will not rewrite itself.
type Suggestion struct {
	// Phrase is what was found, as the table spells it.
	Phrase string `json:"phrase"`
	// Say names the repair, addressed to whoever reads the finding.
	Say string `json:"say"`
	// Line is where it sits, counting from the top of the file.
	Line int `json:"line"`
}

// Suggest reports the phrases in a file's comments that no rewrite can fix.
//
// It is separate from Check because it is separate advice: Check measures a
// comment against its code, and this reads what the comment says. A file can be
// clean by length and still carry every entry here.
func Suggest(filename, src string) []Suggestion {
	language := languageFor(filename)
	if language == nil {
		return nil
	}
	bs, ok := treeBlocks(language, src)
	if !ok {
		return nil
	}
	var out []Suggestion
	for _, b := range bs {
		for i, line := range b.text {
			lower := strings.ToLower(line)
			for _, f := range english.Flags {
				if !containsWord(lower, strings.ToLower(f.Phrase)) {
					continue
				}
				out = append(out, Suggestion{Phrase: f.Phrase, Say: f.Say, Line: b.start + i + 1})
			}
		}
	}
	return out
}

// containsWord reports a whole-word occurrence, so `hack` never matches
// `hackney` and `for now` never matches `for nowhere`.
func containsWord(s, word string) bool {
	for i := 0; ; {
		j := strings.Index(s[i:], word)
		if j < 0 {
			return false
		}
		at := i + j
		if wordBoundary(s, at, at+len(word)) {
			return true
		}
		i = at + 1
	}
}
