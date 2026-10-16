package counts

import (
	"regexp"
	"strings"

	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// unitAbbreviations spell out the abbreviations of a time unit that follow a number.
var unitAbbreviations = map[string]string{"ns": "nanoseconds", "ms": "milliseconds", "s": "seconds", "min": "minutes"}

// abbreviatedUnit matches the blank and abbreviation right after digits.
var abbreviatedUnit = regexp.MustCompile(`^[ \t]+(ns|ms|s|min)\b`)

// isUnit reports a word that names a unit of measure, spelled out or abbreviated.
func isUnit(word string) bool {
	_, abbreviated := unitAbbreviations[strings.ToLower(word)]
	return abbreviated || cardinal.IsUnit(word)
}

// countedEdit rewords digits that a label heuristic took for an item name but that count
// a compound noun. "report 0 validation errors" reads "report no validation errors",
// where an item edit would write "a later report validation errors".
func countedEdit(content string, from int, line string, at int) (edit.Edit, bool) {
	words := syntax.Parse(line, nil).Words
	i := 0
	for i < len(words) && words[i].Start != at {
		i++
	}
	if i == len(words) {
		return edit.Edit{}, false
	}
	head := cardinal.CountedHead(words, i)
	if head < 0 {
		return edit.Edit{}, false
	}
	return RewordAt(content, from+at, line[at:words[head].End])
}

// measureEdit writes the unit out where digits and an abbreviated unit end a
// phrase: "alive 15 s later" reads "alive several seconds later". Before a
// plural noun the measure goes with its number: "Quiet 30 s runs" reads "Quiet runs".
func measureEdit(content string, start int, line string, at int, digits string) (edit.Edit, bool) {
	loc := abbreviatedUnit.FindStringSubmatchIndex(line[at+len(digits):])
	if loc == nil {
		return edit.Edit{}, false
	}
	end := at + len(digits) + loc[1]
	for _, w := range syntax.Parse(line, nil).Words {
		if w.Start >= end {
			if w.Tag == "NNS" || w.Tag == "NNPS" {
				return RewordAt(content, start, line[at:w.End])
			}
			break
		}
	}
	unit := unitAbbreviations[line[at+len(digits)+loc[2]:at+len(digits)+loc[3]]]
	return edit.Edit{Start: start, End: start + (end - at), Text: vague(value(digits)) + unit, Cut: []string{line[at:end]}}, true
}
