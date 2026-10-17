package ste

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/require"
)

// sentencesByRescan is the splitter as it read before the tally: it reread the
// sentence so far at each terminator. It is the oracle for SentencesOpenedBy.
func sentencesByRescan(text string, opens func([]rune) bool) []string {
	closesMark := func(sentence []rune, mark rune) bool {
		switch mark {
		case ')':
			return strings.Count(string(sentence), "(") > strings.Count(string(sentence), ")")
		case '”':
			return strings.ContainsRune(string(sentence), '“')
		case '’':
			return strings.ContainsRune(string(sentence), '‘')
		case '*', '_', '"', '\'':
			return strings.ContainsRune(string(sentence), mark)
		}
		return false
	}
	endsWithAbbreviation := func(sentence []rune) bool {
		fields := strings.Fields(string(sentence))
		if len(fields) == 0 {
			return false
		}
		last := strings.TrimLeft(fields[len(fields)-1], "([\"'“‘*_")
		return abbreviations.Contains(strings.ToLower(last))
	}
	var out []string
	runes := []rune(text)
	start := 0
	for i := 0; i < len(runes); i++ {
		if !terminator(runes[i]) {
			continue
		}
		end := i
		for end+1 < len(runes) && terminator(runes[end+1]) {
			end++
		}
		stop := end
		for end+1 < len(runes) && closesMark(runes[start:stop], runes[end+1]) {
			end++
		}
		gap := end + 1
		if gap >= len(runes) || !unicode.IsSpace(runes[gap]) {
			i = end
			continue
		}
		next := gap
		for next < len(runes) && unicode.IsSpace(runes[next]) {
			next++
		}
		if next >= len(runes) || !opens(runes[next:]) || endsWithAbbreviation(runes[start:stop+1]) {
			i = end
			continue
		}
		out = append(out, strings.TrimSpace(string(runes[start:end+1])))
		start = next
		i = next - 1
	}
	if rest := strings.TrimSpace(string(runes[start:])); rest != "" {
		out = append(out, rest)
	}
	return out
}

func TestTheSplitterAnswersAsTheRescan(t *testing.T) {
	texts := []string{
		`It ends (here.) Then "a quote." And *bold.* Next 'one.' Then “curly.” Then ‘single.’ Done.`,
		`A stray close.) Next. A glob esnext.* stays. e.g. this stays. (e.g. this too.) Fine.`,
		`Run it.)) Twice. "Open. Still open." Closed. _under._ Over.`,
		"Trailing spaces.   \t Then more.\n\nNew paragraph. i.e. not new.",
	}
	for _, glob := range []string{"../docs/*.md", "../*.md", "../rules/*.xml"} {
		paths, err := filepath.Glob(glob)
		require.NoError(t, err)
		for _, path := range paths {
			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			texts = append(texts, string(raw))
		}
	}
	for _, text := range texts {
		require.Equal(t, sentencesByRescan(text, opensSentence), Sentences(text))
	}
}
