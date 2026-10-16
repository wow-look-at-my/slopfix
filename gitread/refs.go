package gitread

import (
	"errors"
	"fmt"
	"github.com/wow-look-at-my/go-containers/set"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrUnknownRevision marks a revision this package cannot name. A caller may
// ask git for a spelling this package does not parse, such as a reflog entry.
var ErrUnknownRevision = errors.New("gitread: the revision is unknown")

// maxSymrefDepth stops a symbolic ref cycle.
const maxSymrefDepth = 10

// Head answers the commit HEAD names.
func (r *Repo) Head() (OID, error) { return r.Resolve("HEAD") }

// Resolve answers the object a revision names: a full or abbreviated object
// name, or a ref by its full or short name, with ~N, ^N and ^{type} suffixes.
func (r *Repo) Resolve(rev string) (OID, error) {
	base, ops := splitRevision(rev)
	oid, err := r.resolveBase(base)
	if err != nil {
		return OID{}, err
	}
	for _, op := range ops {
		if oid, err = r.applySuffix(oid, op); err != nil {
			return OID{}, fmt.Errorf("gitread: %s: %w", rev, err)
		}
	}
	return oid, nil
}

// splitRevision cuts the suffix operators off a revision.
func splitRevision(rev string) (string, []string) {
	cut := len(rev)
	for i := 0; i < len(rev); i++ {
		if rev[i] == '~' || rev[i] == '^' {
			cut = i
			break
		}
	}
	base, rest := rev[:cut], rev[cut:]
	var ops []string
	for len(rest) > 0 {
		end := 1
		if strings.HasPrefix(rest, "^{") {
			end = strings.IndexByte(rest, '}') + 1
			if end == 0 {
				end = len(rest)
			}
		} else {
			for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
				end++
			}
		}
		ops = append(ops, rest[:end])
		rest = rest[end:]
	}
	return base, ops
}

func (r *Repo) applySuffix(oid OID, op string) (OID, error) {
	switch {
	case op == "^{}" || op == "^{commit}":
		return r.PeelCommit(oid)
	case op == "^{tree}":
		commit, err := r.PeelCommit(oid)
		if err != nil {
			return OID{}, err
		}
		c, err := r.Commit(commit)
		if err != nil {
			return OID{}, err
		}
		return c.Tree, nil
	case strings.HasPrefix(op, "^{"):
		return OID{}, ErrUnknownRevision
	}
	n := 1
	if len(op) > 1 {
		v, err := strconv.Atoi(op[1:])
		if err != nil {
			return OID{}, ErrUnknownRevision
		}
		n = v
	}
	commit, err := r.PeelCommit(oid)
	if err != nil {
		return OID{}, err
	}
	if op[0] == '^' {
		if n == 0 {
			return commit, nil
		}
		c, err := r.Commit(commit)
		if err != nil {
			return OID{}, err
		}
		if n > len(c.Parents) {
			return OID{}, fmt.Errorf("%s has no parent number %d", commit, n)
		}
		return c.Parents[n-1], nil
	}
	for i := 0; i < n; i++ {
		c, err := r.Commit(commit)
		if err != nil {
			return OID{}, err
		}
		if len(c.Parents) == 0 {
			return OID{}, fmt.Errorf("%s has no parent", commit)
		}
		commit = c.Parents[0]
	}
	return commit, nil
}

// resolveBase answers a revision without suffixes, in the order git tries.
func (r *Repo) resolveBase(name string) (OID, error) {
	if name == "" || name == "@" {
		name = "HEAD"
	}
	if oid, err := ParseOID(name); err == nil {
		return oid, nil
	}
	for _, ref := range []string{name, "refs/" + name, "refs/tags/" + name, "refs/heads/" + name, "refs/remotes/" + name, "refs/remotes/" + name + "/HEAD"} {
		if oid, ok := r.ref(ref, 0); ok {
			return oid, nil
		}
	}
	if oid, ok := r.abbreviated(name); ok {
		return oid, nil
	}
	return OID{}, fmt.Errorf("%w: %s", ErrUnknownRevision, name)
}

// minAbbrev is the shortest object name prefix git accepts.
const minAbbrev = 4

// abbreviated answers the one object whose name starts with prefix.
func (r *Repo) abbreviated(prefix string) (OID, bool) {
	if len(prefix) < minAbbrev || len(prefix) >= len(OID{})*2 {
		return OID{}, false
	}
	digits := make([]byte, (len(prefix)+1)/2)
	for i := 0; i < len(prefix); i++ {
		v, ok := hexVal(prefix[i])
		if !ok {
			return OID{}, false
		}
		if i%2 == 0 {
			digits[i/2] = v << 4
		} else {
			digits[i/2] |= v
		}
	}
	odd := len(prefix)%2 == 1
	found := set.New[OID]()
	var scan func(db *odb)
	scan = func(db *odb) {
		for _, p := range db.packList(false) {
			p.prefix(digits, odd, func(oid OID) { found.Add(oid) })
		}
		lower := strings.ToLower(prefix)
		entries, _ := os.ReadDir(filepath.Join(db.dir, lower[:2]))
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), lower[2:]) {
				if oid, err := ParseOID(lower[:2] + e.Name()); err == nil {
					found.Add(oid)
				}
			}
		}
		for _, alt := range db.alternates() {
			scan(alt)
		}
	}
	scan(r.db)
	if found.Len() != 1 {
		return OID{}, false
	}
	for oid := range found.All() {
		return oid, true
	}
	return OID{}, false
}

// ref answers a ref by its full name, following symbolic refs.
func (r *Repo) ref(name string, depth int) (OID, bool) {
	if depth > maxSymrefDepth || strings.Contains(name, "..") {
		return OID{}, false
	}
	if r.reftable {
		return r.reftableRef(name)
	}
	for _, base := range []string{r.gitDir, r.commonDir} {
		data, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(name)))
		if err != nil {
			continue
		}
		line := strings.TrimSpace(string(data))
		if target, ok := strings.CutPrefix(line, "ref:"); ok {
			return r.ref(strings.TrimSpace(target), depth+1)
		}
		oid, err := ParseOID(line)
		return oid, err == nil
	}
	oid, ok := r.packedRefs()[name]
	return oid, ok
}

// SymbolicRef answers the ref a symbolic ref names, or false.
func (r *Repo) SymbolicRef(name string) (string, bool) {
	if r.reftable {
		return r.reftableSymbolic(name)
	}
	for _, base := range []string{r.gitDir, r.commonDir} {
		data, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(name)))
		if err != nil {
			continue
		}
		target, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "ref:")
		return strings.TrimSpace(target), ok
	}
	return "", false
}

// packedRefs answers the packed-refs file, read again when it changes.
func (r *Repo) packedRefs() map[string]OID {
	path := filepath.Join(r.commonDir, "packed-refs")
	stamp := stampOf(path)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.packed != nil && stamp == r.packStamp {
		return r.packed
	}
	out := map[string]OID{}
	data, _ := os.ReadFile(path)
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || line[0] == '#' || line[0] == '^' {
			continue
		}
		hex, ref, ok := strings.Cut(strings.TrimSpace(line), " ")
		if oid, err := ParseOID(hex); ok && err == nil {
			out[ref] = oid
		}
	}
	r.packed, r.packStamp = out, stamp
	return out
}

// Refs answers every ref under prefix, loose and packed.
func (r *Repo) Refs(prefix string) map[string]OID {
	if r.reftable {
		return r.reftableRefs(prefix)
	}
	out := map[string]OID{}
	for ref, oid := range r.packedRefs() {
		if strings.HasPrefix(ref, prefix) {
			out[ref] = oid
		}
	}
	root := filepath.Join(r.commonDir, filepath.FromSlash(prefix))
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(r.commonDir, path)
		if err != nil {
			return nil
		}
		if oid, ok := r.ref(filepath.ToSlash(rel), 0); ok {
			out[filepath.ToSlash(rel)] = oid
		}
		return nil
	})
	return out
}
