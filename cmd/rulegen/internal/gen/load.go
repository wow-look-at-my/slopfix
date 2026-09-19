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

// Drop is a word that survives its own deletion.
type Drop struct {
	Word  string `xml:"word,attr"`
	Where string `xml:"where,attr"`
	Test  string `xml:"test,attr"`
}

// Rewrite swaps a whole phrase for another.
type Rewrite struct {
	From   string `xml:"from,attr"`
	To     string `xml:"to,attr"`
	Where  string `xml:"where,attr"`
	Test   string `xml:"test,attr"`
	Expect string `xml:"expect,attr"`
}

// Pattern is a rewrite with captures. Prefix, LeadWord and TailWord are filled
// in by the compile step.
type Pattern struct {
	Match  string `xml:"match,attr"`
	To     string `xml:"to,attr"`
	Where  string `xml:"where,attr"`
	Test   string `xml:"test,attr"`
	Expect string `xml:"expect,attr"`

	Prefix   string
	LeadWord bool
	TailWord bool
}

// Flag names prose a rule refuses to rewrite.
type Flag struct {
	Phrase string `xml:"phrase,attr"`
	Say    string `xml:"say,attr"`
	Test   string `xml:"test,attr"`
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
	Match  string `xml:"match,attr"`
	To     string `xml:"to,attr"`
	Where  string `xml:"where,attr"`
	Test   string `xml:"test,attr"`
	Expect string `xml:"expect,attr"`
}

// Normalize settles a spelling before any match is tried.
type Normalize struct {
	From string `xml:"from,attr"`
	To   string `xml:"to,attr"`
	Test string `xml:"test,attr"`
}

// file mirrors a rules XML document.
type file struct {
	For      string    `xml:"for,attr"`
	Drops    []Drop    `xml:"drop"`
	Rewrites []Rewrite `xml:"rewrite"`
	Patterns []Pattern `xml:"pattern"`
	Flags    []Flag    `xml:"flag"`
	Classes  []Class   `xml:"class"`
	Rephrases []Rephrase `xml:"rephrase"`
	Normals   []Normalize `xml:"normalize"`
}

// Loaded is the folder's entries for a single target, in order.
type Loaded struct {
	Drops    []Drop
	Rewrites []Rewrite
	Patterns []Pattern
	Flags    []Flag
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
		if err := validate(path, parsed); err != nil {
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

// validate refuses an entry that cannot be driven. A test attribute is what
// keeps the table honest, so an entry carrying none never lands.
func validate(path string, f file) error {
	for _, d := range f.Drops {
		if d.Word == "" || d.Test == "" {
			return fmt.Errorf("%s: a <drop> is missing its word or its test", path)
		}
	}
	for _, r := range f.Rewrites {
		if r.From == "" || r.To == "" || r.Test == "" {
			return fmt.Errorf("%s: a <rewrite> is missing from, to or test", path)
		}
	}
	for _, p := range f.Patterns {
		if p.Match == "" || p.Test == "" || p.Expect == "" {
			return fmt.Errorf("%s: a <pattern> is missing match, test or expect", path)
		}
	}
	for _, fl := range f.Flags {
		if fl.Phrase == "" || fl.Say == "" || fl.Test == "" {
			return fmt.Errorf("%s: a <flag> is missing phrase, say or test", path)
		}
	}
	for _, r := range f.Rephrases {
		if r.Match == "" || r.Test == "" || r.Expect == "" {
			return fmt.Errorf("%s: a <rephrase> is missing match, test or expect", path)
		}
		if _, err := table.ParseMatch(r.Match); err != nil {
			return fmt.Errorf("%s: <rephrase match=%q>: %w", path, r.Match, err)
		}
	}
	for _, n := range f.Normals {
		if n.From == "" || n.To == "" {
			return fmt.Errorf("%s: a <normalize> is missing from or to", path)
		}
	}
	return nil
}
