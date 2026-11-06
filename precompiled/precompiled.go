// Package precompiled hands out the regexps that slopfix builds from data.
//
// The compiler precompiles a regexp only when the pattern is a constant. A
// pattern from an XML table or a joined word list is not constant. go
// generate therefore writes every such pattern into regexps.gen.go as a
// constant call, and this package answers each request from that file.
package precompiled

import (
	"regexp"
	"slices"
	"sync"
)

var (
	mu sync.Mutex
	// built holds the generated regexps, keyed by pattern.
	built = map[string]entry{}
	// requested records each pattern a caller asked for, for the generator.
	requested = map[string]bool{}
	// missed records each pattern that compiled at run time.
	missed = map[string]bool{}
)

// entry is a regexp and the literals that every match of it holds.
type entry struct {
	re    *regexp.Regexp
	needs []string
	known bool
}

// add stores a regexp the generated file built, with its needed literals.
func add(re *regexp.Regexp, needs []string) {
	built[re.String()] = entry{re: re, needs: needs, known: true}
}

// MustCompile answers the regexp for pattern. A pattern the build did not
// precompile compiles here, and Missed reports it.
func MustCompile(pattern string) *regexp.Regexp {
	re, err := Compile(pattern)
	if err != nil {
		panic("precompiled: " + err.Error())
	}
	return re
}

// Compile is MustCompile for a pattern that can be invalid.
func Compile(pattern string) (*regexp.Regexp, error) {
	mu.Lock()
	defer mu.Unlock()
	requested[pattern] = true
	if e, ok := built[pattern]; ok {
		return e.re, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	missed[pattern] = true
	built[pattern] = entry{re: re}
	return re, nil
}

// Needs answers literals of which every match of re holds one, folded by
// Fold. Nil means that no such set is known, and a caller must run re.
func Needs(re *regexp.Regexp) []string {
	mu.Lock()
	e, ok := built[re.String()]
	mu.Unlock()
	if ok && e.known {
		return e.needs
	}
	needs := Required(re.String())
	mu.Lock()
	built[re.String()] = entry{re: re, needs: needs, known: true}
	mu.Unlock()
	return needs
}

// Requested answers every pattern a caller asked for, sorted.
func Requested() []string {
	mu.Lock()
	defer mu.Unlock()
	return sortedKeys(requested)
}

// Missed answers every pattern that compiled at run time, sorted.
func Missed() []string {
	mu.Lock()
	defer mu.Unlock()
	return sortedKeys(missed)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
