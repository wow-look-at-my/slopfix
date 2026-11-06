// Deferral is a phrase that hands the reader a decision, with the messages that must report it.

package askproperly

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/wow-look-at-my/slopfix/table"
)

//go:embed questions.xml
var questionsXML []byte

// Deferral is a phrase that hands the reader a decision, with the messages
// that must report it.
type Deferral struct {
	Phrase   string
	Examples []string
}

// Table is questions.xml, read.
type Table struct {
	Deferrals []Deferral
	Cues      []string
}

// DeferralPhrases lists each phrase in file order.
func (t Table) DeferralPhrases() []string {
	out := make([]string, 0, len(t.Deferrals))
	for _, d := range t.Deferrals {
		out = append(out, d.Phrase)
	}
	return out
}

type xmlTable struct {
	Deferrals []struct {
		Phrase   string   `xml:"phrase,attr"`
		Examples []string `xml:"example"`
	} `xml:"deferral"`
	Cues []struct {
		Word string `xml:"word,attr"`
	} `xml:"cue"`
}

// Rules is the table the binary judges with. A malformed file stops the binary at its first start.
var Rules = mustLoad(questionsXML)

func mustLoad(raw []byte) Table {
	t, err := Load(raw)
	if err != nil {
		panic("questions.xml: " + err.Error())
	}
	return t
}

// Load reads a table in the questions.xml format. Phrases and cues match
// without regard to case, so both are stored in lower case.
func Load(raw []byte) (Table, error) {
	var doc xmlTable
	if err := xml.Unmarshal(table.Readable(raw), &doc); err != nil {
		return Table{}, err
	}
	var t Table
	for _, d := range doc.Deferrals {
		phrase := strings.ToLower(strings.TrimSpace(d.Phrase))
		if phrase == "" {
			return Table{}, fmt.Errorf("a deferral has no phrase")
		}
		if len(d.Examples) == 0 {
			return Table{}, fmt.Errorf("deferral %q has no example", phrase)
		}
		examples := make([]string, 0, len(d.Examples))
		for _, e := range d.Examples {
			examples = append(examples, strings.TrimSpace(e))
		}
		t.Deferrals = append(t.Deferrals, Deferral{Phrase: phrase, Examples: examples})
	}
	for _, c := range doc.Cues {
		word := strings.ToLower(strings.TrimSpace(c.Word))
		if word == "" {
			return Table{}, fmt.Errorf("a cue has no word")
		}
		t.Cues = append(t.Cues, word)
	}
	if len(t.Deferrals) == 0 || len(t.Cues) == 0 {
		return Table{}, fmt.Errorf("the table needs a deferral and a cue")
	}
	return t, nil
}
