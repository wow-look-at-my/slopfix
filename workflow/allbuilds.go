package workflow

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/ste"
	yaml "go.yaml.in/yaml/v3"
)

// GuardedName is the name no job may carry: the gate is a commit status.
const GuardedName = "all-builds"

// shadows reports whether a rendered job name is the guarded name. GitHub adds
// a matrix suffix in parentheses, and a reusable workflow's path parts around a
// slash. A segment matches exactly, so all-builds2 is a different name.
func shadows(name string) bool {
	candidate := strings.TrimSpace(name)
	if strings.HasSuffix(candidate, ")") {
		if cut := strings.LastIndex(candidate, " ("); cut != -1 {
			candidate = candidate[:cut]
		}
	}
	for _, segment := range strings.Split(candidate, " / ") {
		if segment == GuardedName {
			return true
		}
	}
	return false
}

// allBuildsJobs reports each job named all-builds, by its key or by its name.
//
// Unparseable YAML yields nothing. A file this rule cannot read is a file the
// runner cannot read either, and it fails on its own.
func allBuildsJobs(content string) []ste.Finding {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil
	}
	jobs := mappingValue(rootOf(&doc), "jobs")
	if jobs == nil {
		return nil
	}
	var out []ste.Finding
	for i := 0; i+1 < len(jobs.Content); i += 2 {
		key, job := jobs.Content[i], jobs.Content[i+1]
		if key.Value == GuardedName {
			out = append(out, finding(key.Line, "its job key"))
			continue
		}
		name := mappingValue(job, "name")
		// An expression resolves at run time, so no file can judge it.
		if name == nil || name.Kind != yaml.ScalarNode || strings.Contains(name.Value, "${{") {
			continue
		}
		if shadows(name.Value) {
			out = append(out, finding(name.Line, "its name"))
		}
	}
	return out
}

// finding carries the operator's own wording. Do not soften it.
func finding(line int, via string) ste.Finding {
	return ste.Finding{
		Line:   line,
		ID:     IDAllBuildsJob,
		Rule:   "a job named " + GuardedName + " is a known deception attempt",
		Detail: via,
		Fix: "It does not satisfy the org's required " + GuardedName + " gate, which is the required-builds-manager app's status. " +
			"It only shadows the real gate in the GitHub UI. Rename the job. Do not try to work around this check.",
	}
}

func rootOf(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}
	return doc
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
