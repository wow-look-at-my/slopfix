package workflow

import (
	"path"
	"regexp"
	"strings"

	"github.com/wow-look-at-my/slopfmt/ste"
)

// runBlock is a run: script, and the line its content starts on.
type runBlock struct {
	start int
	lines []string
}

var runKey = regexp.MustCompile(`^(\s*)(-\s+)?run:\s*(.*)$`)

// blockScalar matches the | and > forms, with their chomping and indent digits.
var blockScalar = regexp.MustCompile(`^[|>][+-]?\d*$`)

// runBlocks reads every run: script. A regex walk rather than a parse: run:
// means the same thing at every depth, and a file this rule rejects may also be
// a file a parser rejects.
func runBlocks(content string) []runBlock {
	all := lines(content)
	var blocks []runBlock

	for index := 0; index < len(all); index++ {
		match := runKey.FindStringSubmatch(all[index])
		if match == nil {
			continue
		}
		// A sequence item carries its key past the dash, so the KEY sets the end.
		indent := len(match[1]) + len(match[2])
		rest := strings.TrimSpace(match[3])
		if !blockScalar.MatchString(rest) {
			if rest != "" {
				blocks = append(blocks, runBlock{start: index + 1, lines: []string{rest}})
			}
			continue
		}

		var body []string
		cursor := index + 1
		for ; cursor < len(all); cursor++ {
			line := all[cursor]
			if strings.TrimSpace(line) == "" {
				body = append(body, "")
				continue
			}
			if indentOf(line) <= indent {
				break
			}
			body = append(body, line)
		}
		// A trailing run of blank lines belongs to the document, not the script.
		for len(body) > 0 && body[len(body)-1] == "" {
			body = body[:len(body)-1]
		}
		if len(body) > 0 {
			blocks = append(blocks, runBlock{start: index + 2, lines: body})
		}
		index = cursor - 1
	}
	return blocks
}

func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
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

// testsInYAML reports every test written into a run: script.
func testsInYAML(content string) []ste.Finding {
	var out []ste.Finding
	for _, block := range runBlocks(content) {
		out = append(out, block.findings()...)
	}
	return out
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
