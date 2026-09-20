// load.go reads the rules folder. A file declares which consumer it belongs to
// with the `for` attribute on its root, and entries are collected in file name
// order and then document order: a longer phrase that must beat a shorter
// phrase sits above it.
package table

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
)

// Load reads every XML in fsys and returns the entries declaring this target.
func Load(fsys fs.FS, target string) (Table, error) {
	paths, err := fs.Glob(fsys, "*.xml")
	if err != nil {
		return Table{}, err
	}
	sort.Strings(paths)

	out := Table{For: target}
	for _, path := range paths {
		raw, err := fs.ReadFile(fsys, path)
		if err != nil {
			return Table{}, err
		}
		var parsed Table
		if err := xml.Unmarshal(raw, &parsed); err != nil {
			return Table{}, fmt.Errorf("%s does not parse: %w", path, err)
		}
		if parsed.For == "" {
			return Table{}, fmt.Errorf("%s: the root element needs a for= naming its consumer", path)
		}
		if parsed.For != target {
			continue
		}
		for i := range parsed.Patterns {
			p := &parsed.Patterns[i]
			if p.re, err = regexp.Compile(p.Match); err != nil {
				return Table{}, fmt.Errorf("%s: <pattern match=%q>: %w", path, p.Match, err)
			}
		}
		out.Drops = append(out.Drops, parsed.Drops...)
		out.Rewrites = append(out.Rewrites, parsed.Rewrites...)
		out.Patterns = append(out.Patterns, parsed.Patterns...)
		out.Flags = append(out.Flags, parsed.Flags...)
		out.Cases = append(out.Cases, parsed.Cases...)
	}
	if out.empty() {
		return Table{}, fmt.Errorf("no entry in the rules folder carries for=%q", target)
	}
	return out, nil
}

// MustLoad is Load for a package-level variable. A rules folder that does not
// load is a broken binary, not a condition a caller can do anything about.
func MustLoad(fsys fs.FS, target string) Table {
	loaded, err := Load(fsys, target)
	if err != nil {
		panic(err)
	}
	return loaded
}

func (t Table) empty() bool {
	return len(t.Drops)+len(t.Rewrites)+len(t.Patterns)+len(t.Flags)+len(t.Cases) == 0
}
