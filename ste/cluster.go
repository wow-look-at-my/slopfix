package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// markConjunctions is the conjunction each mark between nouns stands for.
var markConjunctions = map[string]string{"+": "and", "&": "and", "/": "or", "|": "or"}

// conjoinMark writes the conjunction a mark inside a noun run stands for. The
// conjunction divides the run into the noun phrases the mark joined. The mark
// is either a word of the run or the text between some of its words.
func conjoinMark(s *syntax.Sentence, source string, start, end int) (string, bool) {
	for k := start; k <= end; k++ {
		w := s.Words[k]
		if conj, ok := markConjunctions[w.Text]; ok {
			return source[:w.Start] + conj + source[w.End:], true
		}
		if k == end {
			break
		}
		next := s.Words[k+1].Start
		if conj, ok := markConjunctions[strings.TrimSpace(source[w.End:next])]; ok {
			return source[:w.End] + " " + conj + " " + source[next:], true
		}
	}
	return "", false
}

// ownerAfter rewrites a possessive inside a noun run with "of": "Acme's llvm
// toolchain build" becomes "llvm toolchain build of Acme". The nouns after the
// possessive name the thing, and the possessive names its owner.
func ownerAfter(s *syntax.Sentence, source string, start, end int) (string, bool) {
	for k := end - 1; k >= start; k-- {
		owner, ok := ownerOf(source[s.Words[start].Start:s.Words[k].End])
		if !ok {
			continue
		}
		thing := source[s.Words[k+1].Start:s.Words[end].End]
		return splice(source, s, start, end, thing+" of "+owner), true
	}
	return "", false
}

// ownerOf answers the owner a possessive phrase names, without its ending.
func ownerOf(phrase string) (string, bool) {
	for _, ending := range []string{"'s", "’s", "s'", "s’"} {
		if !strings.HasSuffix(phrase, ending) {
			continue
		}
		owner := strings.TrimSuffix(phrase, ending)
		if strings.HasPrefix(ending, "s") {
			owner += "s"
		}
		return owner, owner != ""
	}
	return "", false
}
