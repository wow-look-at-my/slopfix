package goembed

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"path"
	"regexp"
	"strings"

	yaml "go.yaml.in/yaml/v3"
	"mvdan.cc/sh/v3/syntax"
)

// Lang is a language a string can hold, and the extension of its file.
type Lang struct {
	Name string
	Ext  string
}

// minLines is the fewest lines of text a string holds before it reads as a file.
const minLines = 3

// interpreters maps the program a shebang names to the language it runs.
var interpreters = map[string]Lang{
	"sh":   {"shell", ".sh"},
	"bash": {"shell", ".sh"},
	"dash": {"shell", ".sh"},
	"ksh":  {"shell", ".sh"},
	"zsh":  {"zsh", ".zsh"},
	"node": {"JavaScript", ".js"},
	"deno": {"TypeScript", ".ts"},
	"perl": {"Perl", ".pl"},
	"ruby": {"Ruby", ".rb"},
	"lua":  {"Lua", ".lua"},
}

var (
	sqlStart        = regexp.MustCompile(`(?is)^(select\s.+\sfrom\s|insert\s+into\s|update\s+\S+\s+set\s|delete\s+from\s|create\s+(table|index|unique\s+index|view|trigger)\s|alter\s+table\s|drop\s+(table|index|view)\s|with\s+\w+\s+as\s*\()`)
	dockerDirective = regexp.MustCompile(`^#\s*(syntax|escape|check)=`)
	dockerFrom      = regexp.MustCompile(`^FROM\s+\S+`)
	dockerInstr     = regexp.MustCompile(`^(RUN|COPY|ADD|CMD|ENTRYPOINT|ENV|ARG|WORKDIR|USER|EXPOSE|LABEL|VOLUME)\s`)
	cInclude        = regexp.MustCompile(`^#include\s*[<"]`)
	goAsmText       = regexp.MustCompile(`^TEXT\s+\S*·\S*\(SB\)`)
	goFuncHead      = regexp.MustCompile(`^\s*func\s+(\([^)]*\)\s*)?\w+\s*[\[(]`)
	pyDef           = regexp.MustCompile(`^\s*(def|class)\s+\w+.*:\s*$`)
	pyImport        = regexp.MustCompile(`^(import\s+\w+|from\s+[\w.]+\s+import\s)`)
	jsLine          = regexp.MustCompile(`^\s*(function\s+\w*\s*\(|(const|let|var)\s+\w+\s*=|export\s+(default|function|const|class)\b|import\s.*\sfrom\s+['"]|import\s+['"])`)
	cFunc           = regexp.MustCompile(`^\s*[\w:<>*&]+(\s+[\w:<>*&]+)*\s+[*&]?[\w:~]+\s*\([^;{}]*\)\s*(const\s*)?\{\s*$`)
	rustFn          = regexp.MustCompile(`^\s*(pub(\([\w:]+\))?\s+)?(async\s+)?fn\s+\w+`)
	cssOpen         = regexp.MustCompile(`^\s*[^{}:;()]+\{\s*$`)
	cssDecl         = regexp.MustCompile(`^\s*[\w-]+\s*:\s*[^;{}]+;\s*$`)
	tomlTable       = regexp.MustCompile(`^\[\[?[\w.\-"]+\]\]?\s*$`)
	tomlPair        = regexp.MustCompile(`^[\w.\-"]+\s*=\s*\S`)
	yamlHead        = regexp.MustCompile(`^(-\s+)?[\w.\-/"']+\s*:(\s|$)`)
	// A heading stands alone before a blank line, which a run of comment lines does not.
	mdHeading = regexp.MustCompile(`(?m)^#{1,6}[ \t]+\S.*\n[ \t]*\n`)
	mdFence   = regexp.MustCompile("(?m)^\\s*(```|~~~)")
	htmlTag   = regexp.MustCompile(`(?i)^<(!doctype\s+html|html|head|body|div|span|p|a|ul|ol|li|table|tr|td|script|style|template|section|main|form|nav|header|footer|h[1-6]|svg)[\s>/]`)
	shellWord = regexp.MustCompile(`^\s*(echo|printf|cd|export|set|exit|exec|source|\.|mkdir|rm|cp|mv|cat|chmod|curl|wget|git|go|make|apt-get|apt|sudo|test|\[|\[\[|trap|shift|read|local|readonly|unset|tar|grep|sed|awk|ln|touch|docker|npm|pnpm)(\s|$)`)
)

// Classify answers the language a string holds when it reads as the text of a
// file in a language that is not Go.
func Classify(value string) (Lang, bool) {
	text := strings.TrimSpace(value)
	lines := nonBlank(text)
	if len(lines) < minLines {
		return Lang{}, false
	}
	if isGo(text) {
		return Lang{}, false
	}
	first := lines[0]
	if strings.HasPrefix(first, "#!") {
		return shebang(first), true
	}
	switch {
	case strings.HasPrefix(first, "<?xml"):
		return Lang{"XML", ".xml"}, true
	case strings.HasPrefix(text, "<") && strings.HasSuffix(text, ">") && strings.Contains(text, "</"):
		if strings.HasPrefix(strings.ToLower(first), "<svg") {
			return Lang{"SVG", ".svg"}, true
		}
		if htmlTag.MatchString(first) {
			return Lang{"HTML", ".html"}, true
		}
		return Lang{"XML", ".xml"}, true
	case (text[0] == '{' || text[0] == '[') && json.Valid([]byte(text)):
		return Lang{"JSON", ".json"}, true
	case sqlStart.MatchString(text):
		return Lang{"SQL", ".sql"}, true
	}
	if dockerfile(lines) {
		return Lang{"Dockerfile", ".dockerfile"}, true
	}
	counts := func(re *regexp.Regexp) int {
		n := 0
		for _, line := range lines {
			if re.MatchString(line) {
				n++
			}
		}
		return n
	}
	// Go assembly is a language of the Go toolchain. A .s file beside the package is assembled into it.
	if counts(goAsmText) > 0 {
		return Lang{}, false
	}
	switch {
	case counts(cInclude) > 0:
		return Lang{"C", ".c"}, true
	case counts(pyDef) > 0 && counts(pyImport)+counts(pyDef) >= 2:
		return Lang{"Python", ".py"}, true
	case counts(jsLine) >= 2:
		return Lang{"JavaScript", ".js"}, true
	case counts(rustFn) > 0:
		return Lang{"Rust", ".rs"}, true
	case counts(cFunc) > 0 && counts(goFuncHead) == 0:
		return Lang{"C++", ".cpp"}, true
	case counts(cssOpen) > 0 && counts(cssDecl) >= 2:
		return Lang{"CSS", ".css"}, true
	case counts(tomlTable) > 0 && counts(tomlPair) >= 2:
		return Lang{"TOML", ".toml"}, true
	case yamlDoc(text, first):
		return Lang{"YAML", ".yml"}, true
	case shell(text, lines):
		return Lang{"shell", ".sh"}, true
	case mdHeading.MatchString(text) || len(mdFence.FindAllString(text, -1)) >= 2:
		return Lang{"Markdown", ".md"}, true
	}
	return Lang{}, false
}

// nonBlank answers the lines of text that hold more than white space.
func nonBlank(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// isGo reports whether text parses as a Go file.
func isGo(text string) bool {
	parses := func(src string) bool {
		_, err := parser.ParseFile(token.NewFileSet(), "", src, parser.SkipObjectResolution)
		return err == nil
	}
	if strings.HasPrefix(text, "package ") || strings.HasPrefix(text, "//go:build") {
		_, err := parser.ParseFile(token.NewFileSet(), "", text, parser.PackageClauseOnly)
		return err == nil
	}
	if parses("package p\n" + text) {
		return true
	}
	// A run of statements reads as Go only beside a token no other language here writes.
	return (strings.Contains(text, ":=") || strings.Contains(text, "func ")) && parses("package p\nfunc _() {\n"+text+"\n}")
}

// shebang answers the language the program on a shebang line runs.
func shebang(line string) Lang {
	fields := strings.Fields(strings.TrimPrefix(line, "#!"))
	if len(fields) == 0 {
		return Lang{"shell", ".sh"}
	}
	program := path.Base(fields[0])
	if program == "env" {
		program = ""
		for _, f := range fields[1:] {
			if !strings.HasPrefix(f, "-") {
				program = path.Base(f)
				break
			}
		}
	}
	if strings.HasPrefix(program, "python") {
		return Lang{"Python", ".py"}
	}
	if lang, ok := interpreters[program]; ok {
		return lang
	}
	return Lang{"shell", ".sh"}
}

// dockerfile reports whether the lines open with FROM and hold another
// instruction.
func dockerfile(lines []string) bool {
	from := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if dockerFrom.MatchString(line) {
			from = i
		}
		break
	}
	if from < 0 {
		return false
	}
	// A parser directive names the Dockerfile frontend, so FROM alone is enough beside it.
	if dockerDirective.MatchString(lines[0]) {
		return true
	}
	for _, line := range lines[from+1:] {
		if dockerInstr.MatchString(line) {
			return true
		}
	}
	return false
}

// yamlDoc reports whether text is a YAML mapping with more than one key, a
// key that holds a collection, or a sequence of mappings.
func yamlDoc(text, first string) bool {
	if !yamlHead.MatchString(first) {
		return false
	}
	var doc yaml.Node
	if yaml.Unmarshal([]byte(text), &doc) != nil || len(doc.Content) == 0 {
		return false
	}
	root := doc.Content[0]
	switch root.Kind {
	case yaml.MappingNode:
		// A YAML mapping cannot repeat a key. Lines that repeat one prefix are text output.
		seen := map[string]bool{}
		for idx := 0; idx < len(root.Content); idx += 2 {
			if seen[root.Content[idx].Value] {
				return false
			}
			seen[root.Content[idx].Value] = true
		}
		return len(root.Content) >= 4 || (len(root.Content) == 2 && root.Content[1].Kind != yaml.ScalarNode && root.Content[1].Kind != yaml.AliasNode)
	case yaml.SequenceNode:
		return len(root.Content) > 0 && root.Content[0].Kind == yaml.MappingNode
	}
	return false
}

// shell reports whether text parses as a shell script with a compound command,
// or most of its lines run a common command.
func shell(text string, lines []string) bool {
	file, err := syntax.NewParser().Parse(strings.NewReader(text), "")
	if err != nil {
		return false
	}
	compound := false
	syntax.Walk(file, func(node syntax.Node) bool {
		switch node.(type) {
		case *syntax.IfClause, *syntax.ForClause, *syntax.WhileClause, *syntax.CaseClause, *syntax.FuncDecl:
			compound = true
		}
		return !compound
	})
	if compound {
		return true
	}
	words := 0
	for _, line := range lines {
		if shellWord.MatchString(line) {
			words++
		}
	}
	return words*2 > len(lines)
}
