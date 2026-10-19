package workflow

import (
	"regexp"
	"sort"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/ste"
	"mvdan.cc/sh/v3/syntax"
)

// IDTokenInGitURL is a git config line that puts a token in the user part of a github.com URL with no password.
const IDTokenInGitURL = "yaml/token-in-git-url"

// tokenURLKey matches the key of an insteadOf rewrite whose base URL holds a shell variable as the whole user part, with no password.
var tokenURLKey = regexp.MustCompile(`(?i)^["']?url\.["']?https?://\$(?:\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))@github\.com(?:/[^"'\s]*)?["']?\.insteadof["']?$`)

// tokenUser matches the scheme and the variable of such a URL, inside the text of the command.
var tokenUser = regexp.MustCompile(`(?i)(https?://)(\$(?:\{[A-Za-z_][A-Za-z0-9_]*\}|[A-Za-z_][A-Za-z0-9_]*))@github\.com`)

// tokenURLOptions are the options of a git config command that sets a rewrite. Any other option, such as --unset, makes the command no rewrite.
var tokenURLOptions = set.Of("--global", "--system", "--local", "--worktree", "--add", "--replace-all")

// tokenURL is a git config command that the repair changes. The row and column pairs are 0-based file
// positions, and the end is exclusive.
type tokenURL struct {
	row, col, endRow, endCol int
	// name is the environment variable that holds the token.
	name string
}

// tokenInGitURLs reports each git config command that puts a token in the user part of a github.com URL.
func tokenInGitURLs(content string) []ste.Finding {
	var out []ste.Finding
	for _, t := range tokenURLs(content) {
		out = append(out, ste.Finding{
			Line:   t.row + 1,
			ID:     IDTokenInGitURL,
			Rule:   "A token that is the whole user part of a github.com URL, with no password, fails on a private repository. git sends the `git-upload-pack` POST with no `Authorization` header, GitHub answers 401, and git prints `could not read Username`.",
			Detail: "$" + t.name,
			Fix:    "Write `x-access-token:` before the token, so the URL has a user and a password. `slopfix fix` writes it.",
		})
	}
	return out
}

// tokenURLs answers each such command in the file, in source order. A script the repair cannot write back is left out.
func tokenURLs(content string) []tokenURL {
	rows := lines(content)
	var out []tokenURL
	for _, s := range shellScripts(content) {
		if s.err == nil {
			out = append(out, s.tokenURLs(rows)...)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].row != out[j].row {
			return out[i].row < out[j].row
		}
		return out[i].col < out[j].col
	})
	return out
}

// tokenURLs answers the commands of one script.
func (s shellScript) tokenURLs(rows []string) []tokenURL {
	masked := maskExpressions(s.run.Value)
	file, err := syntax.NewParser(syntax.Variant(s.variant)).Parse(strings.NewReader(masked), "")
	if err != nil {
		return nil
	}
	scriptRows := strings.Split(s.run.Value, "\n")
	var out []tokenURL
	syntax.Walk(file, func(n syntax.Node) bool {
		call, ok := n.(*syntax.CallExpr)
		if !ok {
			return true
		}
		name, ok := gitConfigTokenURL(call, masked)
		if !ok {
			return true
		}
		first, last := call.Args[0], call.Args[len(call.Args)-1]
		row, col, startOK := s.fileSpot(rows, scriptRows, first.Pos())
		endRow, endCol, endOK := s.fileSpot(rows, scriptRows, last.End())
		if !startOK || !endOK {
			return true
		}
		out = append(out, tokenURL{row: row, col: col, endRow: endRow, endCol: endCol, name: name})
		return true
	})
	return out
}

// gitConfigTokenURL answers the variable of a `git config` command that sets such a rewrite.
func gitConfigTokenURL(call *syntax.CallExpr, script string) (name string, ok bool) {
	args := call.Args
	if len(args) < 3 || wordSource(args[0], script) != "git" || wordSource(args[1], script) != "config" {
		return "", false
	}
	var key string
	for _, arg := range args[2:] {
		text := wordSource(arg, script)
		switch {
		case strings.HasPrefix(text, "-"):
			if !tokenURLOptions.Contains(text) {
				return "", false
			}
		case key == "":
			key = text
		}
	}
	m := tokenURLKey.FindStringSubmatch(key)
	if m == nil {
		return "", false
	}
	return m[1] + m[2], true
}

// wordSource answers the text of a word as the script wrote it.
func wordSource(w *syntax.Word, script string) string {
	return script[w.Pos().Offset():w.End().Offset()]
}

// withUsername writes x-access-token as the user of the URL, and leaves the variable as its password.
func withUsername(command string) string {
	return tokenUser.ReplaceAllString(command, "${1}x-access-token:${2}@github.com")
}

// fileSpot answers the file position of a script position. ok is false when the file does not hold the script
// text verbatim at that line. This covers a quoted or folded scalar, or a plain scalar over several rows.
func (s shellScript) fileSpot(rows, scriptRows []string, pos syntax.Pos) (row, col int, ok bool) {
	line, at := int(pos.Line())-1, int(pos.Col())-1
	if line < 0 || line >= len(scriptRows) {
		return 0, 0, false
	}
	if s.literal {
		row = s.run.Line + line
		if row >= len(rows) || !strings.HasSuffix(rows[row], scriptRows[line]) {
			return 0, 0, false
		}
		return row, len(rows[row]) - len(scriptRows[line]) + at, true
	}
	row, base := s.run.Line-1, s.run.Column-1
	if s.run.Style != 0 || line != 0 || row < 0 || row >= len(rows) || base > len(rows[row]) || !strings.HasPrefix(rows[row][base:], s.run.Value) {
		return 0, 0, false
	}
	return row, base + at, true
}

// addTokenUsername writes the user into each rewrite. Commands that share a row are written together, as one
// edit of whole rows.
func addTokenUsername(content string) []edit.Edit {
	found := tokenURLs(content)
	rows := lines(content)
	var out []edit.Edit
	for i := 0; i < len(found); {
		first, last := found[i].row, found[i].endRow
		j := i + 1
		for ; j < len(found) && found[j].row <= last; j++ {
			last = max(last, found[j].endRow)
		}
		out = append(out, rewrite(content, first, last, spliceUsernames(rows[first:last+1], first, found[i:j])))
		i = j
	}
	return out
}

// spliceUsernames writes the user into each command in the rows and answers the rows that result. A span that
// starts inside the one before it is left alone.
func spliceUsernames(rows []string, base int, found []tokenURL) []string {
	starts := make([]int, len(rows))
	for r := 1; r < len(rows); r++ {
		starts[r] = starts[r-1] + len(rows[r-1]) + 1
	}
	text := strings.Join(rows, "\n")
	for k := len(found) - 1; k >= 0; k-- {
		t := found[k]
		from, to := starts[t.row-base]+t.col, starts[t.endRow-base]+t.endCol
		if k+1 < len(found) && to > starts[found[k+1].row-base]+found[k+1].col {
			continue
		}
		text = text[:from] + withUsername(text[from:to]) + text[to:]
	}
	return strings.Split(text, "\n")
}
