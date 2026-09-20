// match.go is the matcher language the prose rules are written in. A term
// names a word CLASS, which is the thing a regexp has no way to say:
//
// A term is {class}, or {class:name} to capture it, or {class*:name} for any
// number including none. A bare word matches itself. The replacement is plain
// text with {name} where a capture goes, so a rule can swap a word, delete it,
// or rewrite the phrase around it.
package table

import "strings"

// A Term is a single element of a match.
type Term struct {
	// Classes are the word classes the term matches, and any of them fits. A literal leaves it empty.
	Classes []string
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
	return Term{Classes: strings.Split(class, "|"), Name: name, Many: many}, nil
}

// Classes answers every class a match names, which is what checks each is
// declared.
func (m Match) Classes() []string {
	var out []string
	for _, t := range m.Terms {
		out = append(out, t.Classes...)
	}
	return out
}

// find answers the words a match covers from i, with each capture, or reports
// that it does not fit here.
func (m Match) find(lex *Lexicon, tokens []string, i int) (end int, caught map[string]string, ok bool) {
	caught = map[string]string{}
	at := i
	for _, term := range m.Terms {
		if term.Many {
			start := at
			for at < len(tokens) && term.fits(lex, tokens[at]) {
				at++
			}
			if term.Name != "" {
				caught[term.Name] = strings.Join(tokens[start:at], " ")
			}
			continue
		}
		if at >= len(tokens) || !term.fits(lex, tokens[at]) {
			return 0, nil, false
		}
		if term.Name != "" {
			caught[term.Name] = tokens[at]
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
