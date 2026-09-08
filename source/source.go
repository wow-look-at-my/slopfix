// Package source is a substrate adapter. It answers where the prose is in a
// source file, by extracting the comments and nothing else.
//
// The syntax table in lang.go spans every language this org writes, and lex.go
// is the shared walk that reads every row of it. Nothing here is specific to any
// language, and a rule that reads a comment reads it the same way whatever
// wrote the file.
package source

// Comment is a comment's text and where it begins in the source.
type Comment struct {
	Text   string
	Offset int
}

// Extract returns every comment in the source, in order.
func Extract(filename, src string) []Comment {
	spans, ok := Lex(filename, src)
	if !ok {
		return nil
	}
	var out []Comment
	for _, s := range spans {
		if s.Kind == KindComment {
			out = append(out, Comment{Text: src[s.Start:s.End], Offset: s.Start})
		}
	}
	return out
}
