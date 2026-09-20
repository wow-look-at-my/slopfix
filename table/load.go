// A file declares its consumer with the `for` attribute on its root, and the
// folder is read in file name order and then document order: a longer phrase
// that must beat a shorter phrase sits above it. What an entry must carry to be
// usable is decided here rather than in the consumer, so a malformed entry
// stops the program at its earliest read instead of quietly matching nothing.
package table

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
)

// document mirrors a rules file. The table types the consumer reads carry no
// XML tags of their own: a class states its words in a single attribute and
// holds them as a list, and a pattern holds a compiled matcher the file cannot spell.
type document struct {
	For       string      `xml:"for,attr"`
	Drops     []xmlDrop   `xml:"drop"`
	Rewrites  []xmlPhrase `xml:"rewrite"`
	Patterns  []xmlShape  `xml:"pattern"`
	Flags     []xmlFlag   `xml:"flag"`
	Classes   []xmlClass  `xml:"class"`
	Rephrases []xmlShape  `xml:"rephrase"`
	Normals   []xmlPhrase `xml:"normalize"`
	Tests     []xmlTest   `xml:"test"`
}

type xmlTest struct {
	In  string `xml:"in,attr"`
	Out string `xml:"out,attr"`
}

type xmlDrop struct {
	ID    string    `xml:"id,attr"`
	Word  string    `xml:"word,attr"`
	Where string    `xml:"where,attr"`
	Tests []xmlTest `xml:"test"`
}

// xmlPhrase is a rewrite and a normalize alike: both name a phrase and what
// stands in for it.
type xmlPhrase struct {
	ID    string    `xml:"id,attr"`
	From  string    `xml:"from,attr"`
	To    string    `xml:"to,attr"`
	Where string    `xml:"where,attr"`
	Tests []xmlTest `xml:"test"`
}

// xmlShape is a pattern and a rephrase alike: both name a match and what to
// write for it.
type xmlShape struct {
	ID    string    `xml:"id,attr"`
	Match string    `xml:"match,attr"`
	To    string    `xml:"to,attr"`
	Where string    `xml:"where,attr"`
	Tests []xmlTest `xml:"test"`
}

type xmlFlag struct {
	ID     string    `xml:"id,attr"`
	Phrase string    `xml:"phrase,attr"`
	Say    string    `xml:"say,attr"`
	Tests  []xmlTest `xml:"test"`
}

type xmlClass struct {
	Name   string `xml:"name,attr"`
	Words  string `xml:"words,attr"`
	Suffix string `xml:"suffix,attr"`
	Open   bool   `xml:"open,attr"`
	Except string `xml:"except,attr"`
}

// Load reads every XML in fsys and returns what a single consumer declares.
func Load(fsys fs.FS, target string) (*Table, error) {
	paths, err := fs.Glob(fsys, "*.xml")
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)

	out := &Table{}
	// An id names an entry across the whole folder, so a duplicate is caught over every file rather than over each alone.
	ids := map[string]string{}
	for _, path := range paths {
		raw, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, err
		}
		var doc document
		if err := xml.Unmarshal(Readable(raw), &doc); err != nil {
			return nil, fmt.Errorf("%s does not parse: %w", path, err)
		}
		if doc.For == "" {
			return nil, fmt.Errorf("%s: the root element needs a for= naming its consumer", path)
		}
		if doc.For != target {
			continue
		}
		if err := out.add(path, doc, ids); err != nil {
			return nil, err
		}
	}
	if out.empty() {
		return nil, fmt.Errorf("no file in the rules folder carries for=%q", target)
	}
	return out, nil
}

// MustLoad is Load for a package-level table. A rules folder that does not load
// is a broken binary rather than a condition a caller can answer.
func MustLoad(fsys fs.FS, target string) *Table {
	loaded, err := Load(fsys, target)
	if err != nil {
		panic(err)
	}
	return loaded
}

// add appends a file's entries, holding each to what it needs to fire.
func (t *Table) add(path string, doc document, ids map[string]string) error {
	named := func(kind, id string, tests []xmlTest, missing bool) error {
		if missing {
			return fmt.Errorf("%s: a <%s> is missing what it needs to fire", path, kind)
		}
		if id == "" {
			return fmt.Errorf("%s: a <%s> carries no id", path, kind)
		}
		if where, taken := ids[id]; taken {
			return fmt.Errorf("%s: the id %q is already used in %s", path, id, where)
		}
		ids[id] = path
		if len(tests) == 0 {
			return fmt.Errorf("%s: <%s id=%q> carries no <test>", path, kind, id)
		}
		for _, c := range tests {
			if c.In == "" {
				return fmt.Errorf("%s: <%s id=%q> has a <test> with no in", path, kind, id)
			}
		}
		return nil
	}

	for _, d := range doc.Drops {
		if err := named("drop", d.ID, d.Tests, d.Word == ""); err != nil {
			return err
		}
		t.Drops = append(t.Drops, Drop{ID: d.ID, Word: d.Word, Where: d.Where, Tests: tests(d.Tests)})
	}
	for _, r := range doc.Rewrites {
		if err := named("rewrite", r.ID, r.Tests, r.From == "" || r.To == ""); err != nil {
			return err
		}
		t.Rewrites = append(t.Rewrites, Rewrite{ID: r.ID, From: r.From, To: r.To, Where: r.Where, Tests: tests(r.Tests)})
	}
	for _, p := range doc.Patterns {
		if err := named("pattern", p.ID, p.Tests, p.Match == ""); err != nil {
			return err
		}
		re, err := regexp.Compile(p.Match)
		if err != nil {
			return fmt.Errorf("%s: <pattern id=%q>: %w", path, p.ID, err)
		}
		t.Patterns = append(t.Patterns, Pattern{
			ID: p.ID, Match: p.Match, To: p.To, Where: p.Where, Tests: tests(p.Tests), re: re,
		})
	}
	for _, f := range doc.Flags {
		if err := named("flag", f.ID, f.Tests, f.Phrase == "" || f.Say == ""); err != nil {
			return err
		}
		t.Flags = append(t.Flags, Flag{ID: f.ID, Phrase: f.Phrase, Say: f.Say, Tests: tests(f.Tests)})
	}
	for _, c := range doc.Classes {
		if c.Name == "" {
			return fmt.Errorf("%s: a <class> carries no name", path)
		}
		t.Classes = append(t.Classes, Class{
			Name:   c.Name,
			Words:  strings.Fields(c.Words),
			Suffix: strings.Fields(c.Suffix),
			Open:   c.Open,
			Except: strings.Fields(c.Except),
		})
	}
	for _, r := range doc.Rephrases {
		if err := named("rephrase", r.ID, r.Tests, r.Match == ""); err != nil {
			return err
		}
		terms, err := ParseMatch(r.Match)
		if err != nil {
			return fmt.Errorf("%s: <rephrase id=%q>: %w", path, r.ID, err)
		}
		t.Rephrasings = append(t.Rephrasings, Rephrase{
			ID: r.ID, Match: r.Match, Terms: terms, To: r.To, Where: r.Where, Tests: tests(r.Tests),
		})
	}
	for _, n := range doc.Normals {
		if err := named("normalize", n.ID, n.Tests, n.From == "" || n.To == ""); err != nil {
			return err
		}
		t.Normals = append(t.Normals, Normalize{ID: n.ID, From: n.From, To: n.To, Tests: tests(n.Tests)})
	}
	for _, c := range doc.Tests {
		if c.In == "" {
			return fmt.Errorf("%s: a <test> under <rules> carries no in", path)
		}
	}
	t.Tests = append(t.Tests, tests(doc.Tests)...)
	return nil
}

func tests(in []xmlTest) []Test {
	out := make([]Test, 0, len(in))
	for _, c := range in {
		out = append(out, Test{In: c.In, Out: c.Out})
	}
	return out
}

func (t *Table) empty() bool {
	return len(t.Drops)+len(t.Rewrites)+len(t.Patterns)+len(t.Flags)+
		len(t.Classes)+len(t.Rephrasings)+len(t.Normals)+len(t.Tests) == 0
}
