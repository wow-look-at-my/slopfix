// subject.go answers whether a tool call is a status read, and what it is a
// status read ABOUT.
//
// The subject is what makes this half work where the Stop half's signature
// comparison does not. A session that asks the same question several ways --
// `gh wait-ci`, then `gh wait-ci checks`, then `pull_request_read` -- produces
// differing signatures and no streak, while every call asks after the same
// pull request and learns the same nothing. Keying on the subject rather than
// the command text is what stops a re-spelling from laundering a repeat.
package busypoll

import (
	"regexp"
	"sort"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// statusReadTools are the non-Bash tools whose entire product is "what is
// the state of X right now". A tool that CHANGES something is never here.
var statusReadTools = set.Of(
	"pull_request_read",
	"list_pull_requests",
	"search_pull_requests",
	"get_commit",
	"list_commits",
	"list_branches",
	"get_session",
	"list_sessions",
	"list_triggers",
	"readnotifications",
	"taskget",
	"taskoutput",
	"tasklist",
)

// statusReadCommands are the Bash spellings of the same question, in statement
// position. Every entry runs `gh`: `git` reads local objects.
var statusReadCommands = []string{
	"gh wait-ci",
	"gh pr view",
	"gh pr checks",
	"gh pr status",
	"gh pr list",
	"gh run view",
	"gh run list",
	"gh run watch",
}

// contentSubcommands read a run's OUTPUT rather than its state, which a
// finished run does not change.
var contentSubcommands = set.Of(
	"log", "grep", "annotations", "artifacts", "jobs", "workflows",
)

// afterWaitCI is the subcommand word following `gh wait-ci`, lowercased, or
// "" when the command names none.
func afterWaitCI(lower string, at int) string {
	rest := strings.TrimSpace(lower[at+len("gh wait-ci"):])
	for _, f := range strings.Fields(rest) {
		if strings.HasPrefix(f, "-") {
			continue
		}
		return f
	}
	return ""
}

var (
	reSlugNum   = regexp.MustCompile(`([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)#(\d+)`)
	reOwner     = regexp.MustCompile(`"owner"\s*:\s*"([^"]+)"`)
	reRepo      = regexp.MustCompile(`"repo"\s*:\s*"([^"]+)"`)
	rePullNum   = regexp.MustCompile(`"pull(?:_?[Nn]umber|Request(?:_?[Nn]umber)?)"\s*:\s*(\d+)`)
	reRepoFlag  = regexp.MustCompile(`--repo[= ]([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)`)
	rePRCommand = regexp.MustCompile(`\bgh\s+pr\s+(?:view|checks|status)\s+(\d+)`)
	reSHA       = regexp.MustCompile(`\b([0-9a-f]{7,40})\b`)
	reHasDigit  = regexp.MustCompile(`[0-9]`)
	reHasHexAZ  = regexp.MustCompile(`[a-f]`)
	reStatement = regexp.MustCompile(`(^|[|;&(]\s*|&&\s*|\|\|\s*)$`)
)

// isStatusRead reports whether this call's only product is a current-state answer.
func isStatusRead(c toolCall) bool {
	name := strings.ToLower(c.name)
	if name == "bash" {
		return namesAStatusCommand(commandOf(c.input))
	}
	// An MCP tool arrives as mcp__<server>__<Tool>; the bare trailing name identifies it.
	if i := strings.LastIndex(name, "__"); i >= 0 {
		name = name[i+2:]
	}
	return statusReadTools.Contains(name)
}

// namesAStatusCommand reports whether cmd runs a status command in statement position.
func namesAStatusCommand(cmd string) bool {
	return len(statusStatements(cmd)) > 0
}

// statusStatements returns the text of each status command cmd runs, from its command word to the end of that statement.
// The bound is what makes the subject the command's own: it must sit in the arguments of the GitHub read itself.
func statusStatements(cmd string) []string {
	lower := strings.ToLower(cmd)
	var out []string
	for _, want := range statusReadCommands {
		for i := 0; ; {
			j := strings.Index(lower[i:], want)
			if j < 0 {
				break
			}
			at := i + j
			i = at + len(want)
			if !reStatement.MatchString(lower[:at]) {
				continue
			}
			if want == "gh wait-ci" && contentSubcommands.Contains(afterWaitCI(lower, at)) {
				continue
			}
			out = append(out, cmd[at:at+statementLen(cmd[at:])])
		}
	}
	return out
}

// statementLen is how far a statement runs from the start of s: anything that begins a new command ends it.
func statementLen(s string) int {
	for i := range len(s) {
		switch s[i] {
		case ';', '\n', '|', '&', ')', '`':
			return i
		}
	}
	return len(s)
}

// reCommand is hoisted because the ledger walk reads every Bash call in the window.
var reCommand = regexp.MustCompile(`"command"\s*:\s*"((?:[^"\\]|\\.)*)"`)

// commandOf pulls the command out of a Bash call's input, undone to the text
// the shell would have run. Encoders differ on how `&&` arrives.
func commandOf(input []byte) string {
	m := reCommand.FindSubmatch(input)
	if m == nil {
		return ""
	}
	return unescapeReplacer.Replace(string(m[1]))
}

// subjectsIn returns every state-carrying thing named in text, normalized so the same subject compares equal.
func subjectsIn(text string) []string {
	seen := set.New[string]()

	for _, m := range reSlugNum.FindAllStringSubmatch(text, -1) {
		seen.Add("pr:" + strings.ToLower(m[1]) + "#" + m[2])
	}

	owner := firstGroup(reOwner, text)
	repo := firstGroup(reRepo, text)
	if num := firstGroup(rePullNum, text); num != "" {
		if owner != "" && repo != "" {
			seen.Add("pr:" + strings.ToLower(owner+"/"+repo) + "#" + num)
		} else {
			seen.Add("pr:#" + num)
		}
	}

	if m := rePRCommand.FindStringSubmatch(text); m != nil {
		if slug := firstGroup(reRepoFlag, text); slug != "" {
			seen.Add("pr:" + strings.ToLower(slug) + "#" + m[1])
		} else {
			seen.Add("pr:#" + m[1])
		}
	}

	for _, m := range reSHA.FindAllStringSubmatch(text, -1) {
		if looksLikeSHA(m[1]) {
			seen.Add("sha:" + m[1][:7])
		}
	}

	out := make([]string, 0, seen.Len())
	for s := range seen.All() {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// looksLikeSHA separates a commit hash from a decimal number and from a word spelled in hex.
func looksLikeSHA(s string) bool {
	return reHasDigit.MatchString(s) && reHasHexAZ.MatchString(s)
}

func firstGroup(re *regexp.Regexp, text string) string {
	if m := re.FindStringSubmatch(text); m != nil {
		return m[1]
	}
	return ""
}

// prSubjects filters to pull-request subjects, which are the only ones a
// merge verdict can be attributed to.
func prSubjects(subs []string) []string {
	var out []string
	for _, s := range subs {
		if strings.HasPrefix(s, "pr:") {
			out = append(out, s)
		}
	}
	return out
}
