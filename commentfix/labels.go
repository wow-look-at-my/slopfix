package commentfix

import (
	"regexp"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/counts"
)

// nameLabels rewrites each number of prose that names an item or sets a
// point, from the back, so an earlier offset stays valid.
func nameLabels(prose string, consts counts.Constants) string {
	tokens := cardinal.Find(prose, cardinal.Comment)
	for i := len(tokens) - 1; i >= 0; i-- {
		if e, ok := counts.LabelAt(prose, tokens[i].Offset, consts); ok {
			prose = prose[:e.Start] + e.Text + prose[e.End:]
		}
	}
	return prose
}

// constantDecl matches a declaration that binds a name to a number: "const NAME = 100", "const NAME: usize = 100;", "#define NAME 100", or a line of a Go const block.
var constantDecl = regexp.MustCompile(`(?m)(?:\b(?:const|static)[ \t]+(?:mut[ \t]+)?([A-Za-z_]\w*)(?:[ \t]*:[^=;\n]+|[ \t]+[A-Za-z_][\w.]*)?[ \t]*=[ \t]*(\d[\d_]*)|#define[ \t]+([A-Za-z_]\w*)[ \t]+(\d[\d_]*)|^[ \t]*([A-Z][A-Z0-9_]*)(?:[ \t]+[A-Za-z_][\w.]*)?[ \t]*=[ \t]*(\d[\d_]*))(?:u8|u16|u32|u64|usize|i32|i64|isize)?\b`)

// constantsIn answers the constants src declares, by value. A value that
// several names share names none of them, because the comment could mean any.
func constantsIn(src string) counts.Constants {
	names := map[string]string{}
	shared := set.New[string]()
	for _, m := range constantDecl.FindAllStringSubmatch(src, -1) {
		for g := 1; g+1 < len(m); g += 2 {
			if m[g] == "" {
				continue
			}
			value := strings.TrimLeft(strings.ReplaceAll(m[g+1], "_", ""), "0")
			if prev, ok := names[value]; ok && prev != m[g] {
				shared.Add(value)
			}
			names[value] = m[g]
		}
	}
	return func(n string) string {
		n = strings.TrimLeft(n, "0")
		if shared.Contains(n) {
			return ""
		}
		return names[n]
	}
}
