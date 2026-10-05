package ste

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// TestTracePrefillCut names the path the prefillBatch second half takes.
func TestTracePrefillCut(t *testing.T) {
	in := "The chunked request's next chunk first, capped by the cede and the balancer's token budget, then the waiting queue in order until a request cannot be added."
	masked := mask(in)
	whole := syntax.Parse(masked, nil)
	var b strings.Builder
	fmt.Fprintf(&b, "Fix=%q\n", Fix(in))
	fmt.Fprintf(&b, "words=%d ends=%d\n", len(whole.Words), len(wordEnds(masked)))
	for _, c := range candidates(in, masked, false, SentenceWordCap) {
		n := wordsBefore(whole, c.left)
		next := ""
		if n < len(whole.Words) {
			next = whole.Words[n].Lower() + "/" + whole.Words[n].Tag
		}
		fmt.Fprintf(&b, "cut@%d n=%d seam=%q next=%s list=%v opensFrag=%v\n", c.left, n, seamBefore(in, c.left), next, n < len(whole.Words) && listAdverb.Contains(whole.Words[n].Lower()), n < len(whole.Words) && opensFragment(whole.Words[n]))
	}
	fd, ok := fragmentDivision(in, masked, whole, SentenceWordCap)
	fmt.Fprintf(&b, "fragmentDivision ok=%v out=%q\n", ok, fd)
	f, fok := forceDivision(in, masked, capSpec{reorder: true, cap: SentenceWordCap})
	fmt.Fprintf(&b, "forceDivision ok=%v out=%q\n", fok, f)
	require.NoError(t, os.WriteFile("../.scratch/trace2.txt", []byte(b.String()), 0o644))

}
