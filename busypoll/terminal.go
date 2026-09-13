// terminal.go finds the subjects this session has already watched reach a
// state they cannot leave.
//
// A merged pull request does not un-merge, and a commit that went green does
// not go red: the sha is fixed, so a later push produces a DIFFERENT sha and
// therefore a different subject. Re-reading either is the purest form of the
// waste this rule exists to stop -- not a loop that eventually learns
// something, but a question whose answer is already in the transcript and
// cannot change.
package busypoll

import "strings"

// mergeVerdicts mark a pull request as finished. Each is a phrase a result
// really carries, taken from the payloads this environment delivers.
var mergeVerdicts = []string{
	`"outcome":"merged"`,
	`"merged":true`,
	`"state":"merged"`,
	`"state":"closed"`,
	`pull_request.closed`,
	`has been merged`,
}

// greenVerdicts mark a commit's checks as finished and passing.
var greenVerdicts = []string{
	`ci passed`,
	`builds passed`,
	`rollup: success`,
	`(rollup: success)`,
}

// terminalSubjects returns the subjects a record reported as finished. A record
// may only settle a subject it names ALONE.
func terminalSubjects(recs []record) map[string]bool {
	out := map[string]bool{}
	for _, r := range recs {
		lower := strings.ToLower(r.raw)
		subs := subjectsIn(lower)

		if containsAny(lower, mergeVerdicts) {
			if prs := prSubjects(subs); len(prs) == 1 {
				out[prs[0]] = true
			}
		}
		if containsAny(lower, greenVerdicts) {
			if shas := shaSubjects(subs); len(shas) == 1 {
				out[shas[0]] = true
			}
		}
	}
	return out
}

// shaSubjects keeps the commits out of a record's subjects. A verdict settles
// the one commit its record is about, and a record naming several says which
// of them went green no more than it says which did not.
func shaSubjects(subs []string) []string {
	var out []string
	for _, sub := range subs {
		if strings.HasPrefix(sub, "sha:") {
			out = append(out, sub)
		}
	}
	return out
}

func containsAny(text string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(text, n) {
			return true
		}
	}
	return false
}
