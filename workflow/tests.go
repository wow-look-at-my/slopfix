package workflow

import (
	"path"
	"regexp"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/ste"
)

// runBlock is a run: script.
type runBlock = scriptRows

// runBlocks reads every run: script off the parser.
func runBlocks(content string) []runBlock {
	return scripts(content)
}

// testFileNames match a name only a test suite gives a file.
var testFileNames = []*regexp.Regexp{
	regexp.MustCompile(`_test\.(go|py|rs|c|cc|cpp|exs?)$`),
	regexp.MustCompile(`\.test\.[jt]sx?$`),
	regexp.MustCompile(`\.spec\.[jt]sx?$`),
	regexp.MustCompile(`^test_[^/]*\.(py|c|cc|cpp)$`),
	regexp.MustCompile(`_spec\.rb$`),
	regexp.MustCompile(`Test\.(java|kt|cs)$`),
	regexp.MustCompile(`\.dats$`),
	regexp.MustCompile(`^conftest\.py$`),
}

func namesATestFile(target string) bool {
	base := path.Base(strings.ReplaceAll(target, `\`, "/"))
	for _, pattern := range testFileNames {
		if pattern.MatchString(base) {
			return true
		}
	}
	return false
}

// comparison spells a bracket apart: a bracket takes no word boundary.
const comparison = `(?:(?:grep|egrep|fgrep|diff|cmp|test|jq\s+-e)\b|\[\[?)`

// assertions pair a comparison with a nonzero exit. That pairing is the test.
var assertions = []*regexp.Regexp{
	regexp.MustCompile(comparison + `[^;&|]*\|\|\s*(?:\{|\(|exit\b|echo\b|printf\b|core\b)`),
	regexp.MustCompile(`^\s*if\s+!\s*` + comparison),
	regexp.MustCompile(comparison + `[^;&|]*&&\s*\{?\s*(?:echo|printf)[^;]*::error`),
	// A case arm asserts with no comparison word: it annotates and exits.
	regexp.MustCompile(`::error.*\bexit\s+[1-9]`),
}

// assertHelper matches a shell function whose name says it asserts.
var assertHelper = regexp.MustCompile(`^\s*(?:function\s+)?(assert|expect|require|must|fail_if|check_that)[A-Za-z0-9_]*\s*\(\)`)

// redirectTarget matches a redirect and the file it names.
var redirectTarget = regexp.MustCompile(`(?:^|[^>\d])>>?\s*(?:"([^"]+)"|'([^']+)'|([^\s;&|]+))`)

var exitWord = regexp.MustCompile(`\bexit\b`)

// testsInYAML reports every test written into a run: script. Each finding is
// a warning: a line of a run: script is shell. Repair removes the line, and
// moving the case into the repository suite is the author's call.
func testsInYAML(content string) []ste.Finding {
	var out []ste.Finding
	for _, block := range runBlocks(content) {
		for _, f := range block.findings() {
			f.Severity = ste.SeverityWarning
			out = append(out, f)
		}
	}
	return out
}

// Repair rewrites the workflow so no run: script holds a test. The lines
// that carry one come out, and the step keeps its other commands.
func RepairTests(content string) string {
	rows := testRows(content)
	if rows.Len() == 0 {
		return content
	}
	kept := []string{}
	for i, row := range strings.Split(content, "\n") {
		if !rows.Contains(i) {
			kept = append(kept, row)
		}
	}
	return strings.Join(kept, "\n")
}

// testRows answers the rows, counted from zero, that carry a test. A run:
// script left empty takes its step entry with it, so the job keeps the steps
// around it.
func testRows(content string) set.Set[int] {
	out := set.New[int]()
	findings := testsInYAML(content)
	if len(findings) == 0 {
		return out
	}
	drop := make(map[int]bool, len(findings))
	for _, finding := range findings {
		drop[finding.Line] = true
	}
	inside := blockScalarRows(content)
	for _, block := range runBlocks(content) {
		if len(block.lines) == 0 || !allDropped(block, drop) {
			continue
		}
		// The block would be left empty. Its step entry comes out too, so
		// the job keeps the steps around it.
		if header := block.start - 1; header >= 0 && header < len(inside) && inside[header] {
			drop[header] = true
		}
	}
	for line, dropped := range drop {
		if dropped {
			out.Add(line - 1)
		}
	}
	return out
}

// allDropped reports whether every line of a run block carries a finding.
func allDropped(block runBlock, drop map[int]bool) bool {
	for offset := range block.lines {
		if !drop[block.start+offset] {
			return false
		}
	}
	return true
}

func (b runBlock) findings() []ste.Finding {
	script := strings.Join(b.lines, "\n")
	annotates := strings.Contains(script, "::error")
	ends := exitWord.MatchString(script)

	var out []ste.Finding
	for offset, line := range b.lines {
		number := b.start + offset

		if target := redirected(line); target != "" && namesATestFile(target) {
			out = append(out, ste.Finding{
				Line:   number,
				ID:     IDTestInYAML,
				Rule:   "a workflow writes a test file",
				Detail: evidence(line),
				Fix:    path.Base(target) + " is a test file. Commit it to the repository's own suite, and let this step invoke the suite.",
			})
			continue
		}

		if assertHelper.MatchString(line) {
			out = append(out, ste.Finding{
				Line:   number,
				ID:     IDTestInYAML,
				Rule:   "a workflow defines its own assertion helper",
				Detail: evidence(line),
				Fix:    "That is a test framework with no test runner. Move these cases into the repository suite.",
			})
			continue
		}

		// An error annotation alone is a report. With a comparison that ends
		// the step, it is an expectation.
		if !annotates && !ends {
			continue
		}
		if matchesAnAssertion(line) {
			out = append(out, ste.Finding{
				Line:   number,
				ID:     IDTestInYAML,
				Rule:   "a workflow asserts on a value",
				Detail: evidence(line),
				Fix:    "This compares a value and fails the step on the answer. Put it in the repository suite, and invoke the suite here.",
			})
		}
	}
	return out
}

func matchesAnAssertion(line string) bool {
	for _, pattern := range assertions {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}

func redirected(line string) string {
	match := redirectTarget.FindStringSubmatch(line)
	if match == nil {
		return ""
	}
	for _, group := range match[1:] {
		if group != "" {
			return group
		}
	}
	return ""
}

// EvidenceCap bounds the quoted line, so a long command cannot fill a report.
const EvidenceCap = 120

func evidence(line string) string {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) > EvidenceCap {
		return trimmed[:EvidenceCap-3] + "..."
	}
	return trimmed
}
