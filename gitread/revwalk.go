package gitread

import (
	"bytes"
	"github.com/wow-look-at-my/go-containers/set"
	"strconv"
	"time"
)

// RevList answers every commit reachable from oid, including oid.
func (r *Repo) RevList(oid OID) ([]OID, error) {
	start, err := r.PeelCommit(oid)
	if err != nil {
		return nil, err
	}
	var out []OID
	seen := set.New[OID]()
	stack := []OID{start}
	for len(stack) > 0 {
		oid := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen.Contains(oid) {
			continue
		}
		seen.Add(oid)
		out = append(out, oid)
		c, err := r.Commit(oid)
		if err != nil {
			continue
		}
		for _, p := range c.Parents {
			if !seen.Contains(p) {
				stack = append(stack, p)
			}
		}
	}
	return out, nil
}

// FirstParent answers the first-parent chain from oid, newest first.
func (r *Repo) FirstParent(oid OID) ([]OID, error) {
	oid, err := r.PeelCommit(oid)
	if err != nil {
		return nil, err
	}
	var out []OID
	for i := 0; i < 1_000_000; i++ {
		out = append(out, oid)
		c, err := r.Commit(oid)
		if err != nil || len(c.Parents) == 0 {
			break
		}
		oid = c.Parents[0]
	}
	return out, nil
}

// MergeBase answers the best common ancestor of commits.
func (r *Repo) MergeBase(a, b OID) (OID, bool, error) {
	a, err := r.PeelCommit(a)
	if err != nil {
		return OID{}, false, err
	}
	b, err = r.PeelCommit(b)
	if err != nil {
		return OID{}, false, err
	}
	ancA, err := r.ancestorSet(a)
	if err != nil {
		return OID{}, false, err
	}
	ancB, err := r.ancestorSet(b)
	if err != nil {
		return OID{}, false, err
	}
	var common []OID
	for oid := range ancA {
		if ancB[oid] {
			common = append(common, oid)
		}
	}
	if len(common) == 0 {
		return OID{}, false, nil
	}
	best := OID{}
	var bestTime time.Time
	for _, c := range common {
		maximal := true
		for _, d := range common {
			if c == d {
				continue
			}
			isAnc, err := r.isAncestorWithin(c, d, ancA)
			if err != nil {
				return OID{}, false, err
			}
			if isAnc {
				maximal = false
				break
			}
		}
		if !maximal {
			continue
		}
		when, _ := r.commitTime(c)
		if best.IsZero() || when.After(bestTime) {
			best, bestTime = c, when
		}
	}
	return best, true, nil
}

// ancestorSet answers every commit reachable from start.
func (r *Repo) ancestorSet(start OID) (map[OID]bool, error) {
	out := map[OID]bool{}
	stack := []OID{start}
	for len(stack) > 0 {
		oid := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if out[oid] {
			continue
		}
		out[oid] = true
		c, err := r.Commit(oid)
		if err != nil {
			continue
		}
		stack = append(stack, c.Parents...)
	}
	return out, nil
}

// isAncestorWithin reports whether a is reachable from b, searching only within limit.
func (r *Repo) isAncestorWithin(a, b OID, limit map[OID]bool) (bool, error) {
	if a == b {
		return true, nil
	}
	seen := set.New[OID]()
	stack := []OID{b}
	for len(stack) > 0 {
		oid := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen.Contains(oid) {
			continue
		}
		seen.Add(oid)
		c, err := r.Commit(oid)
		if err != nil {
			continue
		}
		for _, p := range c.Parents {
			if p == a {
				return true, nil
			}
			if limit[p] && !seen.Contains(p) {
				stack = append(stack, p)
			}
		}
	}
	return false, nil
}

// IsAncestor reports whether a is an ancestor of b (or equal to b).
func (r *Repo) IsAncestor(a, b OID) (bool, error) {
	a, err := r.PeelCommit(a)
	if err != nil {
		return false, err
	}
	b, err = r.PeelCommit(b)
	if err != nil {
		return false, err
	}
	set, err := r.ancestorSet(b)
	if err != nil {
		return false, err
	}
	return set[a], nil
}

// commitTime answers a commit's committer time.
func (r *Repo) commitTime(oid OID) (time.Time, error) {
	obj, err := r.object(oid)
	if err != nil {
		return time.Time{}, err
	}
	for _, raw := range bytes.Split(obj.data, []byte("\n")) {
		if len(raw) == 0 {
			break
		}
		if bytes.HasPrefix(raw, []byte("committer ")) {
			fields := bytes.Fields(raw)
			if len(fields) >= 2 {
				secs, err := strconv.ParseInt(string(fields[len(fields)-2]), 10, 64)
				if err == nil {
					return time.Unix(secs, 0), nil
				}
			}
		}
	}
	return time.Time{}, nil
}
