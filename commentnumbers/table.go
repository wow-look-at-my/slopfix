// table.go loads the English the number repair applies, and applies it to a
// line of comment prose.
//
// The table is XML rather than Go, the same way commentlength carries its own,
// so adding a phrase is an edit somebody makes without reading Go.
package commentnumbers

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

//go:embed numbers.xml
var numbersXML []byte

// xmlTable mirrors numbers.xml.
type xmlTable struct {
	Rewrites []xmlRewrite `xml:"rewrite"`
	Patterns []xmlPattern `xml:"pattern"`
}

// xmlRewrite swaps a whole phrase for another.
type xmlRewrite struct {
	From   string `xml:"from,attr"`
	To     string `xml:"to,attr"`
	Test   string `xml:"test,attr"`
	Expect string `xml:"expect,attr"`
}

// xmlPattern carries a shape a phrase swap cannot express. `to` uses $1, $2 for
// the groups, as Go's regexp expansion spells it.
type xmlPattern struct {
	Match  string `xml:"match,attr"`
	To     string `xml:"to,attr"`
	Test   string `xml:"test,attr"`
	Expect string `xml:"expect,attr"`

	re *regexp.Regexp
}

// table is read at startup, so a malformed entry fails the binary rather than a
// file.
var table = load()

// load parses the embedded table and compiles every pattern.
func load() xmlTable {
	var parsed xmlTable
	if err := xml.Unmarshal(numbersXML, &parsed); err != nil {
		panic(fmt.Sprintf("commentnumbers: numbers.xml does not parse: %v", err))
	}
	for i := range parsed.Patterns {
		re, err := regexp.Compile(parsed.Patterns[i].Match)
		if err != nil {
			panic(fmt.Sprintf("commentnumbers: pattern %q does not compile: %v", parsed.Patterns[i].Match, err))
		}
		parsed.Patterns[i].re = re
	}
	return parsed
}

// Say rewrites a line of comment prose, applying every table entry. It says
// nothing about what is left: the caller checks that.
//
// The rewrites go before the patterns, so a shape reads the text a phrase swap
// has already settled.
func Say(prose string) string {
	original := prose
	for _, r := range table.Rewrites {
		prose = replaceWord(prose, r.From, r.To)
	}
	for _, p := range table.Patterns {
		prose = p.re.ReplaceAllString(prose, p.To)
	}
	prose = strings.Join(strings.Fields(prose), " ")
	prose = strings.ReplaceAll(prose, " ,", ",")
	prose = strings.ReplaceAll(prose, " .", ".")
	return capitalise(original, prose)
}

// replaceWord swaps a whole word or phrase, case-insensitively, leaving a
// longer word that merely contains it alone.
func replaceWord(s, word, with string) string {
	lower := strings.ToLower(s)
	target := strings.ToLower(word)
	var b strings.Builder
	for i := 0; i < len(s); {
		j := strings.Index(lower[i:], target)
		if j < 0 {
			b.WriteString(s[i:])
			break
		}
		at := i + j
		end := at + len(target)
		if !wordBoundary(s, at, end) {
			b.WriteString(s[i : at+1])
			i = at + 1
			continue
		}
		b.WriteString(s[i:at])
		b.WriteString(with)
		i = end
	}
	return b.String()
}

// wordBoundary reports whether s[at:end] stands as its own word.
func wordBoundary(s string, at, end int) bool {
	if at > 0 && isWordByte(s[at-1]) {
		return false
	}
	return end >= len(s) || !isWordByte(s[end])
}

func isWordByte(b byte) bool {
	return b == '_' || b == '-' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// capitalise restores the opening capital a leading rewrite can remove.
func capitalise(original, s string) string {
	if s == "" || sameFirstWord(original, s) {
		return s
	}
	if c := s[0]; c >= 'a' && c <= 'z' {
		return string(c-32) + s[1:]
	}
	return s
}

// sameFirstWord reports whether the repair left the opening word in place.
func sameFirstWord(original, s string) bool {
	before, after := strings.Fields(original), strings.Fields(s)
	if len(before) == 0 || len(after) == 0 {
		return false
	}
	return before[0] == after[0]
}
