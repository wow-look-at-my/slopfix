// blocks.go decides which files this rule reads, and which it skips.
//
// The measuring is in treeblocks.go, off a real syntax tree. What is left here
// is what comes before a parse: does a grammar cover this file, and did a
// generator write it.
package commentfix

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
	return parsed
}
