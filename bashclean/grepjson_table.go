// The grep programs and their flags come from grepjson.xml. A flag outside its
// set stops the rewrite.
package bashclean

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/wow-look-at-my/slopfix/table"
	"mvdan.cc/sh/v3/syntax"
)

//go:embed grepjson.xml
var grepJSONXML []byte

type regexSyntax int

const (
	reBasic regexSyntax = iota
	reExtended
	rePerl
	reFixed
)

var syntaxNames = map[string]regexSyntax{
	"basic": reBasic, "extended": reExtended, "perl": rePerl, "fixed": reFixed,
}

type xmlGrepTest struct {
	Cmd    string `xml:"cmd,attr"`
	Keep   bool   `xml:"keep,attr"`
	Output string `xml:",chardata"`
}

type xmlGrepFlag struct {
	Short  string        `xml:"short,attr"`
	Long   string        `xml:"long,attr"`
	Effect string        `xml:"effect,attr"`
	Value  string        `xml:"value,attr"`
	Tests  []xmlGrepTest `xml:"test"`
}

type xmlGrepJSON struct {
	Tests    []xmlGrepTest `xml:"test"`
	Fixtures []struct {
		Name string `xml:"name,attr"`
		Kind string `xml:"kind,attr"`
		Body string `xml:",chardata"`
	} `xml:"fixture"`
	Flags []struct {
		Name  string        `xml:"name,attr"`
		Flags []xmlGrepFlag `xml:"flag"`
	} `xml:"flags"`
	Programs []struct {
		Name   string        `xml:"name,attr"`
		Flags  string        `xml:"flags,attr"`
		Syntax string        `xml:"syntax,attr"`
		Tests  []xmlGrepTest `xml:"test"`
	} `xml:"program"`
	Translations []struct {
		Syntax string  `xml:"syntax,attr"`
		From   string  `xml:"from,attr"`
		To     *string `xml:"to,attr"`
	} `xml:"translate"`
}

type grepFlag struct {
	effect   string
	anyValue bool
}

type grepFlagSet struct {
	short map[byte]grepFlag
	long  map[string]grepFlag
}

type grepProgramSpec struct {
	flags  grepFlagSet
	syntax regexSyntax
}

var grepEffects = map[string]bool{
	"pattern": true, "ignore-case": true, "case-sensitive": true, "smart-case": true,
	"invert": true, "basic": true, "extended": true, "fixed": true, "perl": true,
	"word": true, "line": true, "with-filename": true, "no-filename": true,
	"line-number": true, "no-line-number": true, "count": true, "none": true,
}

var grepPrograms = mustLoadGrepPrograms(grepJSONXML)

func parseGrepJSONXML(raw []byte) (xmlGrepJSON, error) {
	var doc xmlGrepJSON
	err := xml.Unmarshal(table.Readable(raw), &doc)
	return doc, err
}

// mustLoadGrepPrograms panics on a malformed table, so a broken file stops the
// binary at its first start instead of rewriting nothing.
func mustLoadGrepPrograms(raw []byte) map[string]grepProgramSpec {
	programs, err := loadGrepPrograms(raw)
	if err != nil {
		panic("grepjson.xml: " + err.Error())
	}
	return programs
}

func loadGrepPrograms(raw []byte) (map[string]grepProgramSpec, error) {
	doc, err := parseGrepJSONXML(raw)
	if err != nil {
		return nil, err
	}
	sets := map[string]grepFlagSet{}
	for _, fs := range doc.Flags {
		set := grepFlagSet{short: map[byte]grepFlag{}, long: map[string]grepFlag{}}
		for _, f := range fs.Flags {
			if !grepEffects[f.Effect] {
				return nil, fmt.Errorf("flag set %q: unknown effect %q", fs.Name, f.Effect)
			}
			flag := grepFlag{effect: f.Effect, anyValue: f.Value == "optional"}
			if len(f.Short) > 1 || (f.Short == "" && f.Long == "") {
				return nil, fmt.Errorf("flag set %q: a flag needs one short letter or a long spelling", fs.Name)
			}
			if f.Short != "" {
				set.short[f.Short[0]] = flag
			}
			if f.Long != "" {
				set.long[f.Long] = flag
			}
		}
		sets[fs.Name] = set
	}
	programs := map[string]grepProgramSpec{}
	for _, p := range doc.Programs {
		set, ok := sets[p.Flags]
		if !ok {
			return nil, fmt.Errorf("program %q names flag set %q, which the file does not define", p.Name, p.Flags)
		}
		syn, ok := syntaxNames[p.Syntax]
		if !ok {
			return nil, fmt.Errorf("program %q: unknown syntax %q", p.Name, p.Syntax)
		}
		programs[p.Name] = grepProgramSpec{flags: set, syntax: syn}
	}
	return programs, nil
}

// apply sets what an effect names on the call.
func (g *grepCall) apply(effect string) {
	switch effect {
	case "ignore-case":
		g.icase, g.smart = true, false
	case "case-sensitive":
		g.icase, g.smart = false, false
	case "smart-case":
		g.icase, g.smart = false, true
	case "invert":
		g.invert = true
	case "basic":
		g.syntax = reBasic
	case "extended":
		g.syntax = reExtended
	case "fixed":
		g.syntax = reFixed
	case "perl":
		g.syntax = rePerl
	case "word":
		g.word = true
	case "line":
		g.whole = true
	case "with-filename":
		g.names = 1
	case "no-filename":
		g.names = -1
	case "line-number":
		g.numbers = true
	case "no-line-number":
		g.numbers = false
	case "count":
		g.count = true
	}
}

// parseGrep reads the arguments after the program word. Every word it keeps
// must be literal, because the pattern becomes part of the jq regex.
func parseGrep(spec grepProgramSpec, args []*syntax.Word) (grepCall, bool) {
	g := grepCall{syntax: spec.syntax}
	var positional []*syntax.Word
	explicit, done := false, false
	for i := 0; i < len(args); i++ {
		w := args[i]
		s, ok := literal(w)
		if done || !ok || s == "-" || !strings.HasPrefix(s, "-") {
			positional = append(positional, w)
			continue
		}
		switch {
		case s == "--":
			done = true
		case strings.HasPrefix(s, "--"):
			name, value, hasValue := strings.Cut(s, "=")
			f, known := spec.flags.long[name]
			switch {
			case !known:
				return g, false
			case f.effect == "pattern":
				if !hasValue {
					if value, ok = nextLiteral(args, &i); !ok {
						return g, false
					}
				}
				g.patterns = append(g.patterns, value)
				explicit = true
			case hasValue && !f.anyValue:
				return g, false
			default:
				g.apply(f.effect)
			}
		default:
			if !g.shortCluster(spec.flags, s[1:], args, &i, &explicit) {
				return g, false
			}
		}
	}
	if !explicit {
		if len(positional) == 0 {
			return g, false
		}
		p, ok := literal(positional[0])
		if !ok {
			return g, false
		}
		g.patterns = []string{p}
		positional = positional[1:]
	}
	g.files = positional
	return g, true
}

func nextLiteral(args []*syntax.Word, i *int) (string, bool) {
	if *i+1 >= len(args) {
		return "", false
	}
	v, ok := literal(args[*i+1])
	if ok {
		*i++
	}
	return v, ok
}

// shortCluster reads a cluster such as `-inE`. A pattern flag takes the rest of
// the cluster, or the next word when the rest is empty.
func (g *grepCall) shortCluster(set grepFlagSet, cluster string, args []*syntax.Word, i *int, explicit *bool) bool {
	for j := 0; j < len(cluster); j++ {
		f, known := set.short[cluster[j]]
		if !known {
			return false
		}
		if f.effect != "pattern" {
			g.apply(f.effect)
			continue
		}
		p := cluster[j+1:]
		if p == "" {
			var ok bool
			if p, ok = nextLiteral(args, i); !ok {
				return false
			}
		}
		g.patterns = append(g.patterns, p)
		*explicit = true
		return true
	}
	return true
}
