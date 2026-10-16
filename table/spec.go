package table

import (
	"fmt"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/xmlcheck"
)

// entryChildren are the elements a rules file holds at its top.
var entryChildren = []string{"test", "detect", "drop", "rewrite", "pattern", "flag", "class", "list", "rephrase", "normalize"}

// SpecFor is what the rules files of one consumer are held to. Matching ignores
// case unless a file or an entry says case="sensitive". ste reads a rewrite
// through a lowercased lookup, so its rewrites take no case attribute.
func SpecFor(target string) xmlcheck.Spec {
	one := []string{"test"}
	elems := map[string]xmlcheck.Elem{
		"rules": {Required: []string{"for"}, Case: true, Children: entryChildren},
		"test":  {Required: []string{"in"}, Attrs: []string{"out"}},
		"detect": {
			Required: []string{"in", "substrate"}, Attrs: []string{"why"}, Children: []string{"found"},
			Keys: []xmlcheck.Key{{Attrs: []string{"in"}, Exact: []string{"substrate"}}},
		},
		"found": {Required: []string{"text"}},
		"drop": {
			Required: []string{"word", "id"}, Attrs: []string{"where"}, Children: one, Case: true,
			Keys: []xmlcheck.Key{{Attrs: []string{"word"}, Exact: []string{"where"}}},
		},
		"rewrite": {
			Required: []string{"from", "to", "id"}, Attrs: []string{"where"}, Children: one, Case: target != "ste",
			Keys: []xmlcheck.Key{{Attrs: []string{"from"}, Exact: []string{"where"}}},
		},
		"pattern": {
			Required: []string{"match", "id"}, Attrs: []string{"to", "where"}, Children: one, Case: true,
			Keys: []xmlcheck.Key{{Attrs: []string{"match"}, Exact: []string{"where"}}},
		},
		"flag": {
			Required: []string{"phrase", "say", "id"}, Attrs: []string{"where"}, Children: one, Case: true,
			Keys: []xmlcheck.Key{{Attrs: []string{"phrase"}, Exact: []string{"where"}}},
		},
		"class": {
			Required: []string{"name"}, Attrs: []string{"words", "suffix", "digits", "open", "except"},
			Enum: map[string][]string{"digits": {"true", "false"}, "open": {"true", "false"}},
			Keys: []xmlcheck.Key{{Attrs: []string{"name"}}},
		},
		"list": {Required: []string{"name"}, Text: true},
		"rephrase": {
			Required: []string{"match", "id"}, Attrs: []string{"to", "where"}, Children: one, Case: true,
			Keys: []xmlcheck.Key{{Attrs: []string{"match"}, Exact: []string{"where"}}},
		},
		"normalize": {
			Required: []string{"from", "to", "id"}, Children: one,
			Keys: []xmlcheck.Key{{Attrs: []string{"from"}}},
		},
	}
	spec := xmlcheck.Spec{
		Root:  "rules",
		Elems: elems,
		Test:  xmlcheck.TestDef{Elem: "test", In: "in", Out: []string{"out"}},
		Entry: checkEntry,
		Fires: entryFires,
	}
	if target == "numbers" {
		spec.Shadows = []xmlcheck.Shadow{{Elem: "rewrite", Attr: "from"}, {Elem: "drop", Attr: "word"}}
	}
	return spec
}

// checkEntry answers what makes an entry unusable once its attributes are sound.
func checkEntry(n *xmlcheck.Node, sens bool) []string {
	switch n.Name {
	case "pattern":
		return xmlcheck.PatternProblems(n.Attrs["match"], n.Attrs["to"], sens)
	case "rewrite":
		return xmlcheck.RewriteProblems(n.Attrs["from"], n.Attrs["to"], sens)
	case "rephrase":
		return CaptureProblems(n.Attrs["match"], n.Attrs["to"])
	case "normalize":
		return normalizeProblems(n.Attrs["from"], n.Attrs["to"])
	}
	return nil
}

// normalizeProblems answers what makes a spelling entry pointless. The repair
// writes back by word, so a replacement with a space is refused.
func normalizeProblems(from, to string) []string {
	var out []string
	if strings.ContainsAny(to, " \t") {
		out = append(out, fmt.Sprintf("the replacement %q holds a space, and a normalized token is a single word", to))
	}
	if strings.EqualFold(from, to) {
		out = append(out, fmt.Sprintf("the spelling %q is normalized to itself", from))
	}
	return out
}

// CaptureProblems answers what makes a class match unusable: it does not parse,
// or its replacement writes a name that no term of the match captures.
func CaptureProblems(match, to string) []string {
	terms, err := ParseMatch(match)
	if err != nil {
		return []string{fmt.Sprintf("the match does not parse: %v", err)}
	}
	captured := set.Of[string]()
	for _, term := range terms.Terms {
		if term.Name != "" {
			captured.Add(term.Name)
		}
	}
	var out []string
	for rest := to; ; {
		open := strings.IndexByte(rest, '{')
		if open < 0 {
			return out
		}
		end := strings.IndexByte(rest[open:], '}')
		if end < 0 {
			return out
		}
		if name := rest[open+1 : open+end]; !captured.Contains(name) {
			out = append(out, fmt.Sprintf("the replacement writes {%s}, which no term of the match captures", name))
		}
		rest = rest[open+end+1:]
	}
}

// entryFires answers whether an entry matches the input of one of its tests.
func entryFires(n *xmlcheck.Node, sens bool, in string) bool {
	switch n.Name {
	case "drop":
		return xmlcheck.HasPhrase(in, n.Attrs["word"], sens)
	case "rewrite", "normalize":
		return xmlcheck.HasPhrase(in, n.Attrs["from"], sens)
	case "flag":
		return xmlcheck.HasPhrase(in, n.Attrs["phrase"], sens)
	case "pattern":
		re, err := xmlcheck.Compile(n.Attrs["match"], sens)
		return err != nil || re.MatchString(in)
	}
	return true
}
