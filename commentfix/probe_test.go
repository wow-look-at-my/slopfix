package commentfix

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/counts"
)

func TestProbeLabels(t *testing.T) {
	var b strings.Builder
	for _, prose := range []string{
		"It is slow. Then branch 3 sees the entry already cleared.",
		"The mock with id 1 completes after a short delay.",
		"id 1 completes after a short delay.",
	} {
		for _, tok := range cardinal.Find(prose, cardinal.Comment) {
			e, ok := counts.LabelAt(prose, tok.Offset, nil)
			fmt.Fprintf(&b, "\n%q tok=%q@%d item=%v label=%v %+v", prose, tok.Text, tok.Offset, cardinal.NamesAnItem(prose, tok.Offset), ok, e)
		}
		fmt.Fprintf(&b, "\n  named=%q", nameLabels(prose, nil))
	}
	t.Error(b.String())
}
