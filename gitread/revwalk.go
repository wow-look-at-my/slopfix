package gitread

import (
	"bytes"
	"container/heap"
	"fmt"
	"github.com/wow-look-at-my/go-containers/set"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Commit is the part of a commit object this package reads.
type Commit struct {
	OID     OID
	Tree    OID
	Parents []OID
	// When is the committer time, in seconds since the epoch.
	When int64
}

// maxPeel stops a tag cycle.
const maxPeel = 10

// Commit answers a commit object. A commit on the shallow boundary has no
// parents, as git reads it.
func (r *Repo) Commit(oid OID) (*Commit, error) {
	if c, ok := r.commits.Load(oid); ok {
		return c.(*Commit), nil
	}
	obj, err := r.db.object(oid)
	if err != nil {
		return nil, err
	}
	if obj.kind != kindCommit {
		return nil, fmt.Errorf("gitread: %s is a %s, not a commit", oid, kindNames[obj.kind])
	}
	c, err := parseCommit(oid, obj.data)
	if err != nil {
		return nil, err
	}
	if r.shallowSet()[oid] {
		c.Parents = nil
	}
	r.commits.Store(oid, c)
	return c, nil
}

func parseCommit(oid OID, data []byte) (*Commit, error) {
	c := &Commit{OID: oid}
	for len(data) > 0 {
		line := data
		if nl := bytes.IndexByte(data, '\n'); nl >= 0 {
			line, data = data[:nl], data[nl+1:]
		} else {
			data = nil
		}
		if len(line) == 0 {
			break
		}
		var err error
		switch {
		case bytes.HasPrefix(line, []byte("tree ")):
			c.Tree, err = ParseOID(string(line[5:]))
		case bytes.HasPrefix(line, []byte("parent ")):
			var p OID
			p, err = ParseOID(string(line[7:]))
			c.Parents = append(c.Parents, p)
		case bytes.HasPrefix(line, []byte("committer ")):
			if fields := bytes.Fields(line); len(fields) >= 2 {
				c.When, _ = strconv.ParseInt(string(fields[len(fields)-2]), 10, 64)
			}
		}
		if err != nil {
			return nil, err
		}
	}
	if c.Tree.IsZero() {
		return nil, fmt.Errorf("gitread: the commit %s names no tree", oid)
	}
	return c, nil
}

// shallowSet answers the commits the shallow file names.
func (r *Repo) shallowSet() map[OID]bool {
	path := filepath.Join(r.commonDir, "shallow")
	stamp := stampOf(path)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.shallow != nil && stamp == r.shallowStp {
		return r.shallow
	}
	out := map[OID]bool{}
	data, _ := os.ReadFile(path)
	for _, line := range strings.Fields(string(data)) {
		if oid, err := ParseOID(line); err == nil {
			out[oid] = true
		}
	}
	if stamp != r.shallowStp {
		r.commits.Clear()
	}
	r.shallow, r.shallowStp = out, stamp
	return out
}

// PeelCommit follows tag objects until it reaches a commit.
func (r *Repo) PeelCommit(oid OID) (OID, error) {
	for i := 0; i < maxPeel; i++ {
		if _, ok := r.commits.Load(oid); ok {
			return oid, nil
		}
		obj, err := r.db.object(oid)
		if err != nil {
			return OID{}, err
		}
		switch obj.kind {
		case kindCommit:
			return oid, nil
		case kindTag:
			if oid, err = tagTarget(obj.data); err != nil {
				return OID{}, err
			}
		default:
			return OID{}, fmt.Errorf("gitread: %s peels to a %s, not a commit", oid, kindNames[obj.kind])
		}
	}
	return OID{}, fmt.Errorf("gitread: the tag %s does not peel to a commit", oid)
}

func tagTarget(data []byte) (OID, error) {
	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(line) == 0 {
			break
		}
		if target, ok := bytes.CutPrefix(line, []byte("object ")); ok {
			return ParseOID(string(target))
		}
	}
	return OID{}, fmt.Errorf("gitread: a tag names no object")
}

// commitQueue orders commits newest first, and in insertion order on a tie,
// as git's own walk does.
type commitQueue struct {
	items []*Commit
	seq   []int
	next  int
}

func (q *commitQueue) Len() int { return len(q.items) }
func (q *commitQueue) Less(i, j int) bool {
	if q.items[i].When != q.items[j].When {
		return q.items[i].When > q.items[j].When
	}
	return q.seq[i] < q.seq[j]
}
func (q *commitQueue) Swap(i, j int) {
	q.items[i], q.items[j] = q.items[j], q.items[i]
	q.seq[i], q.seq[j] = q.seq[j], q.seq[i]
}
func (q *commitQueue) Push(x any) {
	q.items = append(q.items, x.(*Commit))
	q.seq = append(q.seq, q.next)
	q.next++
}
func (q *commitQueue) Pop() any {
	n := len(q.items) - 1
	c := q.items[n]
	q.items, q.seq = q.items[:n], q.seq[:n]
	return c
}

// Walk visits every commit reachable from the starts, newest first, the order
// git rev-list prints. A visit that answers false stops the walk.
func (r *Repo) Walk(starts []OID, visit func(*Commit) bool) error {
	q := &commitQueue{}
	seen := set.New[OID]()
	for _, s := range starts {
		oid, err := r.PeelCommit(s)
		if err != nil {
			return err
		}
		if seen.Contains(oid) {
			continue
		}
		c, err := r.Commit(oid)
		if err != nil {
			return err
		}
		seen.Add(oid)
		heap.Push(q, c)
	}
	for q.Len() > 0 {
		c := heap.Pop(q).(*Commit)
		if !visit(c) {
			return nil
		}
		for _, p := range c.Parents {
			if seen.Contains(p) {
				continue
			}
			seen.Add(p)
			pc, err := r.Commit(p)
			if err != nil {
				// A partial history ends where its objects end.
				continue
			}
			heap.Push(q, pc)
		}
	}
	return nil
}

// RevList answers every commit reachable from oid, newest first.
func (r *Repo) RevList(oid OID) ([]OID, error) {
	var out []OID
	err := r.Walk([]OID{oid}, func(c *Commit) bool {
		out = append(out, c.OID)
		return true
	})
	return out, err
}

// FirstParent answers the first-parent chain from oid, newest first.
func (r *Repo) FirstParent(oid OID) ([]OID, error) {
	oid, err := r.PeelCommit(oid)
	if err != nil {
		return nil, err
	}
	var out []OID
	seen := set.New[OID]()
	for !seen.Contains(oid) {
		seen.Add(oid)
		out = append(out, oid)
		c, err := r.Commit(oid)
		if err != nil || len(c.Parents) == 0 {
			break
		}
		oid = c.Parents[0]
	}
	return out, nil
}

// FirstParentExcluding answers the first-parent chain from tip, newest first,
// up to the first commit an excluded commit reaches. What git rev-list
// --first-parent tip ^exclude prints. It paints the excluded history only as
// deep as the chain's dates need.
func (r *Repo) FirstParentExcluding(tip OID, exclude []OID) ([]OID, error) {
	chain, err := r.FirstParent(tip)
	if err != nil {
		return nil, err
	}
	reached := set.New[OID]()
	q := &commitQueue{}
	for _, e := range exclude {
		oid, err := r.PeelCommit(e)
		if err != nil {
			return nil, err
		}
		c, err := r.Commit(oid)
		if err != nil {
			return nil, err
		}
		if !reached.Contains(oid) {
			reached.Add(oid)
			heap.Push(q, c)
		}
	}
	var out []OID
	for _, oid := range chain {
		c, err := r.Commit(oid)
		if err != nil {
			return nil, err
		}
		for q.Len() > 0 && q.items[0].When+skewAllowance >= c.When {
			top := heap.Pop(q).(*Commit)
			for _, p := range top.Parents {
				if reached.Contains(p) {
					continue
				}
				pc, err := r.Commit(p)
				if err != nil {
					continue
				}
				reached.Add(p)
				heap.Push(q, pc)
			}
		}
		if reached.Contains(oid) {
			break
		}
		out = append(out, oid)
	}
	return out, nil
}

// Paint marks for the merge-base walk.
const (
	paintA = 1 << iota
	paintB
	paintStale
	paintResult
)

// MergeBase answers the best common ancestor of commits, as git
// merge-base prints it: the newest of the bases no other base contains.
func (r *Repo) MergeBase(a, b OID) (OID, bool, error) {
	bases, err := r.mergeBases(a, b)
	if err != nil || len(bases) == 0 {
		return OID{}, false, err
	}
	if len(bases) > 1 {
		if bases, err = r.independent(bases); err != nil {
			return OID{}, false, err
		}
	}
	best := bases[0]
	for _, c := range bases[1:] {
		if c.When > best.When {
			best = c
		}
	}
	return best.OID, true, nil
}

// mergeBases paints down from both commits until every queued commit is
// stale, and answers each commit both reach first.
func (r *Repo) mergeBases(a, b OID) ([]*Commit, error) {
	a, err := r.PeelCommit(a)
	if err != nil {
		return nil, err
	}
	b, err = r.PeelCommit(b)
	if err != nil {
		return nil, err
	}
	ca, err := r.Commit(a)
	if err != nil {
		return nil, err
	}
	if a == b {
		return []*Commit{ca}, nil
	}
	cb, err := r.Commit(b)
	if err != nil {
		return nil, err
	}
	marks := map[OID]int{a: paintA, b: paintB}
	q := &commitQueue{}
	heap.Push(q, ca)
	heap.Push(q, cb)
	var result []*Commit
	live := func() bool {
		for _, c := range q.items {
			if marks[c.OID]&paintStale == 0 {
				return true
			}
		}
		return false
	}
	for q.Len() > 0 && live() {
		c := heap.Pop(q).(*Commit)
		flags := marks[c.OID] & (paintA | paintB | paintStale)
		if flags == paintA|paintB {
			if marks[c.OID]&paintResult == 0 {
				marks[c.OID] |= paintResult
				result = append(result, c)
			}
			flags |= paintStale
		}
		for _, p := range c.Parents {
			if marks[p]&flags == flags {
				continue
			}
			pc, err := r.Commit(p)
			if err != nil {
				continue
			}
			marks[p] |= flags
			heap.Push(q, pc)
		}
	}
	// A base that a newer base reaches was painted stale after it was found.
	var out []*Commit
	for _, c := range result {
		if marks[c.OID]&paintStale == 0 {
			out = append(out, c)
		}
	}
	return out, nil
}

// independent drops each base that another base reaches.
func (r *Repo) independent(bases []*Commit) ([]*Commit, error) {
	var out []*Commit
	for i, c := range bases {
		redundant := false
		for j, d := range bases {
			if i == j {
				continue
			}
			reaches, err := r.IsAncestor(c.OID, d.OID)
			if err != nil {
				return nil, err
			}
			if reaches && c.OID != d.OID {
				redundant = true
				break
			}
		}
		if !redundant {
			out = append(out, c)
		}
	}
	return out, nil
}

// IsAncestor reports whether a is an ancestor of b, or b itself. The walk
// from b stops once every commit it queues is older than a.
func (r *Repo) IsAncestor(a, b OID) (bool, error) {
	a, err := r.PeelCommit(a)
	if err != nil {
		return false, err
	}
	b, err = r.PeelCommit(b)
	if err != nil {
		return false, err
	}
	if a == b {
		return true, nil
	}
	ca, err := r.Commit(a)
	if err != nil {
		return false, err
	}
	found := false
	err = r.Walk([]OID{b}, func(c *Commit) bool {
		if c.OID == a {
			found = true
			return false
		}
		// A commit can carry an earlier date than its parent, so the cut allows a day of skew.
		return c.When+skewAllowance >= ca.When
	})
	return found, err
}

// skewAllowance is how far a parent's date may run past its child's.
const skewAllowance = 86400
