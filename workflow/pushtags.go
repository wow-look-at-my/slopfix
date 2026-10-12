package workflow

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/ste"
	yaml "go.yaml.in/yaml/v3"
)

// IDPushTags is a push trigger that names no ref filter, so a tag push starts the workflow.
const IDPushTags = "yaml/push-tags"

// refFilters are the push keys that keep a trigger off at least some refs.
var refFilters = []string{"branches", "branches-ignore", "tags", "tags-ignore"}

// pushTags reports a push trigger with no ref filter. Unparseable YAML yields
// nothing, because the runner rejects that file on its own.
func pushTags(content string) []ste.Finding {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil
	}
	trigger := mappingValue(rootOf(&doc), "on")
	if trigger == nil {
		return nil
	}
	switch trigger.Kind {
	case yaml.ScalarNode:
		if trigger.Value == "push" {
			return []ste.Finding{pushFinding(trigger.Line)}
		}
	case yaml.SequenceNode:
		for _, item := range trigger.Content {
			if item.Kind == yaml.ScalarNode && item.Value == "push" {
				return []ste.Finding{pushFinding(item.Line)}
			}
		}
	case yaml.MappingNode:
		key := mappingKey(trigger, "push")
		if key == nil {
			return nil
		}
		push := mappingValue(trigger, "push")
		for _, name := range refFilters {
			if mappingKey(push, name) != nil {
				return nil
			}
		}
		return []ste.Finding{pushFinding(key.Line)}
	}
	return nil
}

func pushFinding(line int) ste.Finding {
	return ste.Finding{
		Line: line,
		ID:   IDPushTags,
		Rule: "a push trigger with no ref filter also runs on every tag push",
		Fix:  "Add `branches: ['**']` under `push:`, or `tags-ignore: ['**']`, to keep the workflow on branch pushes. `slopfix fix` does this.",
	}
}

// branchFilter is the filter the repair writes. It matches every branch and no tag.
const branchFilter = "branches: ['**']"

// filterPush writes branchFilter under an unfiltered push trigger. A shape no
// single line edit reaches gets its whole on: value written again in block style.
func filterPush(content string) []edit.Edit {
	if len(pushTags(content)) == 0 {
		return nil
	}
	if edits := filterPushLine(content); edits != nil {
		return edits
	}
	return reblock(content)
}

// filterPushLine writes branchFilter with an edit of the lines the trigger
// already holds.
func filterPushLine(content string) []edit.Edit {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil
	}
	root := rootOf(&doc)
	key, trigger := mappingKey(root, "on"), mappingValue(root, "on")
	fileLines := lines(content)
	lineNo := key.Line - 1
	indent := fileLines[lineNo][:key.Column-1]
	switch trigger.Kind {
	case yaml.ScalarNode:
		if trigger.Line != key.Line {
			return nil
		}
		head := strings.TrimRight(fileLines[lineNo][:trigger.Column-1], " ")
		tail := fileLines[lineNo][trigger.Column-1+len(trigger.Value):]
		return []edit.Edit{edit.Lines(content, lineNo, lineNo, 0, []string{
			head + tail, indent + "  push:", indent + "    " + branchFilter,
		})}
	case yaml.SequenceNode:
		if trigger.Style&yaml.FlowStyle == 0 || trigger.Line != key.Line {
			return nil
		}
		out := []string{strings.TrimRight(fileLines[lineNo][:trigger.Column-1], " ")}
		for _, item := range trigger.Content {
			if item.Kind != yaml.ScalarNode || item.Line != key.Line {
				return nil
			}
			out = append(out, indent+"  "+item.Value+":")
			if item.Value == "push" {
				out = append(out, indent+"    "+branchFilter)
			}
		}
		return []edit.Edit{edit.Lines(content, lineNo, lineNo, 0, out)}
	case yaml.MappingNode:
		pushKey, push := mappingKey(trigger, "push"), mappingValue(trigger, "push")
		pushLine := pushKey.Line - 1
		child := fileLines[pushLine][:pushKey.Column-1] + "  "
		switch {
		case push.Kind == yaml.ScalarNode && push.Value == "" && push.Line == pushKey.Line:
		case push.Kind == yaml.MappingNode && push.Style&yaml.FlowStyle == 0 && len(push.Content) > 0:
			first := push.Content[0]
			child = fileLines[first.Line-1][:first.Column-1]
		default:
			return nil
		}
		return []edit.Edit{edit.Lines(content, pushLine, pushLine, 0, []string{fileLines[pushLine], child + branchFilter})}
	}
	return nil
}
