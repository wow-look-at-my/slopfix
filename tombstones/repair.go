// DefaultMaxCommentLines caps a comment block, the tier no rewording defeats.

package tombstones

import (
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/edit"
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
		e := edit.Lines(added, from, to, 0, short)
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
// hard-wrapped, so it goes back as the line the table leaves.
func rewriteParagraphs(added string, blocks []Block) ([]edit.Edit, map[int]int) {
	lines := strings.Split(added, "\n")
	rewrites := map[int]int{}
	var edits []edit.Edit
	for _, b := range blocks {
		from, to, ok := blockSpan(b, len(lines))
		if !ok {
			continue
		}
		body := paragraphBody(lines[from:to+1], b.Prefix)
		// A backtick span is a literal rather than a claim, and the table reads
		if strings.Contains(body, "`") {
			continue
		}
		short, took := english.FixN(body, english.Document)
		if took == 0 {
			continue
		}
		var kept []string
		if hasWord(short) {
			kept = []string{b.Prefix + short}
		}
		e := edit.Lines(added, from, to, 0, kept)
		edits = append(edits, e)
		rewrites[e.Start] = took
	}
	return edits, rewrites
}

// paragraphBody joins a paragraph's lines without the prefix that opens it or
// the indentation each line carries.
func paragraphBody(lines []string, prefix string) string {
	parts := make([]string, 0, len(lines))
	for i, line := range lines {
		if i == 0 {
			line = strings.TrimPrefix(line, prefix)
		}
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, " ")
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

// reflowStripped rewraps each block a strip took a line out of. The prose that
// survives then reads as a paragraph, not as a sentence with a hole.
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
		edits = append(edits, edit.Lines(text, from, to, 0, short))
	}
	return edits
}

// stripEdits deletes the dropped lines, a run of adjoining lines as a single
// edit so no edits share a line end. A block comment that reaches past the run
// keeps the opener or closer the run held, so the comment stays closed.
func stripEdits(text string, drop set.Set[int]) []edit.Edit {
	var edits []edit.Edit
	lines := strings.Split(text, "\n")
	total := len(lines)
	for line := 0; line < total; line++ {
		if !drop.Contains(line) {
			continue
		}
		end := line
		for end+1 < total && drop.Contains(end+1) {
			end++
		}
		edits = append(edits, edit.Lines(text, line, end, 0, delimitersKept(lines[line:end+1])))
		line = end
	}
	return edits
}

// delimitersKept answers the lines a strip of run leaves. The closer of a block
// comment that opened above the run, and the opener of one that closes below
// it. Each on a line of its own at the indentation it had.
func delimitersKept(run []string) []string {
	var kept []string
	open := -1
	for i, line := range run {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "--") {
			continue
		}
		opens := strings.HasPrefix(trimmed, "/*") && !strings.Contains(trimmed[2:], "*/")
		closes := strings.HasSuffix(trimmed, "*/") && !strings.Contains(trimmed, "/*")
		switch {
		case closes && open >= 0:
			open = -1
		case closes:
			kept = append(kept, indentOf(line)+"*/")
		case opens:
			open = i
		}
	}
	if open >= 0 {
		trimmed := strings.TrimSpace(run[open])
		opener := "/*"
		if strings.HasPrefix(trimmed, "/**") {
			opener = "/**"
		}
		kept = append(kept, indentOf(run[open])+opener)
	}
	return kept
}

// indentOf answers the blanks that open line.
func indentOf(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
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
	if doc || !f.Keeps(IDVolume) {
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
	rewrote := f.ApplyComments(edits)
	landed := set.New[[2]int]()
	for _, e := range rewrote.Applied {
		f.Rewrote(took[e.Start])
		landed.Add([2]int{e.Start, e.End})
	}
	missed := slices.ContainsFunc(edits, func(e edit.Edit) bool { return !landed.Contains([2]int{e.Start, e.End}) })
	switch {
	case missed && doc:
		// A rewrite that did not land leaves its wording in the text, so each wording rule it answers is reported there.
		for _, e := range edits {
			if landed.Contains([2]int{e.Start, e.End}) {
				continue
			}
			for _, h := range wordingHits(was, e, doc) {
				f.Note(h)
			}
		}
	case missed:
		// A scope can hold some lines of a block and not others. A phrase the block rewrite missed is cut a line at a time.
		now := f.Text()
		cuts, took := phraseCuts(now, AddedBlocks(path, now))
		for _, e := range f.ApplyComments(cuts).Applied {
			f.Rewrote(took[e.Start])
		}
		for _, h := range phraseHits(f.Text(), AddedBlocks(path, f.Text())) {
			f.Note(h)
		}
	}
	added := f.Text()
	if added != was {
		blocks = AddedBlocks(path, added)
	}
	// A block still over the cap is cut from its end, the way comments/length cuts.
	if capped := f.ApplyComments(capEdits(added, blocks, maxLines)); len(capped.Applied) > 0 {
		f.RemovedOnce("comment prose past the volume cap")
		added = f.Text()
		blocks = AddedBlocks(path, added)
	}

	// A dead name that a defined identifier replaces is renamed, so its sentence stays.
	if dead := DeadReferents(path, added, blocks); len(dead) > 0 {
		if renamed := f.ApplyComments(renameEdits(path, added, blocks, dead)); len(renamed.Applied) > 0 {
			f.Rewrote(len(renamed.Applied))
			added = f.Text()
			blocks = AddedBlocks(path, added)
		}
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
	var stripping []Hit
	cut := false
	for _, h := range hits {
		switch {
		case h.Strippable:
			drop.Add(h.LineNo - 1)
			stripping = append(stripping, h)
		case h.ID != IDVolume:
			// A name no whole-line strip resolves loses its sentence below.
			cut = true
		case cuttable(added, blocks, maxLines, h.LineNo):
			// The cap cut lands outside the scope this run writes, so the block is still over the cap.
			h.Fix = "Cut the run from its end down to the cap. `slopfix fix` does this."
			f.Note(h)
		}
	}
	if cut {
		defer cutReferents(f, path)
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
	gone := set.New[int]()
	for _, e := range stripped.Applied {
		for lineNo := strings.Count(added[:e.Start], "\n"); lineNo <= strings.Count(added[:max(e.Start, e.End-1)], "\n"); lineNo++ {
			gone.Add(lineNo)
		}
	}
	// A strip that did not land leaves the line in the text, so its hit is reported there.
	for _, h := range stripping {
		if !gone.Contains(h.LineNo - 1) {
			f.Note(h)
		}
	}
	// The strip leaves a paragraph with a hole in it, so what survives is rewrapped here. The write then lands finished.
	f.ApplyComments(reflowStripped(path, stripped.Text, blocksLosing(blocks, drop), len(blocks)))
}
