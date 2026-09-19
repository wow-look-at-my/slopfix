// say.go answers what to write in place of a word, once the grammar has said
// which slot the word fills.
//
// The role is decided by asking the parser a question it can answer exactly:
// does the text still parse with this word kept OUT of that slot. A word whose
// removal from a slot breaks every parse fills that slot. A text that parses
// either way is ambiguous, and a text that parses neither way is prose the
// grammar does not cover. Both are left exactly as they are, because the rule
// still reports the word and a warning costs a reader less than a wrong
// repair.
package table

import "strings"

// A Say is what to write instead of a word standing in a given role.
//
// After names the classes that must sit before the word for this entry to
// apply, skipping any modifiers between. It is what tells "the wrong one",
// whose phrase already carries a determiner, from "without one", whose phrase
// does not.
type Say struct {
	Word   string
	Role   string
	To     string
	After  []string
	Where  string
	Test   string
	Expect string
}

// Roles is every slot a Say entry names, which is what the parser is asked
// about. It is derived rather than declared, so a new role in the XML needs no
// Go.
func Roles(says []Say) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range says {
		if !seen[s.Role] {
			seen[s.Role] = true
			out = append(out, s.Role)
		}
	}
	return out
}

// Apply rewrites each word the entries name, in the role the grammar gives it.
func Apply(g *Grammar, lex *Lexicon, says []Say, prose string) string {
	if g == nil || len(says) == 0 {
		return prose
	}
	spans := wordSpans(prose)
	if len(spans) == 0 {
		return prose
	}
	tokens := make([]string, len(spans))
	for i, s := range spans {
		tokens[i] = strings.ToLower(prose[s.at:s.end])
	}

	var out strings.Builder
	last := 0
	for i, tok := range tokens {
		entry, ok := choose(g, lex, says, tokens, i, tok)
		if !ok {
			continue
		}
		out.WriteString(prose[last:spans[i].at])
		out.WriteString(entry.To)
		last = spans[i].end
	}
	out.WriteString(prose[last:])
	return out.String()
}

// choose finds the entry covering the word at i, if the grammar settles its
// role and the entry's own condition holds.
func choose(g *Grammar, lex *Lexicon, says []Say, tokens []string, i int, tok string) (Say, bool) {
	var named []Say
	for _, s := range says {
		if strings.EqualFold(s.Word, tok) {
			named = append(named, s)
		}
	}
	if len(named) == 0 {
		return Say{}, false
	}
	role, ok := roleOf(g, lex, tokens, i, Roles(named))
	if !ok {
		return Say{}, false
	}
	for _, s := range named {
		if s.Role == role && after(lex, tokens, i, s.After) {
			return s, true
		}
	}
	return Say{}, false
}

// roleOf answers which of the candidate roles the word at i fills. It bans
// each role at that word in turn: the role whose absence breaks every parse is
// the role the word has. Anything less decisive answers false.
func roleOf(g *Grammar, lex *Lexicon, tokens []string, i int, roles []string) (string, bool) {
	var only string
	for _, role := range roles {
		without := g.Parses(lex, tokens, map[int]map[string]bool{i: {role: true}})
		if without {
			continue
		}
		if only != "" {
			// Two roles are each indispensable, so the text does not parse
			// without either. Nothing here can choose between them.
			return "", false
		}
		only = role
	}
	if only == "" {
		return "", false
	}
	// The parse has to exist at all. A text nothing covers fails every ban,
	// which would otherwise read as every role being indispensable at once.
	if !g.Parses(lex, tokens, nil) {
		return "", false
	}
	return only, true
}

// after reports whether a word of one of the named classes sits before i, with
// only modifiers between. An entry naming no class always applies.
func after(lex *Lexicon, tokens []string, i int, classes []string) bool {
	if len(classes) == 0 {
		return true
	}
	for at := i - 1; at >= 0; at-- {
		for _, class := range classes {
			if lex.Is(tokens[at], class) {
				return true
			}
		}
		if !lex.Is(tokens[at], "open") {
			return false
		}
	}
	return false
}

// span is a word's place in the prose.
type span struct{ at, end int }

func wordSpans(prose string) []span {
	var out []span
	for i := 0; i < len(prose); {
		if !isWordByte(prose[i]) {
			i++
			continue
		}
		start := i
		for i < len(prose) && isWordByte(prose[i]) {
			i++
		}
		out = append(out, span{at: start, end: i})
	}
	return out
}
