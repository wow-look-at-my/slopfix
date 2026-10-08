package workflow

import (
	"path"
	"regexp"
	"strings"

	"github.com/wow-look-at-my/slopfix/ste"
	yaml "go.yaml.in/yaml/v3"
)

// shellStep is a run: step whose script is POSIX shell.
type shellStep struct {
	// job is the key of the job the step belongs to.
	job string
	// number is the step's place in its job's steps, counted from one.
	number int
	step   *yaml.Node
	run    *yaml.Node
	block  scriptRows
	// shell is the step's own shell key, or else the job's or the workflow's defaults.run.shell.
	shell string
	// moved reports a working-directory that starts the step outside the checkout root.
	moved bool
}

// shellSteps answers every run: step whose script is POSIX shell, read off the
// parser. A step that runs PowerShell, Python or cmd holds no shell test.
func shellSteps(content string) []shellStep {
	var doc yaml.Node
	if yaml.Unmarshal([]byte(content), &doc) != nil {
		return nil
	}
	root := rootOf(&doc)
	jobs := mappingValue(root, "jobs")
	if jobs == nil {
		return nil
	}
	rows := lines(content)
	spans := blockScalars(content)
	var out []shellStep
	for i := 0; i+1 < len(jobs.Content); i += 2 {
		job := jobs.Content[i+1]
		steps := mappingValue(job, "steps")
		if steps == nil || steps.Kind != yaml.SequenceNode {
			continue
		}
		shell := defaultShell(job)
		if shell == "" {
			shell = defaultShell(root)
		}
		windows := runsPowerShell(job, shell)
		moved := defaultDirectory(job) || defaultDirectory(root)
		for n, step := range steps.Content {
			run := mappingValue(step, "run")
			if run == nil || run.Kind != yaml.ScalarNode {
				continue
			}
			s := shellStep{job: jobs.Content[i].Value, number: n + 1, step: step, run: run, shell: shell,
				moved: moved || mappingValue(step, "working-directory") != nil}
			if own := mappingValue(step, "shell"); own != nil {
				s.shell = own.Value
			} else if windows {
				continue
			}
			if !bashLike(s.shell) {
				continue
			}
			s.block = scriptOf(run, rows, spans)
			out = append(out, s)
		}
	}
	return out
}

// defaultDirectory reports a defaults.run.working-directory on a job or a workflow.
func defaultDirectory(node *yaml.Node) bool {
	return mappingValue(mappingValue(mappingValue(node, "defaults"), "run"), "working-directory") != nil
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
// a warning. The repair moves the whole script into a file of its own.
func testsInYAML(content string) []ste.Finding {
	var out []ste.Finding
	for _, s := range shellSteps(content) {
		for _, f := range s.block.findings() {
			f.Severity = ste.SeverityWarning
			out = append(out, f)
		}
	}
	return out
}

func (b scriptRows) findings() []ste.Finding {
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
