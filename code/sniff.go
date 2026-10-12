package code

import (
	"strings"

	ts "github.com/wow-look-at-my/go-tree-sitter"
	"github.com/wow-look-at-my/slopfix/grammars/clang"
	"github.com/wow-look-at-my/slopfix/grammars/cpp"
	"github.com/wow-look-at-my/slopfix/grammars/golang"
	"github.com/wow-look-at-my/slopfix/grammars/javascript"
	"github.com/wow-look-at-my/slopfix/grammars/rust"
	"github.com/wow-look-at-my/slopfix/grammars/tsx"
	"github.com/wow-look-at-my/slopfix/grammars/typescript"
)

// sniffers are the grammars that read text with no file name to choose one.
// Bash is not among them: any run of words is a command to it, so English
// parses as a script.
var sniffers = []func() *ts.Language{
	typescript.Language,
	tsx.Language,
	javascript.Language,
	golang.Language,
	rust.Language,
	clang.Language,
	cpp.Language,
}

// codeMarks are the bytes every statement these grammars read needs at least one of.
const codeMarks = "(){};="

// Is reports text that some grammar parses whole, with no error. English
// prose never does: a couple of words side by side, or a closing period, is
// a syntax error in every grammar sniffers holds.
func Is(text string) bool {
	if strings.TrimSpace(text) == "" || !strings.ContainsAny(text, codeMarks) {
		return false
	}
	for _, load := range sniffers {
		if _, ok := ParseWith(load(), text); ok {
			return true
		}
	}
	return false
}
