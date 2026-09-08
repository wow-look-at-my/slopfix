// lex.go is the shared engine. It walks a file in a single pass and says, for
// every byte, whether it is code, a comment or a literal.
//
// Every language shares this walk. What differs is the table it reads, so a
// language is a row of data rather than a separate implementation. That is what
// keeps a rule's behaviour the same across a tree that holds several languages.
package source

import "strings"

// Kind is what a span holds.
type Kind int

const (
	// KindCode is everything that is neither a comment nor a literal.
	KindCode Kind = iota
	// KindComment is comment text, markers included.
	KindComment
	// KindString is a literal, delimiters included.
	KindString
)

// Span is a run of bytes of a single kind. End is exclusive.
type Span struct {
	Kind  Kind
	Start int
	End   int
}

// Lex returns the spans of a file, in order and covering every byte. It reports
// false for a file whose language the table does not carry.
func Lex(filename, src string) ([]Span, bool) {
	s, ok := syntaxFor(filename)
	if !ok {
		return nil, false
	}
	return lexWith(s, src), true
}

// lexWith is the walk itself, over a syntax the caller already resolved.
func lexWith(s syntax, src string) []Span {
	var out []Span
	code := 0 // where the current run of code began

	flush := func(at int) {
		if at > code {
			out = append(out, Span{Kind: KindCode, Start: code, End: at})
		}
	}

	for i := 0; i < len(src); {
		if n, ok := commentAt(s, src, i); ok {
			flush(i)
			out = append(out, Span{Kind: KindComment, Start: i, End: i + n})
			i += n
			code = i
			continue
		}
		if n, ok := literalAt(s, src, i); ok {
			flush(i)
			out = append(out, Span{Kind: KindString, Start: i, End: i + n})
			i += n
			code = i
			continue
		}
		i++
	}
	flush(len(src))
	return out
}

// commentAt reports the length of the comment opening at i, if a comment opens there.
func commentAt(s syntax, src string, i int) (int, bool) {
	rest := src[i:]
	for _, marker := range s.line {
		if !strings.HasPrefix(rest, marker) {
			continue
		}
		if s.wordStartComment && !atWordStart(src, i) {
			continue
		}
		end := strings.IndexByte(rest, '\n')
		if end < 0 {
			end = len(rest)
		}
		return end, true
	}
	for _, b := range s.block {
		if !strings.HasPrefix(rest, b.open) {
			continue
		}
		return blockLength(rest, b), true
	}
	return 0, false
}

// blockLength measures a block comment, counting an inner open when the
// language nests them. An unterminated block runs to the end of the file, which
// is what the compiler sees too.
func blockLength(rest string, b blockSpec) int {
	depth := 1
	at := len(b.open)
	for at < len(rest) {
		if b.nested && strings.HasPrefix(rest[at:], b.open) {
			depth++
			at += len(b.open)
			continue
		}
		if strings.HasPrefix(rest[at:], b.close) {
			depth--
			at += len(b.close)
			if depth == 0 {
				return at
			}
			continue
		}
		at++
	}
	return len(rest)
}

// atWordStart reports whether the byte before i can precede a comment marker.
// A shell passes `a#b` through whole, so the marker needs a word start.
func atWordStart(src string, i int) bool {
	if i == 0 {
		return true
	}
	switch src[i-1] {
	case ' ', '\t', '\n', '\r', ';', '|', '&', '(', ')', '{', '}':
		return true
	}
	return false
}

// literalAt reports the length of the literal opening at i, if a literal opens there.
func literalAt(s syntax, src string, i int) (int, bool) {
	rest := src[i:]
	if s.heredoc {
		if n, ok := heredocLength(src, i); ok {
			return n, true
		}
	}
	if s.rustRaw {
		if n, ok := rustRawLength(rest); ok {
			return n, true
		}
	}
	for _, spec := range s.str {
		if !strings.HasPrefix(rest, spec.open) {
			continue
		}
		if spec.charLike && !closesNearby(rest, spec) {
			continue
		}
		return literalLength(rest, spec), true
	}
	return 0, false
}

// closesNearby reports a quote that really opens a character literal. Rust
// spells a lifetime with the same byte, and a lifetime has no closing quote.
func closesNearby(rest string, spec stringSpec) bool {
	limit := min(len(rest), 6)
	at := len(spec.open)
	for at < limit {
		if rest[at] == '\\' {
			at += 2
			continue
		}
		if strings.HasPrefix(rest[at:], spec.close) {
			return true
		}
		at++
	}
	return false
}

// literalLength measures a literal. An unterminated literal consumes the rest,
// which stops a stray quote from turning the remaining file into prose.
func literalLength(rest string, spec stringSpec) int {
	at := len(spec.open)
	for at < len(rest) {
		if spec.escape && rest[at] == '\\' {
			at += 2
			continue
		}
		if !spec.multiline && rest[at] == '\n' {
			return at
		}
		if strings.HasPrefix(rest[at:], spec.close) {
			return at + len(spec.close)
		}
		at++
	}
	return len(rest)
}

// rustRawLength measures `r"..."` and `r#"..."#`, whose close carries as many
// hashes as the open did. A byte-string prefix opens the same form.
func rustRawLength(rest string) (int, bool) {
	at := 0
	if strings.HasPrefix(rest, "br") {
		at = 2
	} else if strings.HasPrefix(rest, "r") {
		at = 1
	} else {
		return 0, false
	}
	hashes := 0
	for at+hashes < len(rest) && rest[at+hashes] == '#' {
		hashes++
	}
	at += hashes
	if at >= len(rest) || rest[at] != '"' {
		return 0, false
	}
	at++
	closer := `"` + strings.Repeat("#", hashes)
	if end := strings.Index(rest[at:], closer); end >= 0 {
		return at + end + len(closer), true
	}
	return len(rest), true
}

// heredocLength measures a shell heredoc: `<<WORD` runs to a line holding
// WORD alone. The body is a literal, so a marker inside it is data.
func heredocLength(src string, i int) (int, bool) {
	rest := src[i:]
	if !strings.HasPrefix(rest, "<<") {
		return 0, false
	}
	at := 2
	if at < len(rest) && (rest[at] == '-' || rest[at] == '~') {
		at++
	}
	for at < len(rest) && (rest[at] == ' ' || rest[at] == '\t') {
		at++
	}
	quote := byte(0)
	if at < len(rest) && (rest[at] == '"' || rest[at] == '\'') {
		quote = rest[at]
		at++
	}
	start := at
	for at < len(rest) && isWordByte(rest[at]) {
		at++
	}
	word := rest[start:at]
	if word == "" {
		return 0, false
	}
	if quote != 0 {
		if at >= len(rest) || rest[at] != quote {
			return 0, false
		}
		at++
	}
	nl := strings.IndexByte(rest[at:], '\n')
	if nl < 0 {
		return len(rest), true
	}
	body := at + nl + 1
	for line := body; line <= len(rest); {
		end := strings.IndexByte(rest[line:], '\n')
		stop := len(rest)
		if end >= 0 {
			stop = line + end
		}
		if strings.TrimSpace(rest[line:stop]) == word {
			return stop, true
		}
		if end < 0 {
			break
		}
		line = stop + 1
	}
	return len(rest), true
}

// isWordByte reports a byte that can spell a heredoc's delimiter word.
func isWordByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
