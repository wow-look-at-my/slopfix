package slopfix

import (
	"slices"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/askproperly"
	"github.com/wow-look-at-my/slopfix/blamelanguage"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/counts"
	"github.com/wow-look-at-my/slopfix/english"
	"github.com/wow-look-at-my/slopfix/laziness"
	"github.com/wow-look-at-my/slopfix/pins"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// registerFile registers an error rule a file's text answers: its detection and
// its repair both read one file, and the case is that file.
func registerFile(id string, category Rule, path, text string) {
	RegisterRule(RuleSpec{
		ID:       id,
		Category: category,
		Detect:   detectContent(id),
		Autofix:  repairContent(id),
		Cases:    []RuleCase{{Name: id, Path: path, Text: text}},
	})
}

// registerTree registers a repository rule the tree rules answer: its detection
// and its repair both read a repository. The case is the files under it.
func registerTree(id string, category Rule, repair bool, reason string, files map[string]string) {
	rule := RuleSpec{
		ID:       id,
		Category: category,
		Detect:   detectTree(id),
		Cases:    []RuleCase{{Name: id, Files: files}},
	}
	if repair {
		rule.Autofix = repairTree(id)
	} else {
		rule.ReportOnly = reason
	}
	RegisterRule(rule)
}

// registerExempt registers a rule no rewrite answers, with the reason it
// reports alone. It still carries a detection and a case, so the harness proves
// the rule fires even though nothing repairs it.
func registerExempt(id string, category Rule, reason string, detect func(RuleCase) []ste.Finding, cases ...RuleCase) {
	RegisterRule(RuleSpec{
		ID:         id,
		Category:   category,
		Detect:     detect,
		Cases:      cases,
		ReportOnly: reason,
	})
}

// init builds the whole registry. Every error rule is registered with a
// detection, a repair and a case. The report-only rules are declared with the
// reason no rewrite answers them. The english table's identified patterns
// register themselves from their own worked examples.
func init() {
	registerFile(IDHardWrap, RuleWrap, "x.md", "The gate reads the file\nand writes the result.\n")
	registerFile(IDLongBlock, RuleWrap, "x.md", "- "+repeatPhrase("The gate reads the file. ", 120)+"\n")

	registerFile(ste.IDContraction, RuleSTE, "x.md", "The tool doesn't write the result.\n")
	registerFile(ste.IDModal, RuleSTE, "x.md", "The tool should write the result.\n")
	registerFile(ste.IDSemicolon, RuleSTE, "x.md", "The gate reads the file; the tool writes the result.\n")
	registerFile(ste.IDSentenceCap, RuleSTE, "x.md", "This sentence carries far more words than any reader can hold in mind at one time and it keeps going well past the cap of the rule today.\n")
	registerFile(ste.IDCommaSplice, RuleSTE, "x.md", "The gate reads the file, the tool writes the result.\n")
	registerFile(ste.IDStaleCount, RuleSTE, "x.md", "This project has three rules.\n")
	registerFile(ste.IDPostdeterminer, RuleSTE, "x.md", "All the three rules apply here.\n")

	registerFile(english.IDCommaNever, RuleEnglish, "x.md", "The gate reads the file, never the tool.\n")

	RegisterRule(RuleSpec{
		ID:       commentfix.ID,
		Category: RuleComments,
		Detect:   detectContent(commentfix.ID),
		Autofix:  repairContent(commentfix.ID),
		Cases: []RuleCase{
			{Name: commentfix.ID, Path: "main.go", Text: "package main\n\n// There are 3 modes today.\nfunc main() {}\n"},
			{Name: commentfix.ID + "/doc", Path: "stop.rs", Text: commentNumberDocCase(), Unchanged: true},
		},
	})
	registerFile(commentfix.IDLength, RuleComments, "main.go", "package main\n\n"+overlongComment()+"func main() {}\n")
	registerFile(commentfix.IDTail, RuleComments, "main.go", "package main\n\n// The loop reads each value because\nfunc main() {}\n")

	registerFile(counts.IDSection, RuleCounts, "x.md", "# 9. Owning the renderer\n\nSee §9 for the detail.\n")
	RegisterRule(RuleSpec{
		ID:       counts.ID,
		Category: RuleCounts,
		Detect:   detectInventory,
		Autofix:  repairContent(counts.ID),
		Cases:    []RuleCase{{Name: counts.ID, Path: "x.md", Text: "This project has three rules.\n"}},
	})

	// The URL is joined from parts.
	registerFile(pins.ID, RulePins, "fetch.sh", "curl https://dl.pazer.build/slopfix?"+"v=1.2.3\n")

	registerWorkflowRules()
	registerTombstoneRules()
	registerWarnings()
	registerRepoRules()
	registerMessageRules()
	registerPatternRules()
	AllRules = ruleCategories()
	ReportOnly = reportOnlyIDs()
}

// reportOnlyIDs answers every rule the registry declares report-only, so a
// rule that reports alone is named by its own declaration and no second list.
func reportOnlyIDs() set.Set[string] {
	ids := set.New[string]()
	for _, r := range AllRuleSpecs() {
		if r.ReportOnly != "" {
			ids.Add(r.ID)
		}
	}
	return ids
}

// ruleCategories answers every category a registered rule declares, in the
// order the first rule named it.
func ruleCategories() []Rule {
	var out []Rule
	seen := set.New[Rule]()
	for _, r := range AllRuleSpecs() {
		if seen.Contains(r.Category) {
			continue
		}
		seen.Add(r.Category)
		out = append(out, r.Category)
	}
	return out
}

// registerTombstoneRules registers both tombstone rules the wording table
// does not carry: one names a volume, and one names a symbol nothing defines.
func registerTombstoneRules() {
	registerFile(tombstones.IDVolume, RuleTombstones, "main.go", volumeCase())
	registerFile(referentID(), RuleTombstones, "main.go", referentCase())
}

// referentID is the tombstone rule over a name no file in the repository
// defines. The table carries its wording rules and one volume rule, so the
// remaining name is the referent rule.
func referentID() string {
	patterns := patternGroups()
	for id := range tombstones.AllIDs().All() {
		if id == tombstones.IDVolume {
			continue
		}
		if _, ok := patterns[id]; ok {
			continue
		}
		return id
	}
	panic("tombstones: no referent rule")
}

// referentCase names a symbol no file in the tree defines.
func referentCase() string {
	name := "Old" + "Scanner" + "Two"
	return "package main\n\nfunc main() {\n\tvalue := 1 // The " + name + " reads each value.\n\t_ = value\n}\n"
}

// commentNumberDocCase is a Rust doc comment that states an exit code. An exit
// status is a value the program answers with, not a count of what exists here.
// No rule reports it and the repair returns the comment as the author wrote
// it.
func commentNumberDocCase() string {
	return "/// Dispatch the observe-only session-end `Stop`: runs in stop-gate mode so\n" +
		"/// exit code 2 parses as a block, but the decision is discarded (no turn\n" +
		"/// left to continue).\n" +
		"pub(crate) async fn dispatch_session_end_stop(&self, reason: &str) {\n" +
		"    if self.startup_hints.is_subagent {\n" +
		"        return;\n" +
		"    }\n" +
		"}\n"
}

// volumeCase is a comment run longer than the default cap.
func volumeCase() string {
	out := "package main\n\n"
	for i := range 16 {
		out += "// The loop reads value " + strconv.Itoa(i) + " from the input and adds it to the running total.\n"
	}
	return out + "func main() {}\n"
}

// repeatPhrase writes phrase enough times to reach a length.
func repeatPhrase(phrase string, times int) string {
	out := ""
	for range times {
		out += phrase
	}
	return out
}

// overlongComment is a Go comment far longer than the code beneath it.
func overlongComment() string {
	var out string
	for i := range 6 {
		if i > 0 {
			out += "//\n"
		}
		out += "// The loop reads each value from the input and adds it to the total.\n"
		out += "// It then checks the total against the limit that the caller set.\n"
		out += "// A total over the limit stops the loop before it writes anything.\n"
	}
	return out
}

// detectInventory answers the inventory-count rule's findings from the document
// substrate its repair reads. Check does not surface this rule's own ID, so the
// registry detects it where the fixer acts.
func detectInventory(c RuleCase) []ste.Finding {
	var out []ste.Finding
	for _, hit := range counts.Check(c.Text) {
		out = append(out, ste.Finding{
			Line:   hit.LineNo,
			ID:     counts.ID,
			Rule:   "a stated count goes stale when the set changes",
			Detail: hit.Phrase,
			Fix:    "Describe what is there and let the reader count.",
		})
	}
	return out
}

// workflowCase is the workflow the yaml rules are driven on.
func workflowCase(name, body string) RuleCase {
	return RuleCase{Name: "yaml/" + name, Path: ".github/workflows/ci.yml", Text: body}
}

// registerWorkflowRules registers the error rules the workflow rules carry.
func registerWorkflowRules() {
	registerFile(workflow.IDCommentBlock, RuleWorkflow, ".github/workflows/ci.yml", commentBlockWorkflow())
	registerFile(workflow.IDAllBuildsJob, RuleWorkflow, ".github/workflows/ci.yml", allBuildsWorkflow())
	registerFile(workflow.IDNeuteredGate, RuleWorkflow, ".github/workflows/ci.yml", neuteredGateWorkflow())
	registerFile(workflow.IDEnvIndirection, RuleWorkflow, ".github/workflows/ci.yml", envIndirectionWorkflow())
	registerFile(workflow.IDPushTags, RuleWorkflow, ".github/workflows/ci.yml", "name: CI\n\non:\n  push:\n\njobs:\n  build:\n    runs-on: ubuntu-latest\n    concurrency:\n      group: ci\n      cancel-in-progress: true\n    steps:\n      - run: echo hi\n")
	registerFile(workflow.IDOrgActionRef, RuleWorkflow, ".github/workflows/ci.yml", orgActionRefWorkflow())
	registerFile(workflow.IDConcurrency, RuleWorkflow, ".github/workflows/ci.yml", "name: CI\n\non:\n  push:\n\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo hi\n")
	registerExempt(workflow.IDBranchPin, RuleWorkflow, "no rewrite knows which ref the author meant", branchPinDetect, branchPinCase())
}

// branchPinCase pins a feature branch, so the branch-pin detection fires with
// no lookup.
func branchPinCase() RuleCase {
	return workflowCase("branch-pin", "on: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: o/r@feature\n")
}

// branchPinDetect answers the branch pins of a case against a fixed set of
// refs, so the harness proves the detection without the network.
func branchPinDetect(c RuleCase) []ste.Finding {
	found, err := workflow.BranchPins(c.Text, fixedRefs{})
	if err != nil {
		return nil
	}
	return found
}

// fixedRefs answers refs without the network: master is the default branch, feature is another branch, and v1 is a tag.
type fixedRefs struct{}

func (fixedRefs) DefaultBranch(string) (string, error) { return "master", nil }

func (fixedRefs) Kind(_, ref string) (workflow.RefKind, error) {
	switch ref {
	case "master", "feature":
		return workflow.RefBranch, nil
	case "v1":
		return workflow.RefTag, nil
	}
	return workflow.RefMissing, nil
	registerExempt(workflow.IDRunScriptSyntax, RuleWorkflow, "no rewrite knows what the script meant, and a run script line is shell",
		detectContent(workflow.IDRunScriptSyntax),
		workflowCase("run-script-syntax", "jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo (\n"))
}

func workflowHeader() string { return "name: CI\n\non:\n  push:\n    branches: ['**']\n\n" }

func workflowTail() string {
	return "jobs:\n  build:\n    runs-on: ubuntu-latest\n    concurrency:\n      group: gha_${{ github.repository }}_${{ github.workflow }}_${{ github.ref != 'refs/heads/master' && github.ref || github.run_id }}\n      cancel-in-progress: ${{ github.ref != 'refs/heads/master' }}\n    steps:\n      - run: echo hi\n"
}

func commentBlockWorkflow() string {
	return workflowHeader() + "# first line\n# second line\n# third line\n\n" + workflowTail()
}

func allBuildsWorkflow() string {
	return workflowHeader() + "jobs:\n  all-builds:\n    runs-on: ubuntu-latest\n    concurrency:\n      group: gha_${{ github.repository }}_${{ github.workflow }}_${{ github.ref != 'refs/heads/master' && github.ref || github.run_id }}\n      cancel-in-progress: ${{ github.ref != 'refs/heads/master' }}\n    steps:\n      - run: echo hi\n"
}

func neuteredGateWorkflow() string {
	return workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest\n    concurrency:\n      group: gha_${{ github.repository }}_${{ github.workflow }}_${{ github.ref != 'refs/heads/master' && github.ref || github.run_id }}\n      cancel-in-progress: ${{ github.ref != 'refs/heads/master' }}\n    steps:\n      - uses: wow-look-at-my/slopfix@master\n        continue-on-error: true\n"
}

func envIndirectionWorkflow() string {
	return workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest\n    concurrency:\n      group: gha_${{ github.repository }}_${{ github.workflow }}_${{ github.ref != 'refs/heads/master' && github.ref || github.run_id }}\n      cancel-in-progress: ${{ github.ref != 'refs/heads/master' }}\n    steps:\n      - env:\n          OUT: ${{ github.sha }}\n        run: echo \"$OUT\"\n"
}

func orgActionRefWorkflow() string {
	return workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest\n    concurrency:\n      group: gha_${{ github.repository }}_${{ github.workflow }}_${{ github.ref != 'refs/heads/master' && github.ref || github.run_id }}\n      cancel-in-progress: ${{ github.ref != 'refs/heads/master' }}\n    steps:\n      - uses: wow-look-at-my/slopfix@v1\n"
}

// registerWarnings registers the warning rules. A warning fails no check, so
// the declared exemption is that its finding never stops a build.
func registerWarnings() {
	registerExempt(workflow.IDTestInYAML, RuleWorkflow, "a warning: a test written into a workflow run script fails no check",
		detectContent(workflow.IDTestInYAML),
		workflowCase("test-in-workflow", workflowHeader()+"jobs:\n  build:\n    runs-on: ubuntu-latest\n    concurrency:\n      group: gha_${{ github.repository }}_${{ github.workflow }}_${{ github.ref != 'refs/heads/master' && github.ref || github.run_id }}\n      cancel-in-progress: ${{ github.ref != 'refs/heads/master' }}\n    steps:\n      - run: |\n          assert_ok() { exit 1; }\n"))

	for _, id := range ste.WarningIDs.Values() {
		registerExempt(id, RuleSTE, "a warning: "+warningReason(id), detectContent(id),
			RuleCase{Name: id, Path: "w.md", Text: warningCase(id)})
	}
}

// warningReason names why an STE warning fails no check.
func warningReason(id string) string {
	return "it needs a person's judgment, and a finding that must not fail is a warning"
}

// warningCase is a line that trips an STE warning rule.
func warningCase(id string) string {
	switch id {
	case ste.IDPassive:
		return "The file is read by the gate.\n"
	case ste.IDTense:
		return "The gate has read the file.\n"
	case ste.IDNounCluster:
		return "The gate file system cache lookup stopped.\n"
	case ste.IDDictionary:
		return "The tool gives additional output today.\n"
	case ste.IDParagraphLength:
		return "The gate reads. The gate writes. The gate waits. The gate stops. The gate starts. The gate ends. The gate fails.\n"
	case ste.IDInstructionLength:
		return "The gate reads the file and then it writes the result to the store for the caller before the next build starts.\n"
	}
	return "The gate reads the file.\n"
}

// registerRepoRules registers the rules that judge a repository rather than a
// file, through the files a case writes under its root.
func registerRepoRules() {
	registerTree(IDAgentsFile, RuleRepo, true, "", map[string]string{"CLAUDE.md": "# Rules\n\nDo the thing.\n"})
	registerTree(IDBudget, RuleRepo, true, "", map[string]string{"CLAUDE.md": "## Topic\n\n" + repeatPhrase("word ", CharBudget/5+10) + "\n"})
	registerTree(IDPackageScripts, RuleRepo, true, "", map[string]string{"package.json": "{\n  \"scripts\": {\n    \"build\": \"go build\"\n  }\n}\n"})
	registerTree(IDBinary, RuleRepo, true, "", map[string]string{"bin/tool": "\x7fELF\x02\x01\x01\x00rest of the executable"})
	registerTree(IDNearDuplicate, RuleRepo, false, "the reader decides which copy to keep", map[string]string{"a/list.txt": "one\ntwo\nthree\nfour\n", "b/list.txt": "one\ntwo\nthree\nfour\n"})
	registerTree(IDJSON, RuleRepo, false, "no rewrite can guess what the document meant", map[string]string{"bad.json": "{\n"})
	registerTree(IDXML, RuleRepo, false, "no rewrite can guess what the document meant", map[string]string{"bad.xml": "<a>\n"})
}

// registerMessageRules registers the closing-message guards. The repair is
// work a person does, so the declared exemption is that the sentence reports a
// decision rather than a rewritable string.
func registerMessageRules() {
	registerExempt(laziness.ID, Rule("laziness"), "the repair is work a person does, not a string a machine rewrites",
		detectLaziness, RuleCase{Name: laziness.ID, Text: "Want me to fix it?\n"})
	registerExempt(blamelanguage.ID, Rule("blame"), "the repair is to own the defect, which no string rewrite can decide",
		detectBlame, RuleCase{Name: blamelanguage.ID, Text: "That failure predates this session.\n"})
	registerExempt(askproperly.ID, Rule("ask"), "the repair is to make the decision and say what was assumed",
		detectAsk, RuleCase{Name: askproperly.ID, Text: "Should I use the cache or the store?\n"})
}

func detectLaziness(c RuleCase) []ste.Finding {
	var out []ste.Finding
	for _, hit := range laziness.Check(c.Text) {
		out = append(out, ste.Finding{Line: hit.Line, ID: hit.ID, Rule: hit.Tell, Detail: hit.Sentence})
	}
	return out
}

func detectBlame(c RuleCase) []ste.Finding {
	var out []ste.Finding
	for _, hit := range blamelanguage.Check(c.Text) {
		detail := hit.Phrase
		if detail == "" {
			detail = hit.Sentence
		}
		out = append(out, ste.Finding{Line: hit.Line, ID: hit.ID, Rule: hit.Tell, Detail: detail})
	}
	return out
}

func detectAsk(c RuleCase) []ste.Finding {
	var out []ste.Finding
	for _, hit := range askproperly.FindQuestions(c.Text) {
		out = append(out, ste.Finding{Line: lineOf(c.Text, hit.Line), ID: askproperly.ID, Rule: "a decision handed to the reader in prose", Detail: hit.Text})
	}
	return out
}

// lineOf answers the line number, counting from one, of the first line whose
// trimmed text equals the candidate.
func lineOf(text, candidate string) int {
	for i, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == strings.TrimSpace(candidate) {
			return i + 1
		}
	}
	return 1
}

// registerPatternRules registers every identified pattern the english table
// carries. A pattern's own worked examples are its cases, so the harness proves
// each cut clears the phrase it was written to cut.
func registerPatternRules() {
	groups := patternGroups()
	ids := set.New[string]()
	for id := range groups {
		ids.Add(id)
	}
	for _, id := range slices.Sorted(ids.All()) {
		patterns := groups[id]
		cases := patternCases(patterns, id)
		if len(cases) == 0 {
			continue
		}
		RegisterRule(RuleSpec{
			ID:       id,
			Category: RuleTombstones,
			Detect:   detectPattern(patterns, id),
			Autofix:  repairPattern(patterns),
			Cases:    cases,
		})
	}
}
