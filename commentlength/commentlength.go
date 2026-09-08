// Package commentlength finds a comment longer than the code it documents.
//
// A comment earns its place by stopping the next mistake. One that runs longer
// essay, and the reader pays for it on every pass through the file. The rule is
// a proxy rather than a judgement of content: length is what a machine can
// measure, and past a point every long comment is doing the same thing.
//
// It reads the source package's adapter, so it spans the C family and the hash
// family together -- Go, C, C++, Rust, Java, JavaScript, TypeScript, Swift,
// Kotlin, Zig, Python, Ruby, Bash, YAML and the rest of that table. Nothing
// here is specific to a language.
//
// The repair is to cut, from the end. A comment says its point first and
// elaborates afterwards, so the trailing paragraph is what a reader loses least
// by losing. The opening sentence is never cut: a block trimmed to nothing is a
// worse edit than a block left long.
package commentlength

import (
	"strings"
)

// ID names this rule, on a report and on the command line alike.
const ID = "comments/length"

// floorChars is the size a comment may always be, whatever it documents.
// Without it a single-line declaration makes every useful sentence a finding,
// which is how a rule earns the reputation that gets it turned off.
const floorChars = 120

// Hit is a comment block that outweighs its code.
type Hit struct {
	// ID names the rule, the way a compiler names a warning.
	ID string `json:"id"`
	// Tell says in words which measure was exceeded.
	Tell string `json:"tell"`
	// Sentence quotes the comment's opening, so a report is recognisable.
	Sentence string `json:"sentence"`
	// Line is the block's first line, counting from the top.
	Line int `json:"line"`
	// Repairable reports whether Fix can bring this block inside the budget
	// without deleting its opening sentence.
	Repairable bool `json:"repairable"`
}

// block is a run of comment lines and the code beneath it.
type block struct {
	// start and end are line indexes into the file, half open.
	start, end int
	// codeLines and codeChars measure what the block documents.
	codeLines, codeChars int
	// text is the comment's lines, marker and all.
	text []string
	// exact is true when a parser decided this span rather than a line walk.
	//
	// It gates the REPAIR and nothing else. A finding is worth reporting on a
	// span that was worked out by reading lines, because a reader checks it
	// before acting. Deleting prose on that basis is not: a span wrong by a
	// line deletes the wrong sentence, and nobody reviews a rewrite a hook
	// applied.
	exact bool
}

// Check reports every comment block in src that outweighs its code.
func Check(filename, src string) []Hit {
	var hits []Hit
	for _, b := range blocks(filename, src) {
		tell, over := judge(b)
		if !over {
			continue
		}
		hits = append(hits, Hit{
			ID:         ID,
			Tell:       tell,
			Sentence:   opening(b.text),
			Line:       b.start + 1,
			Repairable: len(trim(b)) < len(b.text),
		})
	}
	return hits
}

// Fix returns src with every over-long comment block cut back inside its
// budget, and whether anything changed.
//
// Cutting is from the end, a paragraph at a time and then a line at a time,
// and it stops before the opening sentence. A block that cannot be brought
// inside the budget that way is left alone and still reported: a repair that
// deletes the only sentence worth keeping is worse than the finding.
func Fix(filename, src string) (string, bool) {
	bs := blocks(filename, src)
	if len(bs) == 0 {
		return src, false
	}
	lines := splitLines(src)
	changed := false

	// Back to front, so an earlier block's line numbers stay valid.
	for i := len(bs) - 1; i >= 0; i-- {
		b := bs[i]
		if _, over := judge(b); !over {
			continue
		}
		kept := trim(b)
		if len(kept) >= len(b.text) {
			continue
		}
		lines = append(lines[:b.start], append(kept, lines[b.end:]...)...)
		changed = true
	}
	if !changed {
		return src, false
	}
	return strings.Join(lines, "\n"), true
}

// judge measures a block against its code and names which measure it failed.
func judge(b block) (string, bool) {
	if b.codeLines == 0 {
		return "", false
	}
	if len(b.text) > b.codeLines {
		return "the comment runs more lines than the code it documents", true
	}
	chars := charsOf(b.text)
	if chars > floorChars && chars > b.codeChars {
		return "the comment runs longer than the code it documents", true
	}
	return "", false
}

// trim cuts the block's trailing prose until it fits, keeping the opening.
//
// A paragraph goes before a line does, because a comment's paragraphs are its
// units of thought and half a paragraph reads as a truncation rather than as a
// shorter comment. The opening paragraph always survives.
func trim(b block) []string {
	kept := b.text
	for len(kept) > 1 {
		if _, over := judge(block{text: kept, codeLines: b.codeLines, codeChars: b.codeChars}); !over {
			return kept
		}
		if next, ok := dropParagraph(kept); ok {
			kept = next
			continue
		}
		kept = kept[:len(kept)-1]
	}
	return kept
}

// dropParagraph removes the last blank-separated paragraph of a comment block,
// and reports false when the block holds only one.
func dropParagraph(text []string) ([]string, bool) {
	for i := len(text) - 1; i > 0; i-- {
		if isBlankComment(text[i]) {
			return text[:i], true
		}
	}
	return text, false
}

// isBlankComment reports a comment line carrying no prose, which is how a
// comment block spells a paragraph break.
func isBlankComment(line string) bool {
	t := strings.TrimSpace(line)
	for _, marker := range []string{"//", "#", "*"} {
		if t == marker {
			return true
		}
		if rest, found := strings.CutPrefix(t, marker); found && strings.TrimSpace(rest) == "" {
			return true
		}
	}
	return t == ""
}

// opening is the block's first line of prose, bounded so a report quotes a
// recognisable fragment rather than a paragraph.
func opening(text []string) string {
	for _, line := range text {
		t := strings.TrimSpace(line)
		if isBlankComment(line) {
			continue
		}
		if len(t) > 90 {
			t = t[:87] + "..."
		}
		return t
	}
	return ""
}

// charsOf is the prose weight of a comment block: its lines, markers and all,
// with the indentation dropped, because indentation is the code's shape rather
// than the comment's size.
func charsOf(text []string) int {
	n := 0
	for _, line := range text {
		n += len(strings.TrimSpace(line))
	}
	return n
}

func splitLines(src string) []string { return strings.Split(src, "\n") }
