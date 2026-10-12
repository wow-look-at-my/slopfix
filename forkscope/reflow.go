package forkscope

import (
	"strings"
	"unicode"

	"github.com/pmezard/go-difflib/difflib"
)

// word is one blank-separated word of a hunk, with the line it stands on.
type word struct {
	text string
	line int
}

// ownedRewrap answers a hunk whose rewrap moved words across a line the fork
// did not write. Each owned line takes the new words that align with its own,
// and every other line keeps its bytes. A word the rewrap moved out of an
// upstream line stays there, so the owned line never repeats it.
func ownedRewrap(old, now []string, owned func(int) bool) string {
	var before []word
	for k, line := range old {
		_, rest := linePrefix(line)
		for _, w := range strings.Fields(rest) {
			before = append(before, word{w, k})
		}
	}
	var after []string
	for _, line := range now {
		_, rest := linePrefix(line)
		after = append(after, strings.Fields(rest)...)
	}
	texts := make([]string, len(before))
	for n, w := range before {
		texts[n] = w.text
	}
	ops := difflib.NewMatcherWithJunk(texts, after, false, nil).GetOpCodes()

	// A refused change leaves its words where they stood.
	stays, back := map[string]int{}, map[string]int{}
	for _, op := range ops {
		if op.Tag == 'e' || landsOnOwned(before, op, owned) >= 0 {
			continue
		}
		for _, w := range before[op.I1:op.I2] {
			if !owned(w.line) {
				stays[w.text]++
			}
		}
		for _, w := range after[op.J1:op.J2] {
			back[w]++
		}
	}

	words := make([][]string, len(old))
	for _, op := range ops {
		line := -1
		if op.Tag != 'e' {
			line = landsOnOwned(before, op, owned)
		}
		if line < 0 {
			for _, w := range before[op.I1:op.I2] {
				words[w.line] = append(words[w.line], w.text)
			}
			continue
		}
		for _, w := range before[op.I1:op.I2] {
			if back[w.text] > 0 {
				back[w.text]--
				words[w.line] = append(words[w.line], w.text)
			}
		}
		for _, w := range after[op.J1:op.J2] {
			if stays[w] > 0 {
				stays[w]--
				continue
			}
			words[line] = append(words[line], w)
		}
	}

	var out strings.Builder
	for k, line := range old {
		if !owned(k) {
			out.WriteString(line)
			continue
		}
		body := strings.TrimRight(line, "\r\n")
		prefix, _ := linePrefix(body)
		out.WriteString(prefix + strings.Join(words[k], " ") + line[len(body):])
	}
	return out.String()
}

// linePrefix splits a line into its indent and comment marker, with the blank
// after the marker, and the words after them. A marker is a first word that
// holds no letter and no digit, such as "#" or "//".
func linePrefix(line string) (string, string) {
	body := strings.TrimRight(line, "\r\n")
	rest := strings.TrimLeft(body, " \t")
	fields := strings.Fields(rest)
	if len(fields) == 0 || strings.IndexFunc(fields[0], func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
		return body[:len(body)-len(rest)], rest
	}
	after := strings.TrimLeft(rest[len(fields[0]):], " \t")
	return body[:len(body)-len(after)], after
}

// An insertion between lines lands on the owned line before it, else on the
// owned line after it.
func landsOnOwned(before []word, op difflib.OpCode, owned func(int) bool) int {
	if op.I1 == op.I2 {
		if op.I1 > 0 && owned(before[op.I1-1].line) {
			return before[op.I1-1].line
		}
		if op.I1 < len(before) && owned(before[op.I1].line) {
			return before[op.I1].line
		}
		return -1
	}
	for _, w := range before[op.I1:op.I2] {
		if !owned(w.line) {
			return -1
		}
	}
	return before[op.I1].line
}
