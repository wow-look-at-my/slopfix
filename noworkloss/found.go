// found.go holds Found work. Work the session finds that exists only on this
// machine reaches a remote before the session moves on. The guard refuses no
// command for it. It commits and pushes the work itself, then lets the
// command run.
package noworkloss

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// foundState is what the guard keeps for a session: every repository the
// session touched, by its top directory.
type foundState struct {
	Repos []string `json:"repos"`
}

// foundStatePath answers the state file of a session.
func foundStatePath(session string) string {
	sum := sha256.Sum256([]byte(session))
	return filepath.Join(os.TempDir(), "slopfix-no-work-loss", hex.EncodeToString(sum[:])[:16]+".json")
}

func readFoundState(session string) foundState {
	var st foundState
	if data, err := os.ReadFile(foundStatePath(session)); err == nil {
		_ = json.Unmarshal(data, &st)
	}
	return st
}

// recordRepos adds the repository that holds each directory to the session's
// state, the first time the session touches it. It answers every repository
// it found.
func recordRepos(session string, dirs []string) []string {
	var roots []string
	for _, dir := range dirs {
		if root := foundRoot(dir); root != "" && !slices.Contains(roots, root) {
			roots = append(roots, root)
		}
	}
	if session == "" || len(roots) == 0 {
		return roots
	}
	st := readFoundState(session)
	changed := false
	for _, root := range roots {
		if !slices.Contains(st.Repos, root) {
			st.Repos = append(st.Repos, root)
			changed = true
		}
	}
	if !changed {
		return roots
	}
	path := foundStatePath(session)
	if os.MkdirAll(filepath.Dir(path), 0o700) == nil {
		if data, err := json.Marshal(st); err == nil {
			_ = os.WriteFile(path, data, 0o600)
		}
	}
	return roots
}

// foundRoot answers the top directory of the repository that holds path, a
// file or a directory, or "".
func foundRoot(path string) string {
	if path == "" {
		return ""
	}
	dir := filepath.Clean(path)
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	return repoRoot(dir)
}

// foundTrigger is a command that sets work aside, and the repositories whose
// work it sets aside.
type foundTrigger struct {
	roots []string
	// exclude is the directory pattern a worktree inside its repository adds to info/exclude.
	exclude map[string]string
	// embedded maps a repository to the undeclared nested repositories a git add would record.
	embedded map[string][]string
	// dropOperands are the operands written into a git add that name an embedded repository.
	dropOperands []string
}

// foundCommand reads a Bash command for the repositories it touches and the
// work it sets aside.
func foundCommand(command, cwd string) (touched []string, trig foundTrigger) {
	touched = []string{cwd}
	segs, _, ok := parseSegments(command, cwd)
	if !ok {
		return touched, trig
	}
	trig.exclude = map[string]string{}
	trig.embedded = map[string][]string{}
	for _, seg := range segs {
		argv := stripWrappers(seg.argv)
		if len(argv) == 0 {
			continue
		}
		touched = append(touched, seg.cwd)
		name := commandName(argv[0].text)
		for _, a := range argv[1:] {
			if a.static && !strings.HasPrefix(a.text, "-") {
				if p := joinDir(seg.cwd, a); p != unknownDirText {
					if _, err := os.Stat(p); err == nil {
						touched = append(touched, p)
					}
				}
			}
		}
		switch name {
		case "git":
			gitTrigger(argv, seg, &trig, &touched)
		case "cp", "rsync", "ditto":
			copyTrigger(name, argv, seg.cwd, &trig)
		}
	}
	return touched, trig
}

// gitTrigger reads one git call for the work it sets aside.
func gitTrigger(argv []word, seg segment, trig *foundTrigger, touched *[]string) {
	g, ok := parseGit(argv, seg.cwd, seg.relocated)
	if !ok || g.dir == unknownDirText {
		return
	}
	*touched = append(*touched, g.dir)
	root := foundRoot(g.dir)
	switch g.verb {
	case "worktree":
		if g.sub != "add" || root == "" {
			return
		}
		trig.roots = append(trig.roots, root)
		if path := worktreePath(g.argv); path != "" {
			abs := path
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(g.dir, abs)
			}
			if pattern := insidePattern(root, abs); pattern != "" {
				trig.exclude[root] = pattern
			}
		}
	case "stash":
		if root != "" && (g.sub == "" || g.sub == "push" || g.sub == "save" || strings.HasPrefix(g.sub, "-")) {
			trig.roots = append(trig.roots, root)
		}
	case "clone":
		if len(g.operands) == 0 {
			return
		}
		src := g.operands[0]
		if src.static {
			if p := joinDir(seg.cwd, src); p != unknownDirText {
				if r := foundRoot(p); r != "" && sameDir(r, p) {
					trig.roots = append(trig.roots, r)
				}
			}
			trig.roots = append(trig.roots, cloneOfRemote(src.text)...)
		}
	case "add":
		if root == "" {
			return
		}
		for _, nested := range undeclaredRepos(root) {
			if addRecords(g, root, nested) {
				trig.embedded[root] = append(trig.embedded[root], nested)
				for _, op := range g.operands {
					if sameDir(joinDir(g.dir, op), filepath.Join(root, nested)) {
						trig.dropOperands = append(trig.dropOperands, op.text)
					}
				}
			}
		}
	}
}

// worktreePath answers the path operand of `git worktree add`.
func worktreePath(argv []word) string {
	at := -1
	for i, a := range argv {
		if a.text == "worktree" && i+1 < len(argv) && argv[i+1].text == "add" {
			at = i + 2
			break
		}
	}
	if at < 0 {
		return ""
	}
	for i := at; i < len(argv); i++ {
		t := argv[i].text
		switch {
		case t == "-b" || t == "-B" || t == "--reason":
			i++
		case strings.HasPrefix(t, "-"):
		default:
			if !argv[i].static {
				return ""
			}
			return t
		}
	}
	return ""
}

// insidePattern answers the info/exclude pattern for a worktree at path
// inside root: the directory that holds it, anchored at the root. A path
// outside root answers "".
func insidePattern(root, path string) string {
	rel, err := filepath.Rel(root, filepath.Clean(path))
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	parent := filepath.Dir(rel)
	if parent == "." {
		parent = rel
	}
	return "/" + filepath.ToSlash(parent) + "/"
}

// copyTrigger reads a copy of a recorded repository's directory.
func copyTrigger(name string, argv []word, cwd string, trig *foundTrigger) {
	var operands []word
	recursive := name != "cp"
	for _, a := range argv[1:] {
		if strings.HasPrefix(a.text, "-") && !strings.HasPrefix(a.text, "--") && strings.ContainsAny(a.text[1:], "rRa") {
			recursive = true
		}
		if a.text == "--recursive" || a.text == "--archive" {
			recursive = true
		}
		if !strings.HasPrefix(a.text, "-") {
			operands = append(operands, a)
		}
	}
	if !recursive || len(operands) < 2 {
		return
	}
	for _, src := range operands[:len(operands)-1] {
		if !src.static {
			continue
		}
		p := joinDir(cwd, word{text: strings.TrimSuffix(src.text, "/"), static: true})
		if r := foundRoot(p); r != "" && sameDir(r, p) {
			trig.roots = append(trig.roots, r)
		}
	}
}

// sameDir reports whether paths name one directory.
func sameDir(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}

// cloneOfRemote answers each recorded repository whose origin is url. The
// caller has no session here, so it reads every state file this machine keeps.
func cloneOfRemote(url string) []string {
	want := normalRemote(url)
	if want == "" {
		return nil
	}
	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "slopfix-no-work-loss", "*.json"))
	var out []string
	for _, m := range matches {
		var st foundState
		data, err := os.ReadFile(m)
		if err != nil || json.Unmarshal(data, &st) != nil {
			continue
		}
		for _, root := range st.Repos {
			got, _, err := runGit(root, "remote", "get-url", "origin")
			if err == nil && normalRemote(got) == want && !slices.Contains(out, root) {
				out = append(out, root)
			}
		}
	}
	return out
}

func normalRemote(url string) string {
	url = strings.TrimSpace(url)
	url = strings.TrimSuffix(strings.TrimSuffix(url, "/"), ".git")
	return url
}

// undeclaredRepos answers each nested repository in root's untracked content
// that .gitmodules does not declare, by its path from root.
func undeclaredRepos(root string) []string {
	out, _, err := runGit(root, "ls-files", "--others", "--exclude-standard", "--directory", "-z")
	if err != nil {
		return nil
	}
	declared := map[string]bool{}
	if mods, _, err := runGit(root, "config", "-f", ".gitmodules", "--get-regexp", `\.path$`); err == nil {
		for _, line := range strings.Split(mods, "\n") {
			if f := strings.Fields(line); len(f) == 2 {
				declared[strings.TrimSuffix(f[1], "/")] = true
			}
		}
	}
	var repos []string
	for _, p := range strings.Split(out, "\x00") {
		if !strings.HasSuffix(p, "/") {
			continue
		}
		p = strings.TrimSuffix(p, "/")
		if declared[p] {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, p, ".git")); err == nil {
			repos = append(repos, p)
		}
	}
	return repos
}

// addRecords reports whether a git add records the nested repository at path:
// it adds everything, or names the path or a directory that holds it.
func addRecords(g *gitCall, root, nested string) bool {
	if g.has("-A", "--all", "-u", "--update") && len(g.operands) == 0 {
		return !g.has("-u", "--update")
	}
	target := filepath.Join(root, nested)
	for _, op := range g.operands {
		if !op.static {
			continue
		}
		p := joinDir(g.dir, op)
		if p == unknownDirText {
			continue
		}
		rel, err := filepath.Rel(p, target)
		if err == nil && (rel == "." || !strings.HasPrefix(rel, "..")) {
			return true
		}
	}
	return false
}

// excludePath appends a pattern to the repository's info/exclude, unless a
// line there already ignores the path it stands for.
func excludePath(root, pattern string) error {
	probe := strings.Trim(pattern, "/") + "/x"
	if _, _, err := runGit(root, "check-ignore", "-q", "--no-index", probe); err == nil {
		return nil
	}
	common, _, err := runGit(root, "rev-parse", "--git-common-dir")
	if err != nil {
		return err
	}
	dir := strings.TrimSpace(common)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	path := filepath.Join(dir, "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	existing, _ := os.ReadFile(path)
	text := string(existing)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return os.WriteFile(path, []byte(text+pattern+"\n"), 0o644)
}

// foundOutput matches command output that lists work only this machine holds. This is a status that names a changed or untracked file, or a branch ahead of its remote.
var foundOutput = regexp.MustCompile(`(?m)^(?:\s*(?:modified|deleted|new file|renamed|typechange):\s|Untracked files:|Changes not staged for commit|Changes to be committed|Your branch is ahead of|[ MADRCUT?][ MADRCUT?] \S|## .*\[ahead \d+|[12u] [.MADRCUT]{2} |\? \S)`)

// showsFoundWork reports output that showed found work.
func showsFoundWork(output string) bool { return foundOutput.MatchString(output) }

// dropWords removes each named operand from a command, where it stands as a whole word.
func dropWords(command string, words []string) string {
	for _, w := range words {
		re := regexp.MustCompile(`\s` + regexp.QuoteMeta(w) + `/?(\s|$|[;&|)])`)
		command = re.ReplaceAllString(command, "$1")
	}
	return command
}
