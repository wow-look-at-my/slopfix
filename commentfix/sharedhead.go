package commentfix

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/ste"
)

// sharedHeadTails answers the sentences that keep the verb in front of the
// first "that" and one later ", and that" clause. "The caller must ensure that
// A, and that B." gives "The caller must ensure that B." The longest tail comes
// first, and only a tail that stands alone as a sentence is answered.
func sharedHeadTails(sentence string) []string {
	const joint = ", and that "
	first := strings.Index(sentence, " that ")
	if first < 0 {
		return nil
	}
	lead := sentence[:first+len(" that ")]
	var out []string
	for from := first + len(" that "); ; {
		at := strings.Index(sentence[from:], joint)
		if at < 0 {
			return out
		}
		from += at + len(joint)
		if tail := lead + sentence[from:]; ste.StandsAlone(tail) {
			out = append(out, tail)
		}
	}
}
