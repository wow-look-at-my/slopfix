// blocks.go decides which files this rule reads, and which it skips.
//
// The measuring is in treeblocks.go, off a real syntax tree. What is left here
// is what comes before a parse: does a grammar cover this file, and did a
// generator write it.
package commentfix

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/treecomments"
)

// blocks returns every comment block in the file, each with the code it
// documents measured beside it.
func blocks(filename, src string) []block {
	language := languageFor(filename)
	if language == nil {
		return nil
	}
	if IsGenerated(filename, src) {
		return nil
	}
	parsed, ok := treeBlocks(language, src)
	if !ok {
		return nil
	}
	if strings.HasSuffix(filename, "_test.go") {
		parsed = withoutExampleOutput(parsed)
	}
	return withoutLicenseNotices(parsed)
}

// withoutExampleOutput ends each block at the line that opens an example's
// output. go test compares that text to what the example prints, so it is data.
func withoutExampleOutput(parsed []block) []block {
	kept := parsed[:0]
	for _, b := range parsed {
		for i, line := range b.text {
			if treecomments.IsExampleOutput(line) {
				b.text, b.end = b.text[:i], b.start+i
				break
			}
		}
		if len(b.text) > 0 {
			kept = append(kept, b)
		}
	}
	return kept
}
