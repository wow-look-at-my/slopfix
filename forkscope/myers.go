// myers finds the lines a shortest edit script inserts.

package forkscope

// myers finds the lines a shortest edit script inserts.
type myers struct {
	a, b    []int
	matched []bool
	vf, vb  []int
}

// insertedLines answers the line numbers of after, from one, that a shortest
// diff from before inserts.
func insertedLines(before, after string) []int {
	ids := map[string]int{}
	intern := func(lines []string) []int {
		out := make([]int, len(lines))
		for i, line := range lines {
			id, ok := ids[line]
			if !ok {
				id = len(ids)
				ids[line] = id
			}
			out[i] = id
		}
		return out
	}
	a, b := intern(splitLines(before)), intern(splitLines(after))
	size := 2*(len(a)+len(b)) + 3
	m := &myers{a: a, b: b, matched: make([]bool, len(b)), vf: make([]int, size), vb: make([]int, size)}
	m.compare(0, len(a), 0, len(b))
	var out []int
	for i, ok := range m.matched {
		if !ok {
			out = append(out, i+1)
		}
	}
	return out
}

// compare marks each line of b[bLo:bHi] that a common subsequence with
// a[aLo:aHi] keeps.
func (m *myers) compare(aLo, aHi, bLo, bHi int) {
	for aLo < aHi && bLo < bHi && m.a[aLo] == m.b[bLo] {
		m.matched[bLo] = true
		aLo++
		bLo++
	}
	for aLo < aHi && bLo < bHi && m.a[aHi-1] == m.b[bHi-1] {
		m.matched[bHi-1] = true
		aHi--
		bHi--
	}
	if aLo == aHi || bLo == bHi {
		return
	}
	// Both ends now differ, so the edit holds at least steps and the middle snake splits it into smaller problems.
	xs, ys, xe, ye := m.snake(aLo, aHi, bLo, bHi)
	for i := 0; i < xe-xs; i++ {
		m.matched[ys+i] = true
	}
	m.compare(aLo, xs, bLo, ys)
	m.compare(xe, aHi, ye, bHi)
}

// snake answers the middle snake of the edit graph of a[aLo:aHi] and
// b[bLo:bHi]. The run of matches where the forward and backward searches meet.
func (m *myers) snake(aLo, aHi, bLo, bHi int) (xs, ys, xe, ye int) {
	n, k0 := aHi-aLo, bHi-bLo
	delta := n - k0
	odd := delta%2 != 0
	maxD := (n + k0 + 1) / 2
	off := maxD + 1
	vf, vb := m.vf, m.vb
	vf[off+1], vb[off+1] = 0, 0
	for d := 0; d <= maxD; d++ {
		for k := -d; k <= d; k += 2 {
			x := vf[off+k-1] + 1
			if k == -d || k != d && vf[off+k-1] < vf[off+k+1] {
				x = vf[off+k+1]
			}
			y := x - k
			sx, sy := x, y
			for x < n && y < k0 && m.a[aLo+x] == m.b[bLo+y] {
				x++
				y++
			}
			vf[off+k] = x
			if c := delta - k; odd && c >= -(d-1) && c <= d-1 && x+vb[off+c] >= n {
				return aLo + sx, bLo + sy, aLo + x, bLo + y
			}
		}
		for k := -d; k <= d; k += 2 {
			x := vb[off+k-1] + 1
			if k == -d || k != d && vb[off+k-1] < vb[off+k+1] {
				x = vb[off+k+1]
			}
			y := x - k
			sx, sy := x, y
			for x < n && y < k0 && m.a[aHi-1-x] == m.b[bHi-1-y] {
				x++
				y++
			}
			vb[off+k] = x
			if c := delta - k; !odd && c >= -d && c <= d && x+vf[off+c] >= n {
				return aHi - x, bHi - y, aHi - sx, bHi - sy
			}
		}
	}
	// The searches always meet by maxD.
	panic("forkscope: the diff searches never met")
}

// inserted answers the lines of after that a shortest diff from before inserts.
func inserted(before, after string) *Scope {
	s := &Scope{base: func() (string, error) { return before, nil }}
	for _, n := range insertedLines(before, after) {
		s.lines.Add(n)
	}
	return s
}
