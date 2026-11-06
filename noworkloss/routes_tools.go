// routes_tools.go names the files that copy, archive, download and in-place rewrite tools write.

package noworkloss

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// copyWrites is where the rule's real shape shows: this is about content
// ENTERING the tree without a tool call. Bytes already in the tree have been
// through a tool call, so moving them around -- `mv old.go new.go` -- is
// ordinary refactoring. A source from outside is the splice this closes: write a
// file to /tmp with Write, then move it over the target.
//
// `cp` is NOT routed here. Copying is how a tree of files gets put in place,
// and denying it forces every such move through the Write tool per file.
func copyWrites(seg segment, name string, rest []word, roots []string) []write {
	flags, operands := scanArgs(rest, set.Of[string](
		"-t", "--target-directory", "-S", "--suffix",
		"-m", "--mode", "-o", "--owner", "-g", "--group",
		"-e", "--rsh", "--exclude", "--include", "--files-from",
	))
	if v, ok := flags["-t"]; ok {
		if fromInsideTree(seg.cwd, operands, roots) {
			return nil
		}
		return []write{{route: name + " -t", dir: abs(seg.cwd, v.text), whole: true}}
	}
	if v, ok := flags["--target-directory"]; ok {
		if fromInsideTree(seg.cwd, operands, roots) {
			return nil
		}
		return []write{{route: name + " --target-directory", dir: abs(seg.cwd, v.text), whole: true}}
	}
	if len(operands) < 2 {
		return nil
	}
	if fromInsideTree(seg.cwd, operands[:len(operands)-1], roots) {
		return nil
	}
	dst := operands[len(operands)-1]
	// rsync's trailing slash, several sources, or an existing directory all mean
	if len(operands) > 2 || strings.HasSuffix(dst.text, "/") {
		return []write{{route: name, dir: abs(seg.cwd, dst.text), whole: true}}
	}
	return []write{{route: name, paths: []word{dst}, dir: seg.cwd}}
}

func tarWrites(seg segment, rest []word) []write {
	flags, _ := scanArgs(rest, set.Of[string]("-f", "--file", "-C", "--directory"))
	has := func(names ...string) bool {
		for _, n := range names {
			if _, ok := flags[n]; ok {
				return true
			}
		}
		return false
	}
	dir := seg.cwd
	if v, ok := flags["-C"]; ok {
		dir = abs(seg.cwd, v.text)
	} else if v, ok := flags["--directory"]; ok {
		dir = abs(seg.cwd, v.text)
	}
	if has("-x", "--extract", "--get") {
		return []write{{route: "tar -x", dir: dir, whole: true}}
	}
	if has("-c", "--create") {
		if v, ok := flags["-f"]; ok {
			return []write{{route: "tar -f", paths: []word{v}, dir: seg.cwd}}
		}
		if v, ok := flags["--file"]; ok {
			return []write{{route: "tar --file", paths: []word{v}, dir: seg.cwd}}
		}
	}
	return nil
}

func curlWrites(seg segment, rest []word) []write {
	flags, _ := scanArgs(rest, set.Of[string](
		"-o", "--output", "--output-dir", "-H", "--header",
		"-d", "--data", "-X", "--request", "-u", "--user",
		"-A", "--user-agent", "-b", "--cookie", "-c", "--cookie-jar",
		"-w", "--write-out", "-D", "--dump-header", "-T", "--upload-file",
	))
	var out []write
	for _, f := range []string{"-o", "--output", "--cookie-jar", "-c", "-D", "--dump-header"} {
		if v, ok := flags[f]; ok && v.text != "" && !isDeviceFile(v.text) {
			out = append(out, write{route: "curl " + f, paths: []word{v}, dir: seg.cwd})
		}
	}
	if _, ok := flags["-O"]; ok {
		dir := seg.cwd
		if v, ok := flags["--output-dir"]; ok {
			dir = abs(seg.cwd, v.text)
		}
		out = append(out, write{route: "curl -O", dir: dir, whole: true})
	}
	return out
}

func ghWrites(seg segment, rest []word) []write {
	_, operands := scanArgs(rest, noFlags)
	if len(operands) < 2 || operands[0].text != "release" || operands[1].text != "download" {
		return nil
	}
	flags, _ := scanArgs(rest, set.Of[string]("-O", "--output", "-D", "--dir", "-p", "--pattern", "-A", "--archive", "-R", "--repo"))
	if v, ok := flags["-O"]; ok {
		return []write{{route: "gh release download -O", paths: []word{v}, dir: seg.cwd}}
	}
	if v, ok := flags["--output"]; ok {
		return []write{{route: "gh release download -O", paths: []word{v}, dir: seg.cwd}}
	}
	dir := seg.cwd
	if v, ok := flags["-D"]; ok {
		dir = abs(seg.cwd, v.text)
	} else if v, ok := flags["--dir"]; ok {
		dir = abs(seg.cwd, v.text)
	}
	return []write{{route: "gh release download", dir: dir, whole: true}}
}

// inPlaceRewrite is the rule for a program this catalog does not name. A long
// in-place flag says the program rewrites its files, whatever it is. Short
// `-i` and `-w` are ambiguous, so they count only for the tools listed below.
func inPlaceRewrite(seg segment, name string, rest []word) []write {
	longFlag := false
	shortFlag := false
	for _, a := range rest {
		t := a.text
		switch {
		case t == "--in-place" || t == "--write" || strings.HasPrefix(t, "--in-place="):
			longFlag = true
		case t == "-i" || t == "-w":
			shortFlag = true
		}
	}
	if !longFlag && !(shortFlag && shortInPlaceTools.Contains(name)) {
		return nil
	}
	_, operands := scanArgs(rest, noFlags)
	// An unrecognised tool's operands hold its subcommand as well as its files
	var targets []word
	for _, o := range operands {
		if looksLikePath(seg.cwd, o) {
			targets = append(targets, o)
		}
	}
	if len(targets) == 0 {
		return nil
	}
	return []write{{route: name + " (in-place rewrite)", paths: targets, dir: seg.cwd}}
}

// fromInsideTree reports whether every source is content the tree already holds.
// An unknowable source is not, which is what keeps `cp $SRC tracked.go` denied.
func fromInsideTree(cwd string, sources []word, roots []string) bool {
	if len(sources) == 0 {
		return false
	}
	for _, s := range sources {
		if !s.static || isRemoteSpec(s.text) {
			return false
		}
		p := abs(cwd, s.text)
		if p == "" {
			return false
		}
		if _, inside := insideGuarded(roots, p); !inside {
			return false
		}
	}
	return true
}

func looksLikePath(cwd string, o word) bool {
	if !o.static {
		return true // unknowable, and unknowable denies
	}
	// An expression is not a filename, and it carries a space.
	if strings.ContainsAny(o.text, " \t") {
		return false
	}
	if strings.Contains(o.text, "/") || hasFileExtension(o.text) {
		return true
	}
	if p := abs(cwd, o.text); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

func hasFileExtension(s string) bool {
	ext := filepath.Ext(s)
	if len(ext) < 2 || len(ext) > 9 {
		return false
	}
	for _, c := range ext[1:] {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// inTreeSymlink reports a symbolic link whose target is a relative path to a
// file the same working tree guards. No edit tool can make a symlink, and its
// only content is a pointer to authored text. A target outside the tree, or in
// build output, stays a write, because it brings in text no edit tool wrote.
func inTreeSymlink(cwd string, flags map[string]word, target, link word, roots []string) bool {
	_, short := flags["-s"]
	_, long := flags["--symbolic"]
	if !short && !long || !target.static || !link.static || filepath.IsAbs(target.text) {
		return false
	}
	linkPath := abs(cwd, link.text)
	if linkPath == "" {
		return false
	}
	if st, err := os.Stat(linkPath); err == nil && st.IsDir() {
		linkPath = filepath.Join(linkPath, filepath.Base(target.text))
	}
	targetPath := filepath.Join(filepath.Dir(linkPath), target.text)
	linkRoot, linkIn := insideGuarded(roots, linkPath)
	targetRoot, targetIn := insideGuarded(roots, targetPath)
	if !linkIn || !targetIn || linkRoot != targetRoot {
		return false
	}
	// A target that is itself a link must also end in the tree.
	if st, err := os.Lstat(targetPath); err == nil && st.Mode()&os.ModeSymlink != 0 {
		real, err := filepath.EvalSymlinks(targetPath)
		realRoot, rootErr := filepath.EvalSymlinks(linkRoot)
		if err != nil || rootErr != nil {
			return false
		}
		rel, err := filepath.Rel(realRoot, real)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	return true
}

// Tools that spell in-place rewriting with a bare short flag. Whether the
// rewrite is allowed stays the formatter table's decision.
var shortInPlaceTools = set.Of[string]("gofmt", "goimports", "shfmt", "ffs",
	"rustfmt", "clang-format", "buf", "yq")

// scanArgs splits an argv into its flags and its operands. A flag named in
// valueFlags consumes the next word, which is what keeps a count argument from
// being read as a path. Bundled short flags each register on their own.
func scanArgs(rest []word, valueFlags set.Set[string]) (map[string]word, []word) {
	flags := map[string]word{}
	var operands []word
	dashDash := false
	for i := 0; i < len(rest); i++ {
		t := rest[i].text
		switch {
		case dashDash:
			operands = append(operands, rest[i])
		case t == "--":
			dashDash = true
		case strings.HasPrefix(t, "--"):
			if k, v, ok := strings.Cut(t, "="); ok {
				flags[k] = word{text: v, static: rest[i].static}
				continue
			}
			if valueFlags.Contains(t) && i+1 < len(rest) {
				i++
				flags[t] = rest[i]
				continue
			}
			flags[t] = word{static: true}
		case len(t) > 1 && strings.HasPrefix(t, "-"):
			if valueFlags.Contains(t) && i+1 < len(rest) {
				i++
				flags[t] = rest[i]
				continue
			}
			flags[t] = word{text: t[1:], static: rest[i].static}
			for _, c := range t[1:] {
				short := "-" + string(c)
				if _, seen := flags[short]; !seen {
					flags[short] = word{static: true}
				}
			}
		default:
			operands = append(operands, rest[i])
		}
	}
	return flags, operands
}
