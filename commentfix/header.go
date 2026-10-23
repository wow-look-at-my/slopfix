// header.go holds comments/header: a source file that opens on code although
// one of its comments can say what the file is.
package commentfix

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/pins"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/trace"
	"github.com/wow-look-at-my/slopfix/treecomments"
)

// IDHeader names the rule.
const IDHeader = "comments/header"

// FixerHeader is the fixer that writes the header.
const FixerHeader = "comments/header"

func init() {
	fixer.Register(fixer.Spec{
		Label:    FixerHeader,
		Families: []string{"comments"},
		Rules:    []string{IDHeader},
		Files:    []fixer.Kind{fixer.Source},
		Place:    50,
		Repair: func(f *fixer.File) {
			if e, ok := headerEdit(f.Path, f.Text()); ok {
				f.ApplyThrough(func(text string, edits []edit.Edit, scope edit.Scope) edit.Result {
					return treecomments.ApplyInsert(f.Path, text, edits, scope)
				}, []edit.Edit{e})
			}
		},
	})
}

// HeaderHit is a file that opens with no header comment.
type HeaderHit struct {
	// Line is the line the header goes above, counted from one.
	Line int
	// Sentence is what the repair writes as the header.
	Sentence string
}

// CheckHeader reports a file that opens with no header comment. This
// happens when one of its comments has a first sentence the repair can copy
// to the top.
func CheckHeader(filename, src string) []HeaderHit {
	defer trace.Phase("rule/comments-header")()
	at, sentence, ok := missingHeader(filename, src)
	if !ok {
		return nil
	}
	return []HeaderHit{{Line: strings.Count(src[:at], "\n") + 1, Sentence: sentence}}
}

// WriteHeader answers src with the header that CheckHeader reports written
// at the top. It answers src unchanged when CheckHeader reports nothing.
func WriteHeader(filename, src string) string {
	f := fixer.Open(filename, src, fixer.Options{Kind: fixer.Source})
	fixer.Run(f, fixer.Named(FixerHeader))
	return f.Text()
}

// headerEdit answers the insertion that writes the header.
func headerEdit(filename, src string) (edit.Edit, bool) {
	at, sentence, ok := missingHeader(filename, src)
	if !ok {
		return edit.Edit{}, false
	}
	// The blank line keeps the header off the code. In Go a comment right above package is the package doc.
	return edit.Edit{Start: at, End: at, Text: headerMarker(filename, src) + sentence + "\n\n"}, true
}

// missingHeader answers where the header goes and the sentence it holds. It
// reports false for a file that has a header, or no comment to copy one from.
func missingHeader(filename, src string) (int, string, bool) {
	if !treecomments.Supported(filename) || pins.IsTest(filename) || IsGenerated(filename, src) {
		return 0, "", false
	}
	at, found := headerSlot(filename, src)
	if found {
		return 0, "", false
	}
	sentence, ok := headerSentence(filename, src)
	return at, sentence, ok
}

// headerSlot answers the start of the line where the file's first code sits.
// It skips blank lines, the interpreter line, directives and a license block.
// It reports true when a comment comes first, which is a header.
func headerSlot(filename, src string) (int, bool) {
	starts := map[int]treecomments.Run{}
	// A run read from any of its comments, since the interpreter line can open one.
	for _, run := range treecomments.Runs(filename, src) {
		for i, c := range run {
			starts[c.Offset] = run[i:]
		}
	}
	pos := 0
	if strings.HasPrefix(src, "#!") {
		pos = lineAfter(src, 0)
	}
	for {
		next := pos + len(src[pos:]) - len(strings.TrimLeft(src[pos:], " \t\r\n"))
		if next >= len(src) {
			return lineStart(src, len(src)), false
		}
		run, ok := starts[next]
		if !ok {
			return lineStart(src, next), false
		}
		if !skipped(run) {
			return 0, true
		}
		last := run[len(run)-1]
		pos = last.Offset + len(last.Text)
	}
}

// skipped reports a comment run that is no header: a license block, or tool lines alone.
func skipped(run treecomments.Run) bool {
	var lines []string
	for _, c := range run {
		lines = append(lines, strings.Split(c.Text, "\n")...)
	}
	if isLicenseNotice(lines) {
		return true
	}
	for i, line := range lines {
		if !isDirectiveLine(line) && !treecomments.DirectiveAt(lines, i) {
			return false
		}
	}
	return true
}

// headerSentence answers the first sentence of the first comment that can
// stand as a one-line header.
func headerSentence(filename, src string) (string, bool) {
	runs := treecomments.Runs(filename, src)
	lines := strings.Split(src, "\n")
	for _, p := range paragraphsOf(lines, runs) {
		if !judged(lines, p) {
			continue
		}
		sentences := ste.Sentences(p.prose)
		if len(sentences) == 0 {
			continue
		}
		first := strings.TrimSpace(sentences[0])
		if usableHeader(first) {
			return first, true
		}
	}
	return "", false
}

// usableHeader reports a sentence that closes and fits the STE cap.
func usableHeader(sentence string) bool {
	if sentence == "" || isHeading(sentence) || strings.Contains(sentence, "*/") {
		return false
	}
	if !strings.ContainsAny(sentence[len(sentence)-1:], ".!?") {
		return false
	}
	return ste.WordCount(sentence) <= ste.SentenceWordCap
}

// headerMarker answers the line comment marker the file's own comments use.
func headerMarker(filename, src string) string {
	for _, c := range treecomments.Extract(filename, src) {
		if strings.HasPrefix(c.Text, "#") {
			return "# "
		}
		return "// "
	}
	return "// "
}

// lineStart answers the offset that opens the line holding at.
func lineStart(src string, at int) int {
	return strings.LastIndexByte(src[:at], '\n') + 1
}

// lineAfter answers the offset that opens the line after the one holding at.
func lineAfter(src string, at int) int {
	if i := strings.IndexByte(src[at:], '\n'); i >= 0 {
		return at + i + 1
	}
	return len(src)
}
