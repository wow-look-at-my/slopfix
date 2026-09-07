// Package workflow checks a GitHub Actions workflow or an action manifest.
//
// The prose rules read a document. These read the other file the org's
// common-checks gate rejects, so the same tool answers for both.
package workflow

import (
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/wow-look-at-my/slopfix/ste"
)

// The rule IDs. A report prints the ID that found the text.
const (
	IDCommentBlock = "yaml/comment-block"
	IDAllBuildsJob = "yaml/all-builds-job"
	IDTestInYAML   = "yaml/test-in-workflow"
	IDNeuteredGate = "yaml/neutered-gate"
)

// AllIDs names every rule this package reports. A set, because every consumer
// asks whether a name is in it.
var AllIDs = set.Of(IDCommentBlock, IDAllBuildsJob, IDTestInYAML, IDNeuteredGate)

// Judges reports whether these rules read the file at this path. A backslash is
// separated here rather than through filepath, which ignores it off Windows.
func Judges(name string) bool {
	slashed := strings.ReplaceAll(name, `\`, "/")
	base := path.Base(slashed)
	if base == "action.yml" || base == "action.yaml" {
		return true
	}
	if !strings.HasSuffix(base, ".yml") && !strings.HasSuffix(base, ".yaml") {
		return false
	}
	return strings.Contains(slashed, "/workflows/") || strings.HasPrefix(slashed, "workflows/")
}

// topLevelKey matches the left-margin key that names a workflow or an action.
var topLevelKey = regexp.MustCompile(`(?m)^(jobs|runs):`)

// Sniff reads what the path does not say, for a caller inside the directory.
func Sniff(content string) bool {
	return topLevelKey.MatchString(content)
}

// Check reports every finding in a workflow or action file, in source order.
func Check(content string) []ste.Finding {
	out := commentBlocks(content)
	out = append(out, allBuildsJobs(content)...)
	out = append(out, testsInYAML(content)...)
	out = append(out, neuteredGates(content)...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return out
}

// lines splits a document, and drops the carriage return a Windows editor left.
func lines(content string) []string {
	split := strings.Split(content, "\n")
	for i, line := range split {
		split[i] = strings.TrimSuffix(line, "\r")
	}
	return split
}
