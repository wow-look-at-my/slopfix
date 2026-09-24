// guard.go refuses a rewrite that leaves a sentence ungrammatical.
//
// A table entry is a string swap, and a swap reads none of the words around
// it. So each step of the repair is checked against the prose it started from,
// and a step that adds a defect is thrown away. The caller reports the entry by
// name, so the author fixes the entry and not the file.
package commentfix

import (
	"fmt"
	"regexp"
	"strings"
)

// A Rejection is a table entry whose rewrite the guard threw away.
type Rejection struct {
	// Path and Line say where the comment sits. Line counts from the top.
	Path string
	Line int
	// Rule is the table entry's id.
	Rule string
	// Defect says what the rewrite got wrong.
	Defect string
	// Wrote is the prose the entry would have written.
	Wrote string
}

var (
	wordRun = regexp.MustCompile(`[A-Za-z]+`)
	// aThenWord catches the article "a" and the word after it.
	aThenWord = regexp.MustCompile(`(?i)\ba\s+([a-z]+)`)
	// singularThenWord catches a determiner that wants a singular noun.
	singularThenWord = regexp.MustCompile(`(?i)\b(?:each|every|a single)\s+([a-z]+)`)
)

// defectOf answers what after gets wrong that before did not, or "".
func defectOf(before, after string) string {
	switch {
	case count(doubledWords(after)) > count(doubledWords(before)):
		return "the same word twice in a row"
	case count(aBeforeVowel(after)) > count(aBeforeVowel(before)):
		return `"a" before a vowel sound`
	case count(singularBeforePlural(after)) > count(singularBeforePlural(before)):
		return `"each", "every" or "a single" before a plural noun`
	case opensUpper(before) && !opensUpper(after) && after != "":
		return "a lowercase start where the original was capitalized"
	}
	return ""
}

func count(s []string) int { return len(s) }

// String is the line a caller prints for the rejection.
func (r Rejection) String() string {
	return fmt.Sprintf("%s:%d: [%s/%s] discarded a rewrite with %s: %q", r.Path, r.Line, ID, r.Rule, r.Defect, r.Wrote)
}

// doubledWords answers each word that directly repeats the word before it.
func doubledWords(s string) []string {
	var out []string
	words := wordRun.FindAllString(s, -1)
	for i := 1; i < len(words); i++ {
		if strings.EqualFold(words[i], words[i-1]) {
			out = append(out, words[i])
		}
	}
	return out
}

// aBeforeVowel answers each word that follows "a" and opens on a vowel sound.
func aBeforeVowel(s string) []string {
	var out []string
	for _, m := range aThenWord.FindAllStringSubmatch(s, -1) {
		if vowelSound(strings.ToLower(m[1])) {
			out = append(out, m[1])
		}
	}
	return out
}

// consonantU are the "u" openings that sound like "you".
var consonantU = []string{"uni", "use", "usu", "uti", "ura", "uri", "ubi", "euro"}

// silentH are the "h" openings that sound like a vowel.
var silentH = []string{"hour", "honest", "honor", "honour", "heir"}

// vowelSound reports whether a lowercase word opens on a vowel sound.
func vowelSound(w string) bool {
	for _, p := range silentH {
		if strings.HasPrefix(w, p) {
			return true
		}
	}
	if w == "" || !strings.ContainsRune("aeiou", rune(w[0])) {
		return false
	}
	for _, p := range consonantU {
		if strings.HasPrefix(w, p) {
			return false
		}
	}
	return !strings.HasPrefix(w, "one")
}

// singularBeforePlural answers each plural noun after a singular determiner.
func singularBeforePlural(s string) []string {
	var out []string
	for _, m := range singularThenWord.FindAllStringSubmatch(s, -1) {
		if looksPlural(strings.ToLower(m[1])) {
			out = append(out, m[1])
		}
	}
	return out
}

// notPlural end in s and name a single thing, or carry a verb.
var notPlural = []string{"ss", "us", "is", "ous", "ics"}

// looksPlural reports whether a word reads as a plural noun.
func looksPlural(w string) bool {
	if len(w) < 4 || !strings.HasSuffix(w, "s") {
		return false
	}
	for _, end := range notPlural {
		if strings.HasSuffix(w, end) {
			return false
		}
	}
	return true
}

// opensUpper reports whether s opens on an uppercase letter.
func opensUpper(s string) bool { return s != "" && s[0] >= 'A' && s[0] <= 'Z' }
