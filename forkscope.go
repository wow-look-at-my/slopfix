// forkscope.go keeps a fork's run to the lines the fork wrote itself.
//
// The forkscope package finds those lines. A tree run skips every file the fork
// never touched. In a file it did touch, a repair lands only on the fork's
// lines, and a finding counts only when it names one.
package slopfix

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/forkscope"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// FileScope answers the lines of content that the fork wrote, for content
// headed for path, or nil when path is in no fork. A fork whose base cannot be
// read is an error.
func FileScope(r forkscope.Resolver, path, content string) (*forkscope.Scope, error) {
	base, err := BaseOf(r, path)
	if err != nil || base == nil {
		return nil, err
	}
	return base.File(path, content)
}

// DefaultGitHubAPI is the API root when GITHUB_API_URL is unset.
const DefaultGitHubAPI = "https://api.github.com"

// githubGet reads path from GITHUB_API_URL, with GITHUB_TOKEN as the bearer
// when it is set. It answers the status and the body. A transport failure is
// the error.
func githubGet(getenv func(string) string, path string) (int, []byte, error) {
	api := strings.TrimRight(getenv("GITHUB_API_URL"), "/")
	if api == "" {
		api = DefaultGitHubAPI
	}
	url := api + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("GET %s: %w", url, err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, nil, fmt.Errorf("GET %s: %w", url, err)
	}
	return resp.StatusCode, body, nil
}

// githubGetJSON decodes path into out.
func githubGetJSON(getenv func(string) string, path string, out any) error {
	status, body, err := githubGet(getenv, path)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("GET %s: %d %s: %s", path, status, http.StatusText(status), strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	return nil
}

// BaseOf answers the fork base of the work tree path is in, or nil when it is
// in no fork. A path that does not exist yet is in the work tree of its
// nearest directory that does.
func BaseOf(r forkscope.Resolver, path string) (*forkscope.Base, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return r.Base(existingDir(filepath.Dir(abs)))
}

// existingDir answers dir, or its nearest parent that exists.
func existingDir(dir string) string {
	for {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}

// FixFileIn is FixFileWith in a fork: a file the fork never touched is neither
// read nor written, and a repair lands only on the lines the fork wrote.
func FixFileIn(r forkscope.Resolver, path string, req Request) (Repair, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Repair{}, err
	}
	scope, err := FileScope(r, path, string(content))
	if err != nil {
		return Repair{}, err
	}
	if scope != nil && scope.Empty() {
		return Repair{Text: string(content)}, nil
	}
	req.Owned = scope
	return FixFileWith(path, req)
}

// within keeps a repair of req to the lines req.Owned names. A change to any
// other line goes back as req.Content had it. The findings and the tombstones
// then come from the text that lands, and only those on owned lines stay.
func within(req Request, repair Repair) Repair {
	scope := req.Owned
	if scope == nil || scope.All() {
		return repair
	}
	text := forkscope.Keep(req.Content, repair.Text, scope)
	if text != repair.Text {
		if repair.Scope.Bounded && repair.Scope.Start <= repair.Scope.End {
			repair.Scope.End = len(text) - (len(repair.Text) - repair.Scope.End)
		}
		again := req
		again.Content, again.Owned = text, nil
		again.Scope = edit.Nowhere()
		landed := Fix(again)
		repair.Findings, repair.Kept = landed.Findings, landed.Kept
		repair.Removed = landedRemovals(req.Content, text, repair.Removed)
		repair.Text, repair.Changed = text, text != req.Content
		if !repair.Changed {
			repair.Rewrites = 0
		}
	}
	repair = alone(req, repair)
	repair, owned := giveBack(req, repair)
	if owned == nil {
		owned = forkscope.Carry(req.Content, repair.Text, scope)
	}
	repair.Findings = ownedFindings(repair.Findings, owned)
	repair.Kept = ownedHits(repair.Kept, owned)
	return repair
}

// blockRules judge a run of lines whole.
var blockRules = set.Of(workflow.IDCommentBlock, tombstones.IDVolume)

// alone runs each block rule still reporting on a fork line by itself, and
// keeps what lands on the fork's lines. Run with every rule, a repair of an
// upstream line beside the run joins the run's change in one diff hunk, and
// the whole hunk goes back.
func alone(req Request, repair Repair) Repair {
	owned := forkscope.Carry(req.Content, repair.Text, req.Owned)
	ids := set.New[string]()
	for _, f := range repair.Findings {
		if blockRules.Contains(f.ID) && owned.Holds(f.Line, max(f.Line, f.EndLine)) {
			ids.Add(f.ID)
		}
	}
	for _, h := range repair.Kept {
		if blockRules.Contains(h.ID) && h.LineNo > 0 && owned.Holds(h.LineNo, max(h.LineNo, h.EndLineNo)) {
			ids.Add(h.ID)
		}
	}
	for _, id := range slices.Sorted(ids.All()) {
		one := req
		one.Content, one.Scope, one.Owned, one.IDs = repair.Text, repair.Scope, nil, []string{id}
		fixed := fixAll(one)
		owned = forkscope.Carry(req.Content, repair.Text, req.Owned)
		text := forkscope.Keep(repair.Text, fixed.Text, owned)
		if text == repair.Text {
			continue
		}
		again := req
		again.Content, again.Owned, again.Scope = text, nil, edit.Nowhere()
		landed := Fix(again)
		repair.Findings, repair.Kept = landed.Findings, landed.Kept
		repair.Removed = append(repair.Removed, landedRemovals(repair.Text, text, fixed.Removed)...)
		if repair.Scope.Bounded && repair.Scope.Start <= repair.Scope.End {
			repair.Scope.End = len(text) - (len(repair.Text) - repair.Scope.End)
		}
		repair.Text, repair.Changed = text, text != req.Content
	}
	return repair
}

// giveBack repairs each run a block rule still reports on a line the fork
// wrote. Each change the fork made inside that run goes back to what the base
// has, through the gate that proves it changed only comment. The run is then
// the base's own, and the fork wrote none of it. It answers the lines the fork
// wrote of the text it gave back, measured from the base, or nil when it gave
// nothing back.
func giveBack(req Request, repair Repair) (Repair, *forkscope.Scope) {
	owned := forkscope.Carry(req.Content, repair.Text, req.Owned)
	type run struct{ first, last int }
	var runs []run
	for _, f := range repair.Findings {
		if blockRules.Contains(f.ID) && owned.Holds(f.Line, max(f.Line, f.EndLine)) {
			runs = append(runs, run{f.Line, max(f.Line, f.EndLine)})
		}
	}
	for _, h := range repair.Kept {
		if blockRules.Contains(h.ID) && h.LineNo > 0 && owned.Holds(h.LineNo, max(h.LineNo, h.EndLineNo)) {
			runs = append(runs, run{h.LineNo, max(h.LineNo, h.EndLineNo)})
		}
	}
	if len(runs) == 0 {
		return repair, nil
	}
	// With no base to write back, the finding stays and the report names it.
	base, err := req.Owned.Base()
	if err != nil {
		return repair, nil
	}
	var edits []edit.Edit
	seen := set.New[int]()
	for _, r := range runs {
		for _, e := range forkscope.GiveBack(base, repair.Text, r.first, r.last) {
			if !seen.Contains(e.Start) {
				seen.Add(e.Start)
				edits = append(edits, e)
			}
		}
	}
	at := req
	at.Content, at.Scope, at.Owned = repair.Text, repair.Scope, nil
	f := openFile(at, kindOf(req.Path, repair.Text))
	res := f.ApplyComments(edits)
	repair.refuse(res.Refused)
	if len(res.Applied) == 0 {
		return repair, nil
	}
	again := at
	again.Content, again.Scope = f.Text(), edit.Nowhere()
	landed := Fix(again)
	repair.Findings, repair.Kept = landed.Findings, landed.Kept
	repair.Removed = append(repair.Removed, res.Cuts()...)
	repair.Text, repair.Changed, repair.Scope = f.Text(), f.Text() != req.Content, f.Scope()
	return repair, forkscope.Changed(base, repair.Text)
}

// landedRemovals keeps each removal that text, the repair as it lands, made.
func landedRemovals(before, text string, removed []string) []string {
	cut := forkscope.Removed(before, text)
	var out []string
	for _, r := range removed {
		if strings.Contains(cut, strings.TrimSpace(r)) {
			out = append(out, r)
		}
	}
	return out
}

func ownedFindings(findings []ste.Finding, owned *forkscope.Scope) []ste.Finding {
	var out []ste.Finding
	for _, f := range findings {
		if owned.Holds(f.Line, f.EndLine) {
			out = append(out, f)
		}
	}
	return out
}

func ownedHits(hits []tombstones.Hit, owned *forkscope.Scope) []tombstones.Hit {
	var out []tombstones.Hit
	for _, h := range hits {
		if owned.Holds(h.LineNo, max(h.LineNo, h.EndLineNo)) {
			out = append(out, h)
		}
	}
	return out
}

// Within keeps what a tree run from root found on lines the fork wrote. A
// repository rule names its file relative to root. Every fixture expectation
// stays, because a fixture is the repository's own test.
func (t TreeRepair) Within(own *forkscope.Lines, root string) TreeRepair {
	if own == nil {
		return t
	}
	findings := t.Findings[:0:0]
	for _, f := range t.Findings {
		path := f.Path
		if RepoIDs.Contains(f.ID) && !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		if own.Holds(path, f.Line, f.EndLine) {
			findings = append(findings, f)
		}
	}
	kept := t.Kept[:0:0]
	for _, k := range t.Kept {
		if own.Holds(k.Path, k.LineNo, max(k.LineNo, k.EndLineNo)) {
			kept = append(kept, k)
		}
	}
	t.Findings, t.Kept = findings, kept
	return t
}
