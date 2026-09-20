// Package rules carries the folder beside this file into the binary.
//
// An entry's own <test> drives that entry, and rulegen writes it into the
// generated table. A <test> directly under <rules> belongs to no entry: it is
// a line of prose and what the consumer as a whole must write for it, which is
// where prose reaching several entries, or reaching none, is stated.
package rules

import (
	"embed"
	"encoding/xml"
	"fmt"
	"io/fs"
	"sort"
)

//go:embed *.xml
var FS embed.FS

// Test is a line of prose and what the consumer must write for it. Out empty
// only holds the consumer to changing the prose at all, as an entry's own test
// does.
type Test struct {
	In  string `xml:"in,attr"`
	Out string `xml:"out,attr"`
}

// file reads the tests a rules document states for its consumer. An entry's
// own tests sit inside that entry, so they are not these.
type file struct {
	For   string `xml:"for,attr"`
	Tests []Test `xml:"test"`
}

// Tests is every consumer-level test the folder declares for a target, in file
// name order and then document order.
func Tests(target string) ([]Test, error) {
	paths, err := fs.Glob(FS, "*.xml")
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)

	var out []Test
	for _, path := range paths {
		raw, err := FS.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var parsed file
		if err := xml.Unmarshal(raw, &parsed); err != nil {
			return nil, fmt.Errorf("%s does not parse: %w", path, err)
		}
		if parsed.For != target {
			continue
		}
		for _, t := range parsed.Tests {
			if t.In == "" {
				return nil, fmt.Errorf("%s: a <test> under <rules> carries no in", path)
			}
		}
		out = append(out, parsed.Tests...)
	}
	return out, nil
}
