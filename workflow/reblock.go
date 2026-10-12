package workflow

import (
	"bytes"
	"strings"

	"github.com/wow-look-at-my/slopfix/edit"
	yaml "go.yaml.in/yaml/v3"
)

// reblock writes the whole on: value again in block style, with branchFilter
// under push. It serves the shapes a line edit cannot reach, such as `push: {}`.
func reblock(content string) []edit.Edit {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil
	}
	root := rootOf(&doc)
	key, trigger := mappingKey(root, "on"), mappingValue(root, "on")
	if key == nil || trigger == nil {
		return nil
	}
	events := blockEvents(trigger)
	if events == nil {
		return nil
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	wrapper := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Value: key.Value}, events}}
	if err := enc.Encode(wrapper); err != nil || enc.Close() != nil {
		return nil
	}
	fileLines := lines(content)
	keyLine := fileLines[key.Line-1]
	indent := keyLine[:key.Column-1]
	var out []string
	for _, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
		out = append(out, indent+line)
	}
	// The key keeps the spelling it had, quotes included.
	if colon := strings.Index(keyLine[key.Column-1:], ":"); colon > 0 && len(out) > 0 {
		out[0] = keyLine[:key.Column-1+colon+1]
	}
	return []edit.Edit{rewrite(content, key.Line-1, lastTriggerLine(fileLines, root, key), out)}
}

// blockEvents answers the trigger as a block mapping of events, push filtered.
func blockEvents(trigger *yaml.Node) *yaml.Node {
	events := &yaml.Node{Kind: yaml.MappingNode}
	add := func(name string) {
		value := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
		if name == "push" {
			value = filtered(nil)
		}
		events.Content = append(events.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: name}, value)
	}
	switch trigger.Kind {
	case yaml.ScalarNode:
		add(trigger.Value)
	case yaml.SequenceNode:
		for _, item := range trigger.Content {
			if item.Kind != yaml.ScalarNode {
				return nil
			}
			add(item.Value)
		}
	case yaml.MappingNode:
		events.HeadComment = trigger.HeadComment
		for i := 0; i+1 < len(trigger.Content); i += 2 {
			name, value := trigger.Content[i], trigger.Content[i+1]
			if name.Value == "push" {
				value = filtered(value)
			}
			events.Content = append(events.Content, name, value)
		}
	default:
		return nil
	}
	return events
}

// filtered answers a push value that carries branchFilter, in block style.
func filtered(push *yaml.Node) *yaml.Node {
	branches := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle, Content: []*yaml.Node{
		{Kind: yaml.ScalarNode, Value: "**", Style: yaml.SingleQuotedStyle},
	}}
	filter := []*yaml.Node{{Kind: yaml.ScalarNode, Value: "branches"}, branches}
	if push == nil || push.Kind != yaml.MappingNode {
		return &yaml.Node{Kind: yaml.MappingNode, Content: filter}
	}
	out := *push
	out.Style = 0
	out.Content = append(append([]*yaml.Node{}, push.Content...), filter...)
	return &out
}

// lastTriggerLine answers the last line the on: value covers. The line before the
// next top-level key, less the blank and comment lines that close the gap.
func lastTriggerLine(fileLines []string, root, key *yaml.Node) int {
	end := len(fileLines) - 1
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i] == key && i+2 < len(root.Content) {
			end = root.Content[i+2].Line - 2
		}
	}
	for end > key.Line-1 {
		trimmed := strings.TrimSpace(fileLines[end])
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			break
		}
		end--
	}
	return end
}
