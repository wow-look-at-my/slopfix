package askproperly

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Every deferral must report its own phrase on each example. A row nothing
// can fire reads as coverage.
func TestEveryDeferralFiresOnItsExamples(t *testing.T) {
	for _, d := range Rules.Deferrals {
		for _, example := range d.Examples {
			var found []string
			for _, h := range FindQuestions(example) {
				if h.Kind == "deferral" {
					found = append(found, h.Text)
				}
			}
			assert.Contains(t, found, d.Phrase, "example %q", example)
		}
	}
}

func TestAMalformedTableIsAnError(t *testing.T) {
	for name, raw := range map[string]string{
		"not XML":       `<questions>`,
		"empty":         `<questions></questions>`,
		"no example":    `<questions><deferral phrase="x"/><cue word="is"/></questions>`,
		"no phrase":     `<questions><deferral><example>x</example></deferral><cue word="is"/></questions>`,
		"no cue":        `<questions><deferral phrase="x"><example>x</example></deferral></questions>`,
		"empty cue":     `<questions><deferral phrase="x"><example>x</example></deferral><cue/></questions>`,
	} {
		_, err := Load([]byte(raw))
		assert.Error(t, err, name)
	}
}
