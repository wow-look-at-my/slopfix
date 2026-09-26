package tombstones

import (
	"slices"
	"strings"
	"unicode"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/english"
	"github.com/wow-look-at-my/slopfix/fixer"
)

// DefaultMaxCommentLines caps a comment block, the tier no rewording defeats.
const DefaultMaxCommentLines = 14

// Repair is the text with every strippable tombstone line deleted and every
// over-long comment reflowed. Kept carries what survives both, and a caller
// refuses on those.
type Repair struct {
	Text     string   `json:"text"`
	Changed  bool     `json:"changed"`
	Removed  []string `json:"removed,omitempty"`
	Rewrites int      `json:"rewrites,omitempty"`
	Kept     []Hit    `json:"kept,omitempty"`
	// Refused names each edit a gate would not write.
	Refused []edit.Refused `json:"-"`
	// Scope bounds, in Text, what the caller's Scope bounded.
	Scope edit.Scope `json:"-"`
}

// rewriteComments applies the english table to every comment block and reflows
// what it leaves. It answers an edit per block it rewrote, and how many
// rewrites that took.
//
// Every block goes through it, not only the ones over the cap. The count is
// keyed by where each edit starts.
func rewriteComments(added string, blocks []Block) ([]edit.Edit, map[int]int) {
	lines := strings.Split(added, "\n")
	rewrites := map[int]int{}
	var edits []edit.Edit
	for _, b := range blocks {
		from, to, ok := pureSpan(b, len(lines))
		if !ok {
			continue
		}
		short, took, rewrote := commentfix.Tighten(lines[from : to+1])
		if !rewrote {
			continue
		}
		e := edit.Rows(added, from, to, 0, short)
		edits = append(edits, e)
		rewrites[e.Start] = took
	}
	return edits, rewrites
}

// pureSpan is blockSpan for a block every line of which is comment alone. A
// line shared with code is never rewritten, because the rewrite carries the code.
func pureSpan(b Block, total int) (from, to int, ok bool) {
	from, to, ok = blockSpan(b, total)
	if !ok || len(b.Pure) != len(b.LineNos) {
		return 0, 0, false
	}
	for _, pure := range b.Pure {
		if !pure {
			return 0, 0, false
		}
	}
	return from, to, true
}

// rewriteParagraphs is rewriteComments for a document, where a block is a
// paragraph rather than a comment. A paragraph carries no marker and is never
// hard-wrapped, so it goes back as the single line the table leaves.
func rewriteParagraphs(added string, blocks []Block) ([]edit.Edit, map[int]int) {
	lines := strings.Split(added, "\n")
	rewrites := map[int]int{}
	var edits []edit.Edit
	for _, b := range blocks {
		from, to, ok := blockSpan(b, len(lines))
		if !ok {
			continue
		}
		body := strings.Join(lines[from:to+1], " ")
		// A backtick span is a literal rather than a claim, and the table reads
		if strings.Contains(body, "`") {
			continue
		}
		short, took := english.FixN(body, english.Comment)
		if took == 0 {
			continue
		}
		var kept []string
		if hasWord(short) {
			kept = []string{short}
		}
		e := edit.Rows(added, from, to, 0, kept)
		edits = append(edits, e)
		rewrites[e.Start] = took
	}
	return edits, rewrites
}

// hasWord reports whether any letter or digit survives, so a paragraph the
// table emptied is recognised as gone.
func hasWord(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// unbroken run is left alone, because splicing it would move code.
func blockSpan(b Block, total int) (from, to int, ok bool) {
	if len(b.LineNos) == 0 {
		return 0, 0, false
	}
	from, to = b.LineNos[0], b.LineNos[len(b.LineNos)-1]
	if from < 0 || to >= total || to-from+1 != len(b.LineNos) {
		return 0, 0, false
	}
	return from, to, true
}

// blocksLosing names the blocks a strip takes a line out of, by index.
func blocksLosing(blocks []Block, drop set.Set[int]) set.Set[int] {
	losing := set.New[int]()
	for i, b := range blocks {
		for _, no := range b.LineNos {
			if drop.Contains(no) {
				losing.Add(i)
				break
			}
		}
	}
	return losing
}

// reflowStripped rewraps each block a strip took a line out of, so the prose
// that survives reads as a paragraph rather than as a sentence with a hole.
// A strip that emptied a block moves every index after it, so a changed block
// count leaves the text as it is.
func reflowStripped(path, text string, losing set.Set[int], was int) []edit.Edit {
	if losing.IsEmpty() {
		return nil
	}
	lines := strings.Split(text, "\n")
	blocks := AddedBlocks(path, text)
	if len(blocks) != was {
		return nil
	}
	var edits []edit.Edit
	for i, b := range blocks {
		if !losing.Contains(i) {
			continue
		}
		from, to, ok := pureSpan(b, len(lines))
		if !ok {
			continue
		}
		short, _, rewrapped := commentfix.Tighten(lines[from : to+1])
		if !rewrapped {
			continue
		}
		edits = append(edits, edit.Rows(text, from, to, 0, short))
	}
	return edits
}

// stripEdits deletes the dropped rows, a run of adjoining rows as a single
// edit so no edits share a line end.
func stripEdits(text string, drop set.Set[int]) []edit.Edit {
	var edits []edit.Edit
	total := strings.Count(text, "\n") + 1
	for row := 0; row < total; row++ {
		if !drop.Contains(row) {
			continue
		}
		end := row
		for end+1 < total && drop.Contains(end+1) {
			end++
		}
		edits = append(edits, edit.Rows(text, row, end, 0, nil))
		row = end
	}
	return edits
}

// Fix scans the text a write adds to path and strips what it safely can.
// maxLines caps a comment block, and a cap below the floor turns it off.
func Fix(path, added string, maxLines int) Repair {
	return FixIn(path, added, maxLines, edit.Scope{})
}

// Name is the fixer's name in the registry.
const Name = "tombstones"

func init() {
	fixer.Register(fixer.Spec{
		Label:    Name,
		Families: []string{"tombstones"},
		Rules:    slices.Sorted(AllIDs().All()),
		Files:    []fixer.Kind{fixer.Source, fixer.Document},
		Place:    10,
		Repair:   repairFile,
	})
}

// FixIn is Fix with every edit held inside scope.
func FixIn(path, added string, maxLines int, scope edit.Scope) Repair {
	kind := fixer.Source
	if IsDocument(path) {
		kind = fixer.Document
	}
	f := fixer.Open(path, added, fixer.Options{Kind: kind, Scope: scope, MaxCommentLines: maxLines})
	fixer.Run(f, fixer.Named(Name))
	rep := f.Report()
	repair := Repair{Text: f.Text(), Changed: f.Text() != added, Removed: rep.Removed, Rewrites: rep.Rewrites, Refused: rep.Refused, Scope: f.Scope()}
	for _, note := range rep.Notes {
		if hit, ok := note.(Hit); ok {
			repair.Kept = append(repair.Kept, hit)
		}
	}
	return repair
}

// repairFile rewrites the tombstones in f, strips the lines a strip resolves,
// and notes every Hit it leaves for the driver to report.
func repairFile(f *fixer.File) {
	if f.Path == "" {
		return
	}
	path, doc := f.Path, f.Kind == fixer.Document
	maxLines := f.MaxCommentLines
	if doc {
		maxLines = 0
	}
	blocks := AddedBlocks(path, f.Text())
	if len(blocks) == 0 {
		return
	}

	// A block over the cap is rewritten before it is judged.
	was := f.Text()
	rewrite := rewriteComments
	if doc {
		rewrite = rewriteParagraphs
	}
	edits, took := rewrite(was, blocks)
	// The count follows the edits that landed. A refused rewrite took no words out.
	for _, e := range f.ApplyComments(edits).Applied {
		f.Rewrote(took[e.Start])
	}
	added := f.Text()
	if added != was {
		blocks = AddedBlocks(path, added)
	}

	hits := Find(blocks, maxLines)
	for _, name := range DeadReferents(path, added, blocks) {
		hits = append(hits, HitForName(blocks, name))
	}
	if doc {
		// A document line is a paragraph rather than a sentence, so
		// deleting it takes a keeper with it.
		for i := range hits {
			hits[i].Strippable = false
		}
	}

	drop := set.New[int]()
	for _, h := range hits {
		if h.Strippable {
			drop.Add(h.LineNo)
			continue
		}
		f.Note(h)
	}
	if drop.Len() == 0 {
		return
	}

	strips := stripEdits(added, drop)
	lines := strings.Split(added, "\n")
	seen := set.New[string]()
	for i := range strips {
		for _, quote := range cutLines(lines, drop, strips[i], added) {
			if !seen.Contains(quote) {
				seen.Add(quote)
				strips[i].Cut = append(strips[i].Cut, quote)
			}
		}
	}
	stripped := f.ApplyComments(strips)
	// The strip leaves a paragraph with a hole in it, so what survives is rewrapped here. The write then lands finished.
	f.ApplyComments(reflowStripped(path, stripped.Text, blocksLosing(blocks, drop), len(blocks)))
}

// cutLines quotes the dropped rows a strip edit covers.
func cutLines(lines []string, drop set.Set[int], e edit.Edit, text string) []string {
	var out []string
	row := strings.Count(text[:max(e.Start, 0)], "\n")
	if e.Start > 0 && e.Start < len(text) && text[e.Start] == '\n' {
		// A strip of the last row starts on the line end before it.
		row++
	}
	for ; row < len(lines) && drop.Contains(row); row++ {
		out = append(out, strings.TrimSpace(lines[row]))
	}
	return out
}

