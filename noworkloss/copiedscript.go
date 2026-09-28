package noworkloss

import (
	"github.com/wow-look-at-my/go-containers/set"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// A new script that repeats another script's lines is a variant of that
// script. The variant belongs in it as an option or a subcommand, reached with
// Edit. A folder of set-x, set-y and set-z files, each carrying the same fetch
// and the same PUT, is what this refuses.

// scriptExts are the files run as programs, where a copy becomes a second
// place to fix every bug.
var scriptExts = set.Of[string](".js", ".mjs", ".cjs",
	".ts", ".mts", ".cts",
	".py", ".sh", ".bash", ".zsh",
	".ps1", ".rb", ".pl")

// copiedLineFloor is how many substantial lines a new script may share with
// one sibling.
const copiedLineFloor = 4

// substantialLen keeps braces, `}`, `return;` and blank lines out of the count.
const substantialLen = 20

// siblingSizeCap keeps a generated or vendored file out of the comparison.
const siblingSizeCap = 1 << 20

// copiedScriptReason refuses a new script whose lines repeat a script already
// in the same directory. A sibling with the same stem is the same script in
// another language, so a port is left alone.
func copiedScriptReason(path, content string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if !scriptExts.Contains(ext) || content == "" {
		return ""
	}
	added := substantialLines(content)
	if len(added) < copiedLineFloor {
		return ""
	}
	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	best, bestShared := "", []string(nil)
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() || !scriptExts.Contains(strings.ToLower(filepath.Ext(name))) {
			continue
		}
		if strings.TrimSuffix(name, filepath.Ext(name)) == stem {
			continue
		}
		shared := sharedLines(filepath.Join(dir, name), added)
		if len(shared) > len(bestShared) {
			best, bestShared = name, shared
		}
	}
	if len(bestShared) < copiedLineFloor {
		return ""
	}
	sort.Strings(bestShared)
	return "blocked: " + path + " would be a new script that repeats " + strconv.Itoa(len(bestShared)) + " lines of " + best + ", for example:\n" +
		"  " + bestShared[0] + "\n" +
		"A variation on an existing script is an option or a subcommand of that script, not another file. " +
		"Add it to " + filepath.Join(dir, best) + " with the Edit tool. " +
		"If this is a separate tool, put the shared code in one module that both scripts import."
}

// substantialLines is the set of trimmed lines that carry code, without
// comments and without short structural lines.
func substantialLines(content string) map[string]bool {
	lines := map[string]bool{}
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if len(line) < substantialLen || isCommentLine(line) {
			continue
		}
		lines[line] = true
	}
	return lines
}

func isCommentLine(line string) bool {
	for _, prefix := range []string{"//", "#", "/*", "*", "--", "<#"} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

// sharedLines answers which of the added lines the sibling already carries.
// A sibling that cannot be read shares nothing.
func sharedLines(sibling string, added map[string]bool) []string {
	info, err := os.Stat(sibling)
	if err != nil || info.Size() > siblingSizeCap {
		return nil
	}
	data, err := os.ReadFile(sibling)
	if err != nil {
		return nil
	}
	var shared []string
	for line := range substantialLines(string(data)) {
		if added[line] {
			shared = append(shared, line)
		}
	}
	return shared
}
