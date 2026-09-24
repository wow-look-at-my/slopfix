// match.go is the matcher language the prose rules are written in. A term
// names a word CLASS, which is the thing a regexp has no way to say.
package table

import "strings"

// A Term is a single element of a match.
type Term struct {
	// Classes are the word classes the term matches, and any of them fits. A literal leaves it empty.
	Classes []string
	// Also are classes the word must belong to as well, whichever of Classes fits.
	Also []string
	// Word is the literal a term matches, lowercased.
	Word string
	// Name is what the replacement calls this term's text.
	Name string
	// Many lets the term match any number of words, including none.
	Many bool
}

// A Match is a parsed match string.
type Match struct {
	Terms []Term
}

// ParseMatch reads a match string. Its error reaches the generate step, so a
// malformed rule never lands in the binary.
func ParseMatch(s string) (Match, error) {
	var m Match
	for _, field := range strings.Fields(s) {
		term, err := parseTerm(field)
		if err != nil {
			return Match{}, err
		}
		m.Terms = append(m.Terms, term)
	}
	if len(m.Terms) == 0 {
		return Match{}, matchError("a match names no term")
	}
	return m, nil
}

// parseTerm reads {class}, {class:name} or {class*:name}. {a|b} fits either
// class and {a+b} fits both. A bare word matches itself.
func parseTerm(field string) (Term, error) {
	body, braced := strings.CutPrefix(field, "{")
	if !braced {
		return Term{Word: strings.ToLower(field)}, nil
	}
	body, closed := strings.CutSuffix(body, "}")
	if !closed || body == "" {
		return Term{}, errBadTerm(field)
	}
	class, name, _ := strings.Cut(body, ":")
	class, many := strings.CutSuffix(class, "*")
	if class == "" {
		return Term{}, errBadTerm(field)
	}
	anyOf, also, _ := strings.Cut(class, "+")
	term := Term{Classes: strings.Split(anyOf, "|"), Name: name, Many: many}
	if also != "" {
		term.Also = strings.Split(also, "+")
	}
	return term, nil
}

// Classes answers every class a match names, which is what checks each is
// declared.
func (m Match) Classes() []string {
	var out []string
	for _, t := range m.Terms {
		out = append(out, t.Classes...)
		out = append(out, t.Also...)
	}
	return out
}

// find answers the words a match covers from i, with each capture, or reports
// that it does not fit here.
//
// A term matches the lowercased tokens and captures from written, which holds
// the words as their author spelled them. A capture writes a word back, and a
// name it lowercases is a name that does not exist.
func (m Match) find(lex *Lexicon, tokens, written []string, i int) (end int, caught map[string]string, ok bool) {
	caught = map[string]string{}
	at := i
	for _, term := range m.Terms {
		if term.Many {
			start := at
			for at < len(tokens) && term.fits(lex, tokens[at]) {
				at++
			}
			if term.Name != "" {
				caught[term.Name] = strings.Join(written[start:at], " ")
			}
			continue
		}
		if at >= len(tokens) || !term.fits(lex, tokens[at]) {
			return 0, nil, false
		}
		if term.Name != "" {
			caught[term.Name] = written[at]
		}
		at++
	}
	return at, caught, true
}

// fits reports whether a word satisfies a term.
func (t Term) fits(lex *Lexicon, word string) bool {
	if len(t.Classes) == 0 {
		return strings.EqualFold(t.Word, word)
	}
	for _, class := range t.Also {
		if !lex.Is(word, class) {
			return false
		}
	}
	for _, class := range t.Classes {
		if lex.Is(word, class) {
			return true
		}
	}
	return false
}

// Expand writes a replacement, putting each capture where its name stands. A
// name nothing captured writes nothing, and the spacing closes around it.
func Expand(to string, caught map[string]string) string {
	var out strings.Builder
	for i := 0; i < len(to); i++ {
		if to[i] != '{' {
			out.WriteByte(to[i])
			continue
		}
		end := strings.IndexByte(to[i:], '}')
		if end < 0 {
			out.WriteByte(to[i])
			continue
		}
		out.WriteString(caught[to[i+1:i+end]])
		i += end
	}
	return strings.Join(strings.Fields(out.String()), " ")
}

type matchError string

func (e matchError) Error() string { return string(e) }

func errBadTerm(field string) error {
	return matchError("a term is not {class}, {class:name} or {class*:name}: " + field)
}
