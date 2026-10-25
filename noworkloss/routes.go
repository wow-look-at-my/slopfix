// noFlags is scanArgs' "this command has no value-taking flags" argument.

package noworkloss

import (
	"path/filepath"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"mvdan.cc/sh/v3/syntax"
)

// noFlags is scanArgs' "this command has no value-taking flags" argument.
var noFlags = set.Of[string]()

// A write is a file mutation a segment would perform.
type write struct {
	route string // how the message names the command
	paths []word // named targets, resolved against dir
	dir   string // where the write lands when paths is empty
	whole bool   // the write lands somewhere under dir rather than at a named path
	// opaque carries the reason a route's targets cannot be resolved, and denies.
	opaque string
	// fromScript marks a write read out of a script FILE. classify stamps it,
	fromScript bool
}

// classify returns every write a segment performs. What it closes is every
// route where the command text itself performs or directs the write.
func classify(seg segment, roots []string, aliases *aliasResolver, depth int) []write {
	return stampScriptWrites(classifyRoutes(seg, roots, aliases, depth), seg.fromScript)
}

// stampScriptWrites records where a write came from, in a single place, so a route
// added later inherits the answer instead of forgetting it.
func stampScriptWrites(out []write, fromScript bool) []write {
	if !fromScript {
		return out
	}
	for i := range out {
		out[i].fromScript = true
	}
	return out
}

func classifyRoutes(seg segment, roots []string, aliases *aliasResolver, depth int) []write {
	out := redirectWrites(seg)
	if len(seg.argv) == 0 {
		return out
	}
	name := commandName(seg.argv[0].text)
	rest := seg.argv[1:]

	if allowedFormatter(name, rest) {
		return out
	}
	if w, ok := gitWrites(seg, name, rest); ok {
		if len(w) > 0 {
			return append(out, w...)
		}
		// A verb with no write route of its own may still be an alias for a
		// write, so provenance sees through the same aliases.
		for _, expanded := range aliases.expand(seg, depth) {
			out = append(out, classify(expanded, roots, aliases, depth+1)...)
		}
		return out
	}
	if w, ok := remoteWrites(seg, name, rest); ok {
		return append(out, w...)
	}
	if w, ok := interpreterWrites(seg, name, rest, roots); ok {
		return append(out, w...)
	}
	return append(out, fileWrites(seg, name, rest, roots)...)
}

// redirectWrites covers `cmd > path`, `>>`, `>|`, `&>` and `<>`: the shell
// truncates or extends the file before the command it decorates even runs.
func redirectWrites(seg segment) []write {
	var out []write
	for _, r := range seg.redirs {
		if !writesFile(r) || isDeviceFile(r.file.text) {
			continue
		}
		op := ">"
		if r.op == syntax.AppOut || r.op == syntax.AppAll {
			op = ">>"
		}
		// Every descriptor counts: a file the tree holds is authored content
		out = append(out, write{route: redirLabel(r, op), paths: []word{r.file}, dir: seg.cwd})
	}
	return out
}

func writesFile(r redirTarget) bool {
	switch r.op {
	case syntax.RdrOut, syntax.AppOut, syntax.ClbOut, syntax.RdrAll, syntax.AppAll, syntax.RdrInOut:
		return true
	case syntax.DplOut:
		// A digit target duplicates a descriptor; `>&file` opens a file.
		t := r.file.text
		if t == "-" {
			return false
		}
		for _, c := range t {
			if c < '0' || c > '9' {
				return true
			}
		}
	}
	return false
}

func isDeviceFile(p string) bool {
	return p == "/dev/null" || p == "/dev/stdout" || p == "/dev/stderr" ||
		p == "/dev/tty" || strings.HasPrefix(p, "/dev/fd/")
}

// fileWrites is the catalog of programs that mutate a file named on their own
// argv. Each entry resolves the target rather than matching the program name and
// stopping there, so the denial can say which path it stopped.
func fileWrites(seg segment, name string, rest []word, roots []string) []write {
	one := func(route string, paths ...word) []write {
		return []write{{route: route, paths: paths, dir: seg.cwd}}
	}
	under := func(route, dir string) []write {
		return []write{{route: route, dir: dir, whole: true}}
	}

	switch name {
	case "sed":
		return nil
	case "awk", "gawk", "mawk", "nawk":
		return awkWrites(seg, rest)

	case "tee":
		_, operands := scanArgs(rest, noFlags)
		if len(operands) == 0 {
			return nil
		}
		return one("tee", operands...)

	case "sponge":
		_, operands := scanArgs(rest, noFlags)
		return one("sponge", operands...)

	case "dd":
		for _, a := range rest {
			if v, ok := strings.CutPrefix(a.text, "of="); ok {
				return one("dd of=", word{text: v, static: a.static})
			}
		}

	case "truncate":
		flags, operands := scanArgs(rest, set.Of[string]("-s", "--size", "-r", "--reference"))
		if _, ok := flags["-s"]; !ok {
			if _, ok := flags["--size"]; !ok {
				return nil
			}
		}
		return one("truncate", operands...)

	case "mv", "install", "rsync", "scp":
		return copyWrites(seg, name, rest, roots)

	case "ln":
		// A symlink replaces the path it is created at, so the link is written.
		flags, operands := scanArgs(rest, set.Of[string]("-t", "--target-directory"))
		if v, ok := flags["-t"]; ok {
			return under("ln -t", abs(seg.cwd, v.text))
		}
		if v, ok := flags["--target-directory"]; ok {
			return under("ln --target-directory", abs(seg.cwd, v.text))
		}
		switch len(operands) {
		case 0:
			return nil
		case 1:
			return one("ln", word{text: filepath.Base(operands[0].text), static: operands[0].static})
		case 2:
			if inTreeSymlink(seg.cwd, flags, operands[0], operands[1], roots) {
				return nil
			}
			return one("ln", operands[1])
		default:
			return under("ln", abs(seg.cwd, operands[len(operands)-1].text))
		}

	case "patch":
		// The files a patch touches are named inside the patch, not on the argv,
		flags, _ := scanArgs(rest, set.Of[string]("-o", "--output", "-d", "--directory", "-i", "--input", "-p", "--strip", "-B", "-D", "-r", "-z"))
		if v, ok := flags["-o"]; ok {
			return one("patch -o", v)
		}
		if v, ok := flags["--output"]; ok {
			return one("patch --output", v)
		}
		dir := seg.cwd
		if v, ok := flags["-d"]; ok {
			dir = abs(seg.cwd, v.text)
		} else if v, ok := flags["--directory"]; ok {
			dir = abs(seg.cwd, v.text)
		}
		return under("patch", dir)

	case "tar":
		return tarWrites(seg, rest)

	case "unzip":
		flags, _ := scanArgs(rest, set.Of[string]("-d", "-x"))
		dir := seg.cwd
		if v, ok := flags["-d"]; ok {
			dir = abs(seg.cwd, v.text)
		}
		return under("unzip", dir)

	case "xxd":
		flags, operands := scanArgs(rest, set.Of[string]("-c", "-g", "-l", "-o", "-s"))
		if _, ok := flags["-r"]; !ok {
			if _, ok := flags["--revert"]; !ok {
				return nil
			}
		}
		if len(operands) >= 2 {
			return one("xxd -r", operands[1])
		}

	case "base64", "openssl":
		flags, _ := scanArgs(rest, set.Of[string]("-o", "--output", "-out"))
		for _, f := range []string{"-o", "--output", "-out"} {
			if v, ok := flags[f]; ok && v.text != "" {
				return one(name+" "+f, v)
			}
		}

	case "curl":
		return curlWrites(seg, rest)

	case "wget":
		flags, _ := scanArgs(rest, set.Of[string]("-O", "--output-document", "-P", "--directory-prefix"))
		if v, ok := flags["-O"]; ok {
			return one("wget -O", v)
		}
		if v, ok := flags["--output-document"]; ok {
			return one("wget -O", v)
		}
		dir := seg.cwd
		if v, ok := flags["-P"]; ok {
			dir = abs(seg.cwd, v.text)
		} else if v, ok := flags["--directory-prefix"]; ok {
			dir = abs(seg.cwd, v.text)
		}
		return under("wget", dir)

	case "gh":
		return ghWrites(seg, rest)
	}

	if w, ok := auditedWrites(seg, name, rest); ok {
		return w
	}
	return inPlaceRewrite(seg, name, rest)
}
