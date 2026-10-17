package gitread

import (
	"os/exec"
	"strings"
)

// A reftable repository keeps its refs in a binary format. Git reads them,
// and every object read stays in-process.

// gitRef runs git on this repository's refs and answers its trimmed output.
func (r *Repo) gitRef(args ...string) (string, bool) {
	full := append([]string{"--git-dir=" + r.gitDir, "-c", "safe.directory=*"}, args...)
	out, err := exec.Command("git", full...).Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

func (r *Repo) reftableRef(name string) (OID, bool) {
	out, ok := r.gitRef("rev-parse", "--verify", "--quiet", "--end-of-options", name)
	if !ok {
		return OID{}, false
	}
	oid, err := ParseOID(out)
	return oid, err == nil
}

func (r *Repo) reftableSymbolic(name string) (string, bool) {
	out, ok := r.gitRef("symbolic-ref", "--quiet", name)
	return out, ok && out != ""
}

func (r *Repo) reftableRefs(prefix string) map[string]OID {
	out := map[string]OID{}
	listed, _ := r.gitRef("for-each-ref", "--format=%(objectname) %(refname)", prefix)
	for _, line := range strings.Split(listed, "\n") {
		hex, ref, ok := strings.Cut(line, " ")
		if oid, err := ParseOID(hex); ok && err == nil {
			out[ref] = oid
		}
	}
	return out
}
