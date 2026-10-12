package workflow

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/ste"
	yaml "go.yaml.in/yaml/v3"
)

// IDConcurrency is a workflow without the org's concurrency block.
const IDConcurrency = "yaml/concurrency"

// The group is unique to a branch off master, so a new push cancels the run.
// On master the run ID makes each group unique, so every run completes. A
// concurrency group spans every workflow in the repository.
const (
	concurrencyGroup  = "gha_${{ github.repository }}_${{ github.workflow }}_${{ github.ref != 'refs/heads/master' && github.ref || github.run_id }}"
	concurrencyCancel = "${{ github.ref != 'refs/heads/master' }}"
)

// concurrencyLines are the lines the repair writes, under the key's own indent.
func concurrencyLines(indent, step string) []string {
	return []string{
		indent + "concurrency:",
		indent + step + "group: " + concurrencyGroup,
		indent + step + "cancel-in-progress: " + concurrencyCancel,
	}
}

// concurrency reports a workflow whose top-level concurrency block is not the
// org's. An action manifest has no such key. A reusable workflow is skipped,
// because its github context is the caller's, and the same group then deadlocks.
func concurrency(content string) []ste.Finding {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil
	}
	root := rootOf(&doc)
	onKey, jobs := mappingKey(root, "on"), mappingValue(root, "jobs")
	if onKey == nil || jobs == nil || reusable(mappingValue(root, "on")) {
		return nil
	}
	key, value := mappingKey(root, "concurrency"), mappingValue(root, "concurrency")
	if key == nil {
		return []ste.Finding{concurrencyFinding(onKey.Line, "a workflow with no concurrency block runs every push to a branch to the end")}
	}
	if !orgConcurrency(value) {
		return []ste.Finding{concurrencyFinding(key.Line, "the concurrency block is not the org's per-branch block")}
	}
	return nil
}

// reusable reports a trigger that names workflow_call.
func reusable(trigger *yaml.Node) bool {
	if trigger == nil {
		return false
	}
	switch trigger.Kind {
	case yaml.ScalarNode:
		return trigger.Value == "workflow_call"
	case yaml.SequenceNode:
		for _, item := range trigger.Content {
			if item.Value == "workflow_call" {
				return true
			}
		}
	case yaml.MappingNode:
		return mappingKey(trigger, "workflow_call") != nil
	}
	return false
}

// orgConcurrency reports a block that holds the org's group and cancel value,
// and nothing else. Blanks inside an expression do not count.
func orgConcurrency(value *yaml.Node) bool {
	if value == nil || value.Kind != yaml.MappingNode || len(value.Content) != 4 {
		return false
	}
	group, cancel := mappingValue(value, "group"), mappingValue(value, "cancel-in-progress")
	return group != nil && cancel != nil &&
		squeeze(group.Value) == squeeze(concurrencyGroup) &&
		squeeze(cancel.Value) == squeeze(concurrencyCancel)
}

// squeeze drops every blank, so spellings of one expression compare equal.
func squeeze(s string) string {
	return strings.Join(strings.Fields(s), "")
}

func concurrencyFinding(line int, rule string) ste.Finding {
	return ste.Finding{
		Line: line,
		ID:   IDConcurrency,
		Rule: rule,
		Fix: "Set `concurrency:` to `group: " + concurrencyGroup + "` and `cancel-in-progress: " + concurrencyCancel +
			"`. A push to a branch then cancels the run before it. Every run on master completes. `slopfix fix` does this.",
	}
}

// setConcurrency writes the org's block. A block that exists is written again
// in place. With no block, the lines go after the on: value.
func setConcurrency(content string) []edit.Edit {
	if len(concurrency(content)) == 0 {
		return nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil
	}
	root := rootOf(&doc)
	fileLines := lines(content)
	step := childIndent(root, fileLines)
	if key := mappingKey(root, "concurrency"); key != nil {
		indent := fileLines[key.Line-1][:key.Column-1]
		return []edit.Edit{rewrite(content, key.Line-1, lastTriggerLine(fileLines, root, key), concurrencyLines(indent, step))}
	}
	onKey := mappingKey(root, "on")
	indent := fileLines[onKey.Line-1][:onKey.Column-1]
	last := lastTriggerLine(fileLines, root, onKey)
	out := append([]string{fileLines[last]}, concurrencyLines(indent, step)...)
	if last+1 < len(fileLines) && strings.TrimSpace(fileLines[last+1]) == "" {
		out = append([]string{fileLines[last], ""}, concurrencyLines(indent, step)...)
	}
	return []edit.Edit{rewrite(content, last, last, out)}
}

// childIndent answers the step the file indents a block mapping by, read off the
// first top-level block mapping that has a child. The default is spaces.
func childIndent(root *yaml.Node, fileLines []string) string {
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, value := root.Content[i], root.Content[i+1]
		if value.Kind != yaml.MappingNode || value.Style&yaml.FlowStyle != 0 || len(value.Content) == 0 {
			continue
		}
		if step := value.Content[0].Column - key.Column; step > 0 && value.Content[0].Line > key.Line {
			line := fileLines[value.Content[0].Line-1]
			return line[key.Column-1 : value.Content[0].Column-1]
		}
	}
	return "  "
}
