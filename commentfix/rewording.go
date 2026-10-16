package commentfix

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/counts"
	"github.com/wow-look-at-my/slopfix/ste"
)

// cutWhatIsLeft rewords each number the table did not rewrite, as the document
// count rule does. It removes the sentence around a number that is still left,
// and reports what it cut.
func cutWhatIsLeft(prose string) (string, []string) {
	prose = rewordCounts(prose)
	if len(cardinal.Find(prose, cardinal.Comment)) == 0 {
		return prose, nil
	}
	var kept, cut []string
	for _, sentence := range sentences(prose) {
		if len(cardinal.Find(sentence, cardinal.Comment)) > 0 {
			cut = append(cut, strings.TrimSpace(sentence))
			continue
		}
		kept = append(kept, strings.TrimSpace(sentence))
	}
	return strings.TrimSpace(strings.Join(kept, " ")), cut
}

// governed is a number and the word after it, which is the noun it counts.
var governed = regexp.MustCompile(`^\S+\s+[A-Za-z][A-Za-z-]*`)

// rewordCounts takes each number out of the prose with counts.RewordAt, from
// the back, so an earlier offset stays valid.
func rewordCounts(prose string) string {
	tokens := cardinal.Find(prose, cardinal.Comment)
	for i := len(tokens) - 1; i >= 0; i-- {
		t := tokens[i]
		phrase := governed.FindString(prose[t.Offset:])
		if phrase == "" {
			continue
		}
		at := cardinal.Match{At: t.Offset, Text: phrase}
		zeroSubject := strings.HasPrefix(phrase, "0 ") && cardinal.ValueSubject(prose, at)
		if !zeroSubject && cardinal.NotAPluralNoun(prose, at) {
			continue
		}
		if e, ok := counts.RewordAt(prose, t.Offset, phrase); ok {
			prose = prose[:e.Start] + e.Text + prose[e.End:]
		}
	}
	return prose
}

// sentences splits comment prose on its sentence ends. A comment sentence
// often opens on a lower-case identifier, such as a parameter name.
func sentences(prose string) []string {
	return ste.SentencesOpenedBy(prose, func(rest []rune) bool {
		return unicode.IsLower(rest[0]) || ste.OpensSentence(rest)
	})
}
