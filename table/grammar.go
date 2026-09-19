// grammar.go parses prose against the phrase structure rules/ declares.
//
// The parser is an Earley recognizer. It takes ANY context-free grammar,
// including a left-recursive or an ambiguous one, which ordinary English needs
// and a run of slots cannot express. A noun phrase holding a prepositional
// phrase holding a noun phrase is a rule naming itself, and that is what makes
// the grammar describe a sentence of any depth rather than a fixed window.
//
// What the repair asks of it is never "what does this sentence mean". It is
// narrower and decidable: can this text parse at all with the word in that
// slot. Asking it once per candidate slot settles the role without building a
// parse forest, and a text that parses either way is left alone.
package table

import "strings"

// A Sym is what a production names: another rule, a word class, or any word no
// class with a word list claims.
type Sym struct {
	Rule  string
	Class string
	Open  bool
}

// A Prod is a run of symbols. An empty production matches nothing at all,
// which is how a rule spells "none of these".
type Prod []Sym

// A Rule is a phrase and the ways it can be built.
type Rule struct {
	Name  string
	Prods []Prod
}

// A Grammar is the phrase structure. Start is the rule a whole text has to
// reduce to.
type Grammar struct {
	Start string
	Rules []Rule
}

// prods answers a rule's productions.
func (g *Grammar) prods(name string) []Prod {
	for _, r := range g.Rules {
		if r.Name == name {
			return r.Prods
		}
	}
	return nil
}

// item is a production with a position in it, and the token it began at.
type item struct {
	rule   string
	prod   int
	dot    int
	origin int
}

// Parses reports whether the words reduce to the grammar's start rule.
//
// banned forbids a rule from covering the single word at an index. That is how
// a caller asks whether the text still parses with a word kept OUT of a slot,
// which is the question that settles the word's role.
func (g *Grammar) Parses(lex *Lexicon, tokens []string, banned map[int]map[string]bool) bool {
	if len(tokens) == 0 {
		return false
	}
	chart := make([]*itemSet, len(tokens)+1)
	for i := range chart {
		chart[i] = newItemSet()
	}
	for p := range g.prods(g.Start) {
		chart[0].add(item{rule: g.Start, prod: p, dot: 0, origin: 0})
	}

	for i := 0; i <= len(tokens); i++ {
		for at := 0; at < len(chart[i].order); at++ {
			it := chart[i].order[at]
			prod := g.prods(it.rule)[it.prod]
			if it.dot >= len(prod) {
				g.complete(chart, i, it, banned)
				continue
			}
			switch sym := prod[it.dot]; {
			case sym.Rule != "":
				for p := range g.prods(sym.Rule) {
					chart[i].add(item{rule: sym.Rule, prod: p, dot: 0, origin: i})
				}
				// A rule that can vanish has to let the item past it, or an
				// empty production never advances anything.
				if g.nullable(sym.Rule, map[string]bool{}) {
					chart[i].add(item{rule: it.rule, prod: it.prod, dot: it.dot + 1, origin: it.origin})
				}
			case i < len(tokens) && matches(lex, sym, tokens[i]):
				chart[i+1].add(item{rule: it.rule, prod: it.prod, dot: it.dot + 1, origin: it.origin})
			}
		}
	}

	for _, it := range chart[len(tokens)].order {
		if it.rule == g.Start && it.origin == 0 && it.dot >= len(g.prods(it.rule)[it.prod]) {
			return true
		}
	}
	return false
}

// complete advances every item waiting on the rule this one just finished.
func (g *Grammar) complete(chart []*itemSet, i int, done item, banned map[int]map[string]bool) {
	if i == done.origin+1 && banned[done.origin][done.rule] {
		return
	}
	for _, it := range chart[done.origin].order {
		prod := g.prods(it.rule)[it.prod]
		if it.dot < len(prod) && prod[it.dot].Rule == done.rule {
			chart[i].add(item{rule: it.rule, prod: it.prod, dot: it.dot + 1, origin: it.origin})
		}
	}
}

// nullable reports whether a rule can cover no words at all.
func (g *Grammar) nullable(name string, seen map[string]bool) bool {
	if seen[name] {
		return false
	}
	seen[name] = true
	for _, prod := range g.prods(name) {
		empty := true
		for _, sym := range prod {
			if sym.Rule == "" || !g.nullable(sym.Rule, seen) {
				empty = false
				break
			}
		}
		if empty {
			return true
		}
	}
	return false
}

// matches reports whether a word fills a terminal symbol.
func matches(lex *Lexicon, sym Sym, word string) bool {
	if sym.Open {
		return lex.Is(word, "open")
	}
	return sym.Class != "" && lex.Is(word, sym.Class)
}

// itemSet keeps a chart column's items in the order they arrived, and refuses
// a duplicate so the loop terminates.
type itemSet struct {
	order []item
	seen  map[item]bool
}

func newItemSet() *itemSet { return &itemSet{seen: map[item]bool{}} }

func (s *itemSet) add(it item) {
	if s.seen[it] {
		return
	}
	s.seen[it] = true
	s.order = append(s.order, it)
}

// Words splits prose into the word runs the parser reads.
func Words(prose string) []string {
	var out []string
	for i := 0; i < len(prose); {
		if !isWordByte(prose[i]) {
			i++
			continue
		}
		start := i
		for i < len(prose) && isWordByte(prose[i]) {
			i++
		}
		out = append(out, strings.ToLower(prose[start:i]))
	}
	return out
}
