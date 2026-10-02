package workflow

import (
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
		Fix:  "Add `branches: ['**']` under `push:`, or `tags-ignore: ['**']`, to keep the workflow on branch pushes.",
	}
}

// mappingKey answers the key node itself, so a finding can point at it.
func mappingKey(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i]
		}
	}
	return nil
}
