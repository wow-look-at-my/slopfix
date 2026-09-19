// load.go reads the folder. What a file says, and what an entry must carry to
// be usable, is decided here rather than in the consumer: a malformed entry
// fails the generate step, where somebody is looking.
package gen

import (
	"encoding/xml"
	"fmt"
	"os"

	"github.com/wow-look-at-my/slopfix/table"
)

// Test drives an entry. Out is what the repair must produce, and an entry that
// asserts no particular output leaves it empty.
type Test struct {
	In  string `xml:"in,attr"`
	Out string `xml:"out,attr"`
}

// Drop is a word that survives its own deletion.
type Drop struct {
	ID    string `xml:"id,attr"`
	Word  string `xml:"word,attr"`
	Where string `xml:"where,attr"`
	Tests []Test `xml:"test"`
}

// Rewrite swaps a whole phrase for another.
type Rewrite struct {
	ID    string `xml:"id,attr"`
	From  string `xml:"from,attr"`
	To    string `xml:"to,attr"`
	Where string `xml:"where,attr"`
	Tests []Test `xml:"test"`
}

// Pattern is a rewrite with captures. Prefix, LeadWord and TailWord are filled
// in by the compile step.
type Pattern struct {
	ID    string `xml:"id,attr"`
	Match string `xml:"match,attr"`
	To    string `xml:"to,attr"`
	Where string `xml:"where,attr"`
	Tests []Test `xml:"test"`

	Prefix   string
	LeadWord bool
	TailWord bool
}

// Flag names prose a rule refuses to rewrite.
type Flag struct {
	ID     string `xml:"id,attr"`
	Phrase string `xml:"phrase,attr"`
	Say    string `xml:"say,attr"`
	Tests  []Test `xml:"test"`
}

// Class is a set of words that fill the same slot, declared in its own file.
type Class struct {
	Name   string `xml:"name,attr"`
	Words  string `xml:"words,attr"`
	Suffix string `xml:"suffix,attr"`
	Open   bool   `xml:"open,attr"`
	Except string `xml:"except,attr"`
}

// Rephrase is a match over word classes and what to write instead.
type Rephrase struct {
	ID    string `xml:"id,attr"`
	Match string `xml:"match,attr"`
	To    string `xml:"to,attr"`
	Where string `xml:"where,attr"`
	Tests []Test `xml:"test"`
}

// Normalize settles a spelling before any match is tried.
type Normalize struct {
	ID    string `xml:"id,attr"`
	From  string `xml:"from,attr"`
	To    string `xml:"to,attr"`
	Tests []Test `xml:"test"`
}

// file mirrors a rules XML document.
type file struct {
	For       string      `xml:"for,attr"`
	Drops     []Drop      `xml:"drop"`
	Rewrites  []Rewrite   `xml:"rewrite"`
	Patterns  []Pattern   `xml:"pattern"`
	Flags     []Flag      `xml:"flag"`
	Classes   []Class     `xml:"class"`
	Rephrases []Rephrase  `xml:"rephrase"`
	Normals   []Normalize `xml:"normalize"`
}

// Loaded is the folder's entries for a single target, in order.
type Loaded struct {
	Drops     []Drop
	Rewrites  []Rewrite
	Patterns  []Pattern
	Flags     []Flag
	Classes   []Class
	Rephrases []Rephrase
	Normals   []Normalize
}

func (l *Loaded) empty() bool {
	return len(l.Drops)+len(l.Rewrites)+len(l.Patterns)+len(l.Flags)+
		len(l.Classes)+len(l.Rephrases)+len(l.Normals) == 0
}

// Load reads every XML in dir and keeps the entries declaring this target.
func Load(dir, target string) (*Loaded, error) {
	paths, err := sortedXML(dir)
	if err != nil {
		return nil, err
	}
	out := &Loaded{}
	// An id names an entry across the whole folder, so the check for a
	// duplicate spans every file rather than each one alone.
	ids := map[string]string{}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var parsed file
		if err := xml.Unmarshal(raw, &parsed); err != nil {
			return nil, fmt.Errorf("%s does not parse: %w", path, err)
		}
		if parsed.For == "" {
			return nil, fmt.Errorf(`%s: the root element needs a for= naming its consumer`, path)
		}
		if parsed.For != target {
			continue
		}
		if err := validate(path, parsed, ids); err != nil {
			return nil, err
		}
		out.Drops = append(out.Drops, parsed.Drops...)
		out.Rewrites = append(out.Rewrites, parsed.Rewrites...)
		out.Patterns = append(out.Patterns, parsed.Patterns...)
		out.Flags = append(out.Flags, parsed.Flags...)
		out.Classes = append(out.Classes, parsed.Classes...)
		out.Rephrases = append(out.Rephrases, parsed.Rephrases...)
		out.Normals = append(out.Normals, parsed.Normals...)
	}
	return out, nil
}

// validate refuses an entry that cannot be driven. Every entry carries an id
// somebody can name it by, and at least one <test>, which is what keeps the
// table honest: an entry that has stopped firing says so rather than sitting
// in the file looking enforced.
func validate(path string, f file, ids map[string]string) error {
	check := func(kind, id string, tests []Test, missing bool) error {
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
		for _, t := range tests {
			if t.In == "" {
				return fmt.Errorf("%s: <%s id=%q> has a <test> with no in", path, kind, id)
			}
		}
		return nil
	}

	for _, d := range f.Drops {
		if err := check("drop", d.ID, d.Tests, d.Word == ""); err != nil {
			return err
		}
	}
	for _, r := range f.Rewrites {
		if err := check("rewrite", r.ID, r.Tests, r.From == "" || r.To == ""); err != nil {
			return err
		}
	}
	for _, p := range f.Patterns {
		if err := check("pattern", p.ID, p.Tests, p.Match == ""); err != nil {
			return err
		}
	}
	for _, fl := range f.Flags {
		if err := check("flag", fl.ID, fl.Tests, fl.Phrase == "" || fl.Say == ""); err != nil {
			return err
		}
	}
	for _, r := range f.Rephrases {
		if err := check("rephrase", r.ID, r.Tests, r.Match == ""); err != nil {
			return err
		}
		if _, err := table.ParseMatch(r.Match); err != nil {
			return fmt.Errorf("%s: <rephrase id=%q>: %w", path, r.ID, err)
		}
	}
	for _, n := range f.Normals {
		if err := check("normalize", n.ID, n.Tests, n.From == "" || n.To == ""); err != nil {
			return err
		}
	}
	return nil
}
