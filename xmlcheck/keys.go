package xmlcheck

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// An item is a value an index compares: a part read with the case rule, and a
// part that always compares exactly.
type item struct {
	node  *Node
	fold  string
	exact string
	sens  bool
}

// collides reports items that are one value. Sensitive items differ when their
// case differs. If either is insensitive, case does not separate them.
func collides(a, b item) bool {
	if a.exact != b.exact {
		return false
	}
	if a.sens && b.sens {
		return a.fold == b.fold
	}
	return strings.EqualFold(a.fold, b.fold)
}

// An index holds items by their lowercased value.
type index struct{ by map[string][]item }

func newIndex() *index { return &index{by: map[string][]item{}} }

func bucket(it item) string { return strings.ToLower(it.fold) + "\x00" + it.exact }

// find answers every earlier item the new one collides with.
func (ix *index) find(it item) []item {
	var out []item
	for _, prev := range ix.by[bucket(it)] {
		if collides(prev, it) {
			out = append(out, prev)
		}
	}
	return out
}

func (ix *index) put(it item) {
	k := bucket(it)
	ix.by[k] = append(ix.by[k], it)
}

// scopeOf answers the node whose children share a scope. The children of the
// root share one scope across every file of the table.
func scopeOf(n *Node) *Node {
	if n.Parent == nil || n.Parent.Parent == nil {
		return nil
	}
	return n.Parent
}

type scopeKey struct {
	parent *Node
	elem   string
	def    int
}

// keys reports entries written twice.
func (c *checker) keys(roots []*Node) {
	seen := map[scopeKey]*index{}
	for _, root := range roots {
		walk(root, func(n *Node) {
			for i, key := range c.spec.Elems[n.Name].Keys {
				sk := scopeKey{parent: scopeOf(n), elem: n.Name, def: i}
				ix := seen[sk]
				if ix == nil {
					ix = newIndex()
					seen[sk] = ix
				}
				for _, value := range keyValues(n, key) {
					it := item{node: n, fold: value, exact: exactPart(n, key), sens: n.Sensitive()}
					if prev := ix.find(it); len(prev) > 0 {
						c.add(n, "<%s> %q repeats the one at %s%s", n.Name, value, prev[0].node.Where(), foldNote(prev[0], it))
					}
					ix.put(it)
				}
			}
		})
	}
}

// foldNote says when only case separates values.
func foldNote(prev, it item) string {
	if prev.fold == it.fold {
		return ""
	}
	return " (case does not separate them)"
}

// keyValues answers the keys an element holds under one key definition.
func keyValues(n *Node, key Key) []string {
	vals := make([]string, 0, len(key.Attrs))
	for _, name := range key.Attrs {
		v := attrOrText(n, name)
		if v == "" {
			return nil
		}
		vals = append(vals, v)
	}
	if !key.Split {
		return []string{strings.Join(vals, "\x00")}
	}
	var out []string
	for _, member := range strings.Split(vals[0], ",") {
		if member = strings.TrimSpace(member); member != "" {
			out = append(out, member)
		}
	}
	return out
}

func exactPart(n *Node, key Key) string {
	parts := make([]string, len(key.Exact))
	for i, name := range key.Exact {
		parts[i] = attrOrText(n, name)
	}
	return strings.Join(parts, "\x00")
}

func attrOrText(n *Node, name string) string {
	if name == "#text" {
		return n.Body()
	}
	return strings.TrimSpace(n.Attrs[name])
}

// A testItem is a worked example and the entry it belongs to.
type testItem struct {
	node    *Node
	owner   *Node
	scope   *Node
	in, out string
	haveOut bool
	sens    bool
}

// collectTests reads every worked example: the test elements, and the inline
// test an entry carries on its own attributes.
func (c *checker) collectTests(roots []*Node) []testItem {
	var out []testItem
	for _, root := range roots {
		walk(root, func(n *Node) {
			elem := c.spec.Elems[n.Name]
			if elem.TestAttr != "" && strings.TrimSpace(n.Attrs[elem.TestAttr]) != "" {
				expect, have := n.Attrs[elem.ExpectAttr]
				out = append(out, testItem{node: n, owner: n, scope: n, in: n.Attrs[elem.TestAttr], out: expect, haveOut: have, sens: n.Sensitive()})
			}
			if n.Name != c.spec.Test.Elem || n.Parent == nil || strings.TrimSpace(n.Attrs[c.spec.Test.In]) == "" {
				return
			}
			item := testItem{node: n, scope: scopeOf(n), in: n.Attrs[c.spec.Test.In], sens: n.Sensitive()}
			if n.Parent.Parent != nil {
				item.owner = n.Parent
				item.scope = n.Parent
			}
			item.out, item.haveOut = c.expectation(n)
			out = append(out, item)
		})
	}
	return out
}

// expectation answers what a test says the result is.
func (c *checker) expectation(n *Node) (string, bool) {
	def := c.spec.Test
	parts := make([]string, 0, len(def.Out)+1)
	have := def.EmptyOut
	for _, name := range def.Out {
		v, ok := n.Attrs[name]
		have = have || (ok && v != "")
		parts = append(parts, v)
	}
	if def.Body {
		body := strings.Join(strings.Fields(n.Text), " ")
		have = have || body != ""
		parts = append(parts, body)
	}
	return strings.Join(parts, "\x00"), have
}

// tests reports an example written twice, an example entries answer
// differently, and an example no part of its entry matches.
func (c *checker) tests(roots []*Node) {
	all := c.collectTests(roots)
	perScope := map[*Node]*index{}
	global := newIndex()
	for _, t := range all {
		it := item{node: t.node, fold: t.in, sens: t.sens}
		ix := perScope[t.scope]
		if ix == nil {
			ix = newIndex()
			perScope[t.scope] = ix
		}
		if prev := ix.find(it); len(prev) > 0 {
			c.add(t.node, "the test %q repeats the one at %s%s", t.in, prev[0].node.Where(), foldNote(prev[0], it))
		}
		ix.put(it)
		for _, prev := range global.find(it) {
			c.contradiction(all, t, prev.node)
		}
		global.put(it)
	}
	c.fires(roots, all)
}

// contradiction reports tests of one input that expect different results.
func (c *checker) contradiction(all []testItem, t testItem, other *Node) {
	var prev *testItem
	for i := range all {
		if all[i].node == other {
			prev = &all[i]
			break
		}
	}
	if prev == nil || prev.scope == t.scope || !prev.haveOut || !t.haveOut || prev.out == t.out {
		return
	}
	c.add(t.node, "the test %q expects a different result than the one at %s", t.in, prev.node.Where())
}

// fires reports an example that changes something, and that nothing in its
// entry matches.
func (c *checker) fires(roots []*Node, all []testItem) {
	for _, t := range all {
		if t.haveOut && t.out == t.in {
			continue
		}
		switch {
		case t.owner != nil && c.spec.Fires != nil:
			if !c.spec.Fires(t.owner, t.owner.Sensitive(), t.in) {
				c.add(t.node, "the test %q is matched by nothing in its <%s>", t.in, t.owner.Name)
			}
		case t.owner == nil && c.spec.FiresAny != nil:
			if !c.spec.FiresAny(roots, t.in) {
				c.add(t.node, "the test %q is matched by no entry of the table", t.in)
			}
		}
	}
}

// shadows reports an entry whose phrase holds the phrase of an entry applied
// before it. The earlier entry claims the text, so the later one never fires.
func (c *checker) shadows(roots []*Node) {
	for _, sh := range c.spec.Shadows {
		var seen []*Node
		for _, root := range roots {
			walk(root, func(n *Node) {
				if n.Name != sh.Elem || attrOrText(n, sh.Attr) == "" {
					return
				}
				for _, prev := range seen {
					if shadowedBy(prev, n, sh.Attr) {
						c.add(n, "<%s> %q never fires: the entry at %s is applied first and claims %q", n.Name, attrOrText(n, sh.Attr), prev.Where(), attrOrText(prev, sh.Attr))
						break
					}
				}
				seen = append(seen, n)
			})
		}
	}
}

// shadowedBy reports whether the earlier entry claims every text the later one
// would.
func shadowedBy(earlier, later *Node, attr string) bool {
	e, l := attrOrText(earlier, attr), attrOrText(later, attr)
	if strings.EqualFold(e, l) || !surfacesCover(earlier.Attrs["where"], later.Attrs["where"]) {
		return false
	}
	if earlier.Sensitive() {
		return later.Sensitive() && containsPhrase(l, e)
	}
	return containsPhrase(strings.ToLower(l), strings.ToLower(e))
}

// surfacesCover reports whether every surface the later entry reads is a
// surface the earlier one reads.
func surfacesCover(earlier, later string) bool {
	e, l := strings.Fields(earlier), strings.Fields(later)
	if len(e) == 0 || slices.Contains(e, "both") {
		return true
	}
	if len(l) == 0 || slices.Contains(l, "both") {
		return false
	}
	for _, surface := range l {
		if !slices.Contains(e, surface) {
			return false
		}
	}
	return true
}

// containsPhrase reports whether hay holds needle between word boundaries.
func containsPhrase(hay, needle string) bool {
	for from := 0; from < len(hay); {
		i := strings.Index(hay[from:], needle)
		if i < 0 {
			return false
		}
		at := from + i
		end := at + len(needle)
		if edgeOK(hay[:at], needle, true) && edgeOK(hay[end:], needle, false) {
			return true
		}
		from = at + 1
	}
	return false
}

// edgeOK reports whether the text beside a phrase, on its left or its right,
// is not a word character joined to the phrase's own edge.
func edgeOK(beside, phrase string, left bool) bool {
	if beside == "" || phrase == "" {
		return true
	}
	var next, own rune
	if left {
		next, _ = utf8.DecodeLastRuneInString(beside)
		own, _ = utf8.DecodeRuneInString(phrase)
	} else {
		next, _ = utf8.DecodeRuneInString(beside)
		own, _ = utf8.DecodeLastRuneInString(phrase)
	}
	return !(isWordRune(next) && isWordRune(own))
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
