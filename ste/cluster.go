package ste

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// clusterAt answers the first run of more than NounClusterCap common nouns.
// Code, a quotation, a parenthetical and a bracket are data, so each ends a
// run. A proper noun ends one too, because the tagger also gives that tag to
// a capitalized adjective, such as "Generic".
func clusterAt(s *syntax.Sentence, off [][]int) (int, int, bool) {
	last := lastContent(s)
	noun := func(k int) bool {
		w := s.Words[k]
		return (w.Tag == "NN" || w.Tag == "NNS") && !insideAny(off, w.Start) && foldable(w.Text)
	}
	for start := 0; start <= last; start++ {
		if !noun(start) {
			continue
		}
		end := start
		for end+1 <= last && noun(end+1) {
			end++
		}
		if end-start+1 > NounClusterCap {
			return start, end, true
		}
		start = end
	}
	return -1, -1, false
}

// foldable reports a common noun the cluster repair can move. A symbol, an
// article and a name in capitals, such as REST or GitHub, are not.
func foldable(word string) bool {
	if articles.Contains(strings.ToLower(word)) {
		return false
	}
	for i, r := range word {
		switch {
		case unicode.IsLower(r):
		case unicode.IsUpper(r) && i == 0:
		case (r == '-' || r == '\'') && i > 0 && i < len(word)-1:
		default:
			return false
		}
	}
	return word != ""
}

var articles = set.Of("a", "an", "the")

// clusterRewritable reports whether the words of a cluster stand with only
// spaces between them.
func clusterRewritable(s *syntax.Sentence, source string, start, end int) bool {
	for k := start; k < end; k++ {
		if strings.TrimSpace(source[s.Words[k].End:s.Words[k+1].Start]) != "" {
			return false
		}
	}
	return true
}

// rewriteCluster breaks a run of nouns apart: "the gate file system cache
// lookup" becomes "the lookup of the gate file system cache". A longer run
// keeps taking its last noun as the head until at most NounClusterCap nouns
// stand together, all in one rewrite.
func rewriteCluster(s *syntax.Sentence, source string, off [][]int) (string, bool) {
	start, end, ok := clusterAt(s, off)
	if !ok || !clusterRewritable(s, source, start, end) {
		return "", false
	}
	var out strings.Builder
	last := end
	// The head names the thing, and the nouns in front of it describe it.
	for last-start+1 > NounClusterCap {
		out.WriteString(source[s.Words[last].Start:s.Words[last].End])
		out.WriteString(" of the ")
		last--
	}
	out.WriteString(source[s.Words[start].Start:s.Words[last].End])
	return splice(source, s, start, end, out.String()), true
}
