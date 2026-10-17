// The shapes, patterns and pardons come from laziness.xml. Extending the rule
// is adding a pattern there, with a punt that proves it fires.
package laziness

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"

	"github.com/wow-look-at-my/slopfix/table"
)

//go:embed laziness.xml
var lazinessXML []byte

// Pattern is a row of laziness.xml: a regular expression and the punts that
// prove it fires.
type Pattern struct {
	Shape string
	Re    *regexp.Regexp
	Punts []string
}

// Pardon is a regular expression that clears a sentence, with the messages
// it must clear.
type Pardon struct {
	Re    *regexp.Regexp
	Clean []string
}

// Table is laziness.xml, compiled.
type Table struct {
	// Answer is what the Stop hook sends back on a punt.
	Answer   string
	Patterns []Pattern
	Pardons  []Pardon
	// Clean holds messages that own the work. None may fire.
	Clean []string
}

type xmlTable struct {
	Answer string `xml:"answer,attr"`
	Shapes []struct {
		Name     string `xml:"name,attr"`
		Patterns []struct {
			Re    string   `xml:"re,attr"`
			Punts []string `xml:"punt"`
		} `xml:"pattern"`
	} `xml:"shape"`
	Pardons []struct {
		Re    string   `xml:"re,attr"`
		Clean []string `xml:"clean"`
	} `xml:"pardon"`
	Clean []string `xml:"clean"`
}

// Rules is the table the binary judges with.
var Rules = mustLoad(lazinessXML)

var tells, pardons = Rules.Patterns, Rules.Pardons

func mustLoad(raw []byte) Table {
	t, err := Load(raw)
	if err != nil {
		panic("laziness.xml: " + err.Error())
	}
	return t
}

// Load compiles a table in the laziness.xml format.
func Load(raw []byte) (Table, error) {
	var doc xmlTable
	if err := xml.Unmarshal(table.Readable(raw), &doc); err != nil {
		return Table{}, err
	}
	t := Table{Answer: strings.TrimSpace(doc.Answer)}
	if t.Answer == "" {
		return Table{}, fmt.Errorf("no answer")
	}
	for _, s := range doc.Shapes {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			return Table{}, fmt.Errorf("a shape has no name")
		}
		for _, p := range s.Patterns {
			re, err := regexp.Compile(p.Re)
			if err != nil {
				return Table{}, fmt.Errorf("shape %q: %w", name, err)
			}
			if len(p.Punts) == 0 {
				return Table{}, fmt.Errorf("shape %q: pattern %q has no punt", name, p.Re)
			}
			t.Patterns = append(t.Patterns, Pattern{Shape: name, Re: re, Punts: trimAll(p.Punts)})
		}
	}
	if len(t.Patterns) == 0 {
		return Table{}, fmt.Errorf("no pattern")
	}
	for _, p := range doc.Pardons {
		re, err := regexp.Compile(p.Re)
		if err != nil {
			return Table{}, fmt.Errorf("pardon: %w", err)
		}
		t.Pardons = append(t.Pardons, Pardon{Re: re, Clean: trimAll(p.Clean)})
	}
	t.Clean = trimAll(doc.Clean)
	return t, nil
}

func trimAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, strings.TrimSpace(s))
	}
	return out
}
