package noworkloss

import (
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// sed and awk are the filters that can write a file without being told to on
// the argv. Sed's `w` command and awk's `print >` both name their target inside
// the program text. Reading the program is what separates a filter (a bare
// comparison writes nothing) from a writer, so neither is denied for its name
// alone.

func awkWrites(seg segment, rest []word) []write {
	valueFlags := set.Of[string]("-v", "--assign", "-f", "--file", "-F", "--field-separator", "-i", "--include")
	flags, operands := scanArgs(rest, valueFlags)

	// gawk spells in-place editing as loading its inplace library, `-i inplace`,
	inPlace := false
	for _, f := range []string{"-i", "--include"} {
		if v, ok := flags[f]; ok && v.text == "inplace" {
			inPlace = true
		}
	}
	for _, a := range rest {
		if a.text == "--in-place" {
			inPlace = true
		}
	}

	var program word
	files := operands
	progFromFile := false
	for _, f := range []string{"-f", "--file"} {
		if _, ok := flags[f]; ok {
			progFromFile = true
		}
	}
	if !progFromFile && len(operands) > 0 {
		program, files = operands[0], operands[1:]
	}

	var out []write
	switch {
	case inPlace && len(files) > 0:
		out = append(out, write{route: "awk -i inplace", paths: files, dir: seg.cwd})
	case inPlace:
		out = append(out, write{route: "awk -i inplace", opaque: "an in-place awk whose files are supplied at runtime rather than named in the command"})
	}
	if progFromFile || program.text == "" {
		return out
	}
	if !program.static {
		return append(out, write{route: "awk", opaque: "an awk program assembled from an expansion, whose redirects cannot be resolved"})
	}
	targets, unresolvable := awkRedirectTargets(program.text)
	if unresolvable {
		out = append(out, write{route: "awk", opaque: "an awk program whose print redirect names a file this hook cannot resolve"})
	}
	for _, t := range targets {
		out = append(out, write{route: "awk print >", paths: []word{{text: t, static: true}}, dir: seg.cwd})
	}
	return out
}

// awkRedirectTargets finds the files an awk program writes. In awk's grammar an
// unparenthesised `>` after print or printf is a redirect.
func awkRedirectTargets(prog string) (targets []string, unresolvable bool) {
	printSeen := false
	inString := false
	for i := 0; i < len(prog); i++ {
		c := prog[i]
		if inString {
			if c == '\\' {
				i++
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch {
		case c == '"':
			inString = true
		case c == ';' || c == '}' || c == '{' || c == '\n':
			printSeen = false
		case c == '>' && printSeen:
			if i+1 < len(prog) && prog[i+1] == '>' {
				i++
			}
			name, ok := awkLiteralAfter(prog[i+1:])
			if !ok {
				unresolvable = true
				continue
			}
			if !isDeviceFile(name) {
				targets = append(targets, name)
			}
		case isWordStart(c):
			word, next := readWord(prog, i)
			i = next - 1
			if word == "print" || word == "printf" {
				printSeen = true
			}
		}
	}
	return targets, unresolvable
}

func awkLiteralAfter(s string) (string, bool) {
	s = strings.TrimLeft(s, " \t")
	if !strings.HasPrefix(s, `"`) {
		return "", false
	}
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			b.WriteByte(s[i])
			continue
		}
		if s[i] == '"' {
			return b.String(), true
		}
		b.WriteByte(s[i])
	}
	return "", false
}

func isWordStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
}

func readWord(s string, i int) (string, int) {
	j := i
	for j < len(s) && (isWordStart(s[j]) || s[j] >= '0' && s[j] <= '9') {
		j++
	}
	return s[i:j], j
}
