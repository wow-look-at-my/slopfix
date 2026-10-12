package commentfix

import (
	"fmt"
	"testing"

	"github.com/wow-look-at-my/slopfix/ste"
)

func TestDiagPrint(t *testing.T) {
	cases := map[string]string{
		"A": "package p\n\nfunc f(m string) {\n\t// An org module has no version of its own: the token on the require line\n\t// is a placeholder, and the real version is the head of the branch the\n\t// target follows.\n\t_ = m\n}\n",
		"B": "package p\n\n// " + rep("a very long single opening sentence that will not fit ", 6) + "\nconst p = 1\n",
		"C": "package p\n\n// The cache writes every entry to disk before it answers the caller, because a crash between the answer and the write loses the entry, and the caller then reads a value that the next process cannot find again.\nconst p = 1\n",
		"D_oneLine": "package p\n\n// False when the id is not in this session's catalog at all, which is a\n// different fault from a model that is there and unflagged.\nconst inCatalog = true\n",
		"E_force":  "// Foo names a thing, and then it says a great deal more about that thing, at such length that no width lays it out inside the budget it must meet.\n// A second sentence carries on well past the point.\nconst Foo = 1\n",
		"F_line":   "// Run does the thing, and it says so at a length no single line holds. So the cut lands here, because the words past the floor are what the character half of the rule counts.\nfunc Run() {}\n",
	}
	for name, src := range cases {
		for _, b := range blocks("x.go", src) {
			if b.header {
				continue
			}
			joined := ""
			for _, l := range prose(b.text) {
				joined += stripMarker(l) + " "
			}
			tell, over := judge(b)
			fixed := repair(b)
			_, still := judge(block{text: fixed, codeLines: b.codeLines, codeChars: b.codeChars})
			fmt.Printf("\n=== %s lines=%d chars=%d codeLines=%d codeChars=%d over=%v tell=%q\n", name, b.codeLines, b.codeChars, b.codeLines, b.codeChars, over, tell)
			fmt.Printf("sentences(%d) oneSentence=%v singleThought=%v\n", len(ste.Sentences(joined)), oneSentence(b.text), singleThought(b.text))
			fmt.Printf("fixed:\n%v\n stillOver=%v\n", fixed, still)
			fmt.Printf("sentences(%d) oneSentence=%v singleThought=%v\n", len(ste.Sentences(joined)), oneSentence(b.text), singleThought(b.text))
			for _, para := range paragraphs(b.text) {
				if para.blank || para.verbatim {
					continue
				}
				jt := closeThoughts(para.lines)
				if !endsSentence(jt) {
					jt += "."
				}
				ss := ste.Sentences(ste.FixKeepingOpening(jt))
				if len(ss) > 0 {
					first := ss[0]
					lead, ok := ste.Leading(first)
					fmt.Printf("  first=%q words=%d leading=(%q,%v)\n", first, ste.WordCount(first), lead, ok)
				}
				break
			}
		}
	}
}

func rep(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
