// renameEdits answers an edit for each dead name that a defined identifier replaces.

package tombstones

import (
	"context"
	"os/exec"
	"slices"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/ste"
)

// renameEdits answers an edit for each dead name that a defined identifier
// replaces. A rename keeps the sentence, so it runs before any cut.
func renameEdits(path, text string, blocks []Block, dead []string) []edit.Edit {
	if IsDocument(path) || len(dead) == 0 {
		return nil
	}
	lines := strings.Split(text, "\n")
	starts := lineStarts(lines)
	gone := set.New[string]()
	gone.AddRange(dead...)
	var edits []edit.Edit
	for _, name := range dead {
		lineNo := HitForName(blocks, name).LineNo - 1
		if lineNo < 0 || lineNo >= len(lines) {
			continue
		}
		with := replacementFor(path, text, lines, blocks, name, gone)
		if with == "" {
			continue
		}
		at := wordIndex(lines[lineNo], name)
		if at < 0 {
			continue
		}
		start := starts[lineNo] + at
		edits = append(edits, edit.Edit{Start: start, End: start + len(name), Text: with, Cut: []string{name}})
	}
	return edits
}

// lineStarts answers the byte offset where each line of lines starts.
func lineStarts(lines []string) []int {
	starts := make([]int, len(lines))
	for i, at := 1, 0; i < len(lines); i++ {
		at += len(lines[i-1]) + 1
		starts[i] = at
	}
	return starts
}

// wordIndex answers where name stands in line as a whole identifier, or a
// negative offset when it does not.
func wordIndex(line, name string) int {
	for from := 0; from < len(line); {
		i := strings.Index(line[from:], name)
		if i < 0 {
			return -1
		}
		at := from + i
		end := at + len(name)
		if (at == 0 || !identByte(line[at-1])) && (end == len(line) || !identByte(line[end])) {
			return at
		}
		from = at + 1
	}
	return -1
}

// identByte reports a byte an identifier can hold.
func identByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// replacementFor answers the defined identifier a dead name stands for, or
// "". The first choice is the identifier that differs from it in case alone.
// The second is the name the comment documents, when the dead name opens the
// comment the way a doc comment opens with its subject.
func replacementFor(path, text string, lines []string, blocks []Block, name string, dead set.Set[string]) string {
	if with := caseVariant(path, text, name, dead); with != "" {
		return with
	}
	for _, b := range blocks {
		if !strings.Contains(b.Prose, name) || len(b.LineNos) == 0 {
			continue
		}
		words := identifierWords(proseOf(firstLine(b.Prose)))
		if len(words) == 0 || words[0] != name {
			return ""
		}
		next := b.LineNos[len(b.LineNos)-1] + 1
		if next >= len(lines) {
			return ""
		}
		with := declaredName(lines[next])
		if with == "" || with == name || dead.Contains(with) {
			return ""
		}
		return with
	}
	return ""
}

// caseVariant answers the identifier of the file or the repository that equals
// name when case is ignored, or "" when none or several do. A word in lower
// case alone is no spelling of a name that opens on a capital. That word is
// plain prose or a value, and the name in the comment is a proper noun.
func caseVariant(path, text, name string, dead set.Set[string]) string {
	found := set.New[string]()
	capital := name[0] >= 'A' && name[0] <= 'Z'
	collect := func(word string) {
		if capital && word == strings.ToLower(word) {
			return
		}
		if word != name && strings.EqualFold(word, name) && !dead.Contains(word) {
			found.Add(word)
		}
	}
	for _, word := range identifierWords(text) {
		collect(word)
	}
	if found.Len() == 0 {
		for _, word := range repoCaseVariants(path, name) {
			collect(word)
		}
	}
	if found.Len() != 1 {
		return ""
	}
	return slices.Collect(found.All())[0]
}

// repoCaseVariants answers each spelling of name, in any case, that a file of
// the repository holds. It answers nothing when it cannot search.
func repoCaseVariants(path, name string) []string {
	root := RepoRoot(path)
	if root == "" {
		return nil
	}
	if v, ok := indexes.Load(root); ok {
		if ix := v.(*symbolIndex); ix.ok {
			var out []string
			for word := range ix.files {
				if strings.EqualFold(word, name) {
					out = append(out, word)
				}
			}
			return out
		}
	}
	rg, err := exec.LookPath("rg")
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, rg, "--no-messages", "--ignore-case", "--fixed-strings", "--word-regexp",
		"--only-matching", "--no-filename", "--no-line-number", "-e", name, root).Output()
	if err != nil || ctx.Err() != nil {
		return nil
	}
	return strings.Fields(string(out))
}

// declKeywords open a declaration before the name it declares.
var declKeywords = set.Of("func", "type", "var", "const", "fn", "pub", "def", "class", "struct", "enum", "interface",
	"static", "let", "export", "function", "async", "public", "private", "protected", "final", "mut")

// declaredName answers the identifier a code line declares, or "". A Go
// method's receiver is skipped. The name must be built like a symbol, the
// same test a dead name passes.
func declaredName(line string) string {
	rest := strings.TrimSpace(line)
	for {
		if strings.HasPrefix(rest, "(") {
			end := strings.IndexByte(rest, ')')
			if end < 0 {
				return ""
			}
			rest = strings.TrimSpace(rest[end+1:])
			continue
		}
		words := identifierWords(rest)
		if len(words) == 0 || !strings.HasPrefix(rest, words[0]) {
			return ""
		}
		if !declKeywords.Contains(words[0]) {
			if isCandidate(words[0]) {
				return words[0]
			}
			return ""
		}
		rest = strings.TrimSpace(rest[len(words[0]):])
	}
}

// proseOf answers a comment line without its marker or a block closer.
func proseOf(line string) string {
	t := strings.TrimSpace(line)
	for _, opener := range []string{"///", "//!", "//", "/**", "/*", "#", "--", "*"} {
		if rest, found := strings.CutPrefix(t, opener); found {
			t = rest
			break
		}
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(t), "*/"))
}

// endsSentence reports prose that stops on a sentence end.
func endsSentence(prose string) bool {
	prose = strings.TrimRight(prose, `)"'`+"`")
	return strings.HasSuffix(prose, ".") || strings.HasSuffix(prose, "!") || strings.HasSuffix(prose, "?")
}

// wholeSentencesOnLine reports whether line li of a block's prose holds only
// sentences that name name, each opened and closed on that line. A strip of
// such a line takes no other sentence with it.
func wholeSentencesOnLine(proseLines []string, li int, name string) bool {
	prose := proseOf(proseLines[li])
	if prose == "" {
		return false
	}
	// A line with no sentence end still closes its sentence when no prose follows it.
	if !endsSentence(prose) && li+1 < len(proseLines) && proseOf(proseLines[li+1]) != "" {
		return false
	}
	for i := li - 1; i >= 0; i-- {
		prev := proseOf(proseLines[i])
		if prev == "" {
			break
		}
		if !endsSentence(prev) {
			return false
		}
		break
	}
	for _, s := range ste.Sentences(prose) {
		if wordIndex(s, name) < 0 {
			return false
		}
	}
	return true
}
