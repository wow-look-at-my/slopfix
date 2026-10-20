// A file declares its consumer with the `for` attribute on its root. The folder
// is read in file name order and then document order: a longer phrase that must
// beat a shorter phrase sits above it. What an entry must carry to be usable is
// decided here rather than in the consumer. A malformed entry stops the program
// at its earliest read instead of quietly matching nothing.
package table

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/wow-look-at-my/slopfix/memo"
	"github.com/wow-look-at-my/slopfix/precompiled"
	"github.com/wow-look-at-my/slopfix/xmlcheck"
)

// document mirrors a rules file. The table types the consumer reads carry no
// XML tags of their own: a class states its words in a single attribute and holds
// them as a list. A pattern holds a compiled matcher the file cannot spell.
type document struct {
	For       string      `xml:"for,attr"`
	Case      string      `xml:"case,attr"`
	Drops     []xmlDrop   `xml:"drop"`
	Rewrites  []xmlPhrase `xml:"rewrite"`
	Patterns  []xmlShape  `xml:"pattern"`
	Flags     []xmlFlag   `xml:"flag"`
	Classes   []xmlClass  `xml:"class"`
	Rephrases []xmlShape  `xml:"rephrase"`
	Normals   []xmlPhrase `xml:"normalize"`
	Tests     []xmlTest   `xml:"test"`
	Detects   []xmlDetect `xml:"detect"`
	Lists     []xmlList   `xml:"list"`
}

type xmlTest struct {
	In  string `xml:"in,attr"`
	Out string `xml:"out,attr"`
}

// xmlDetect states what a single substrate reports for a line. The
// phrases are children rather than an attribute, because a phrase carries spaces.
type xmlDetect struct {
	In        string     `xml:"in,attr"`
	Substrate string     `xml:"substrate,attr"`
	Why       string     `xml:"why,attr"`
	Found     []xmlFound `xml:"found"`
}

type xmlFound struct {
	Text string `xml:"text,attr"`
}

type xmlDrop struct {
	ID    string    `xml:"id,attr"`
	Word  string    `xml:"word,attr"`
	Where string    `xml:"where,attr"`
	Case  string    `xml:"case,attr"`
	Tests []xmlTest `xml:"test"`
}

// xmlPhrase is a rewrite and a normalize alike: both name a phrase and what
// stands in for it.
type xmlPhrase struct {
	ID    string    `xml:"id,attr"`
	From  string    `xml:"from,attr"`
	To    string    `xml:"to,attr"`
	Where string    `xml:"where,attr"`
	Case  string    `xml:"case,attr"`
	Tests []xmlTest `xml:"test"`
}

// xmlShape is a pattern and a rephrase alike: both name a match and what to
// write for it.
type xmlShape struct {
	ID    string    `xml:"id,attr"`
	Match string    `xml:"match,attr"`
	To    string    `xml:"to,attr"`
	Where string    `xml:"where,attr"`
	Case  string    `xml:"case,attr"`
	Tests []xmlTest `xml:"test"`
}

type xmlFlag struct {
	ID     string    `xml:"id,attr"`
	Phrase string    `xml:"phrase,attr"`
	Say    string    `xml:"say,attr"`
	Case   string    `xml:"case,attr"`
	Tests  []xmlTest `xml:"test"`
}

// sensitive answers the case rule of an entry: its own case attribute, else the file's.
func sensitive(root, entry string) bool {
	if entry != "" {
		return entry == xmlcheck.CaseSensitive
	}
	return root == xmlcheck.CaseSensitive
}

// xmlList holds one string on each line of its text.
type xmlList struct {
	Name string `xml:"name,attr"`
	Body string `xml:",chardata"`
}

type xmlClass struct {
	Name   string `xml:"name,attr"`
	Words  string `xml:"words,attr"`
	Suffix string `xml:"suffix,attr"`
	Digits bool   `xml:"digits,attr"`
	Open   bool   `xml:"open,attr"`
	Except string `xml:"except,attr"`
}

// loaded is a rules file and the name it was read under.
type loaded struct {
	path string
	raw  []byte
}

// Load reads every XML in fsys and returns what a single consumer declares.
func Load(fsys fs.FS, target string) (*Table, error) {
	paths, err := fs.Glob(fsys, "*.xml")
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)

	out := &Table{}
	var files []xmlcheck.File
	var docs []loaded
	for _, path := range paths {
		raw, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, err
		}
		// Only the for= attribute is read here: the schema is the authority on the
		// shape of a file.
		var head struct {
			For string `xml:"for,attr"`
		}
		if err := xml.Unmarshal(Readable(raw), &head); err != nil {
			return nil, fmt.Errorf("%s does not parse: %w", path, err)
		}
		if head.For == "" {
			return nil, fmt.Errorf("%s: the root element needs a for= naming its consumer", path)
		}
		if head.For != target {
			continue
		}
		docs = append(docs, loaded{path: path, raw: raw})
		files = append(files, xmlcheck.File{Name: path, Raw: raw})
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no file in the rules folder carries for=%q", target)
	}
	if err := xmlcheck.Run(SpecFor(target), files...); err != nil {
		return nil, fmt.Errorf("for=%q: %w", target, err)
	}
	// An id names an entry across the whole folder, so a duplicate is caught over every file rather than over each alone.
	ids := map[string]string{}
	for _, d := range docs {
		parsed := decoded.Get(string(d.raw), func() decoding {
			var doc document
			err := xml.Unmarshal(Readable(d.raw), &doc)
			return decoding{doc, err}
		})
		if parsed.err != nil {
			return nil, fmt.Errorf("%s does not parse: %w", d.path, parsed.err)
		}
		if err := out.add(d.path, parsed.doc, ids); err != nil {
			return nil, err
		}
	}
	if out.empty() {
		return nil, fmt.Errorf("no file in the rules folder carries for=%q", target)
	}
	if err := out.checkRephrasings(); err != nil {
		return nil, fmt.Errorf("for=%q: %w", target, err)
	}
	return out, nil
}

// checkRephrasings holds each rephrase to its own tests: a test that changes
// the prose must be prose the entry's match fits.
func (t *Table) checkRephrasings() error {
	lex := t.Lexicon()
	for _, r := range t.Rephrasings {
		for _, c := range r.Tests {
			if c.Out != "" && c.Out == c.In {
				continue
			}
			if !r.Matches(lex, t.Normals, c.In) {
				return fmt.Errorf("<rephrase id=%q>: the test %q is matched by nothing in its match %q", r.ID, c.In, r.Match)
			}
		}
	}
	return nil
}

// decoded keeps each file's document, because every consumer reads every file.
var decoded = memo.New[string, decoding](64)

type decoding struct {
	doc document
	err error
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
	named := func(id string) error {
		if where, taken := ids[id]; taken {
			return fmt.Errorf("%s: the id %q is already used in %s", path, id, where)
		}
		ids[id] = path
		return nil
	}

	for _, d := range doc.Drops {
		if err := named(d.ID); err != nil {
			return err
		}
		t.Drops = append(t.Drops, Drop{ID: d.ID, Word: d.Word, Where: d.Where, Tests: tests(d.Tests), Sensitive: sensitive(doc.Case, d.Case)})
	}
	for _, r := range doc.Rewrites {
		if err := named(r.ID); err != nil {
			return err
		}
		t.Rewrites = append(t.Rewrites, Rewrite{
			ID: r.ID, From: r.From, To: r.To, Where: r.Where, Tests: tests(r.Tests), Sensitive: sensitive(doc.Case, r.Case),
		})
	}
	for _, p := range doc.Patterns {
		if err := named(p.ID); err != nil {
			return err
		}
		sens := sensitive(doc.Case, p.Case)
		re, err := precompiled.Compile(xmlcheck.Pattern(p.Match, sens))
		if err != nil {
			return fmt.Errorf("%s: <pattern id=%q>: %w", path, p.ID, err)
		}
		t.Patterns = append(t.Patterns, Pattern{
			ID: p.ID, Match: p.Match, To: p.To, Where: p.Where, Tests: tests(p.Tests), Sensitive: sens, re: re, needs: precompiled.Needs(re),
		})
	}
	for _, f := range doc.Flags {
		if err := named(f.ID); err != nil {
			return err
		}
		t.Flags = append(t.Flags, Flag{ID: f.ID, Phrase: f.Phrase, Say: f.Say, Tests: tests(f.Tests), Sensitive: sensitive(doc.Case, f.Case)})
	}
	for _, c := range doc.Classes {
		t.Classes = append(t.Classes, Class{
			Name:   c.Name,
			Words:  strings.Fields(c.Words),
			Suffix: strings.Fields(c.Suffix),
			Digits: c.Digits,
			Open:   c.Open,
			Except: strings.Fields(c.Except),
		})
	}
	for _, r := range doc.Rephrases {
		if err := named(r.ID); err != nil {
			return err
		}
		terms, err := ParseMatch(r.Match)
		if err != nil {
			return fmt.Errorf("%s: <rephrase id=%q>: %w", path, r.ID, err)
		}
		terms.Sensitive = sensitive(doc.Case, r.Case)
		t.Rephrasings = append(t.Rephrasings, Rephrase{
			ID: r.ID, Match: r.Match, Terms: terms, To: r.To, Where: r.Where, Tests: tests(r.Tests),
		})
	}
	for _, n := range doc.Normals {
		if err := named(n.ID); err != nil {
			return err
		}
		t.Normals = append(t.Normals, Normalize{ID: n.ID, From: n.From, To: n.To, Tests: tests(n.Tests)})
	}
	t.Tests = append(t.Tests, tests(doc.Tests)...)
	for _, d := range doc.Detects {
		found := make([]string, 0, len(d.Found))
		for _, f := range d.Found {
			found = append(found, f.Text)
		}
		t.Detects = append(t.Detects, Detect{In: d.In, Substrate: d.Substrate, Why: d.Why, Found: found})
	}
	for _, l := range doc.Lists {
		if err := t.addList(path, l); err != nil {
			return err
		}
	}
	return nil
}

// addList reads a list's text as one item on each line. A blank line holds no
// item. An empty list and a reused name are errors, because a rule that reads
// either reads nothing.
func (t *Table) addList(path string, l xmlList) error {
	if _, taken := t.Lists[l.Name]; taken {
		return fmt.Errorf("%s: the list %q is already defined", path, l.Name)
	}
	var items []string
	for _, line := range strings.Split(l.Body, "\n") {
		if item := strings.TrimSpace(line); item != "" {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return fmt.Errorf("%s: <list name=%q> holds no item", path, l.Name)
	}
	if t.Lists == nil {
		t.Lists = map[string][]string{}
	}
	t.Lists[l.Name] = items
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
		len(t.Classes)+len(t.Rephrasings)+len(t.Normals)+len(t.Tests)+len(t.Detects)+len(t.Lists) == 0
}
