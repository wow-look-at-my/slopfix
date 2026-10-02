package bashclean

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
)

// jsonKind says how a file holds JSON. A stream of several top-level values is JSON Lines.
type jsonKind int

const (
	notJSON jsonKind = iota
	jsonDoc
	jsonLines
)

// The sniff reads a fixed head and a fixed tail, so its cost does not grow with
// the file size.
const (
	sniffHead = 64 << 10
	sniffTail = 4 << 10
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// sniffJSON decides from content alone, never from the extension. It opens only
// a regular file, because an open on a FIFO blocks.
func sniffJSON(path string) jsonKind {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() || st.Size() == 0 {
		return notJSON
	}
	f, err := os.Open(path)
	if err != nil {
		return notJSON
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, min(st.Size(), sniffHead))
	if _, err := io.ReadFull(f, head); err != nil {
		return notJSON
	}
	whole := int64(len(head)) == st.Size()
	if !whole && !closesJSON(f, st.Size()) {
		return notJSON
	}
	return classify(bytes.TrimPrefix(head, utf8BOM), whole)
}

// closesJSON asks whether the last byte that is not white space closes an
// object or an array.
func closesJSON(f *os.File, size int64) bool {
	n := min(size, sniffTail)
	tail := make([]byte, n)
	if _, err := f.ReadAt(tail, size-n); err != nil {
		return false
	}
	tail = bytes.TrimRight(tail, " \t\r\n")
	return len(tail) > 0 && (tail[len(tail)-1] == '}' || tail[len(tail)-1] == ']')
}

// classify tokenizes the head. A cut head may end inside a value, so an
// unexpected end is acceptable there. A syntax error never is.
func classify(head []byte, whole bool) jsonKind {
	trimmed := bytes.TrimLeft(head, " \t\r\n")
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return notJSON
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	depth, values := 0, 0
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			if depth != 0 && whole {
				return notJSON
			}
			break
		}
		if err != nil {
			var syn *json.SyntaxError
			if whole || errors.As(err, &syn) {
				return notJSON
			}
			break
		}
		if d, ok := tok.(json.Delim); ok {
			if d == '{' || d == '[' {
				depth++
			} else {
				depth--
			}
		}
		if depth == 0 {
			if _, ok := tok.(json.Delim); !ok {
				return notJSON
			}
			values++
		}
	}
	if values > 1 {
		return jsonLines
	}
	return jsonDoc
}
