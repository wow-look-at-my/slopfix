// userscriptOpen and userscriptClose bound a userscript metadata block.

package treecomments

import "strings"

// userscriptOpen and userscriptClose bound a userscript metadata block.
const (
	userscriptOpen  = "==UserScript=="
	userscriptClose = "==/UserScript=="
)

// withoutUserscriptMetadata drops every comment from a line that opens a
// userscript metadata block through the line that closes it. A block that
// never closes is not metadata, and every comment of it stays.
func withoutUserscriptMetadata(comments []Comment) []Comment {
	kept := comments[:0:0]
	for i := 0; i < len(comments); i++ {
		if metadataMarker(comments[i].Text) != userscriptOpen {
			kept = append(kept, comments[i])
			continue
		}
		end := i + 1
		for end < len(comments) && metadataMarker(comments[end].Text) != userscriptClose {
			end++
		}
		if end == len(comments) {
			kept = append(kept, comments[i])
			continue
		}
		i = end
	}
	return kept
}

// metadataMarker answers the text of a line comment with its marker and blanks taken off.
func metadataMarker(text string) string {
	rest, _ := strings.CutPrefix(strings.TrimSpace(text), "//")
	return strings.TrimSpace(rest)
}
