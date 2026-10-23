// Shell selection for a step that names no shell of its own.

package workflow

import (
	"strings"

	yaml "go.yaml.in/yaml/v3"
	"mvdan.cc/sh/v3/syntax"
)

// runsPowerShell reports whether a step that names no shell of its own runs
// PowerShell: the job sets no default shell and runs on a Windows runner.
func runsPowerShell(job *yaml.Node, shell string) bool {
	on := mappingValue(job, "runs-on")
	if on == nil || shell != "" {
		return false
	}
	var text strings.Builder
	walk(on, func(n *yaml.Node) { text.WriteString(n.Value) })
	return strings.Contains(strings.ToLower(text.String()), "windows")
}

func defaultShell(node *yaml.Node) string {
	if shell := defaultShellNode(node); shell != nil {
		return shell.Value
	}
	return ""
}

// defaultShellNode answers the value of defaults.run.shell, or nil.
func defaultShellNode(node *yaml.Node) *yaml.Node {
	return mappingValue(mappingValue(mappingValue(node, "defaults"), "run"), "shell")
}

// bashLike reports whether a shell value runs a POSIX shell.
func bashLike(shell string) bool {
	if shell == "" {
		return true
	}
	first := strings.Fields(shell)[0]
	return first == "bash" || first == "sh"
}

func readEnvStep(step *yaml.Node, container, windows bool, fileLines []string, spans map[int]lineSpan) (envStep, bool) {
	run := mappingValue(step, "run")
	if run == nil || run.Kind != yaml.ScalarNode {
		return envStep{}, false
	}
	if shell := mappingValue(step, "shell"); shell != nil {
		if !bashLike(shell.Value) {
			return envStep{}, false
		}
	} else if windows {
		return envStep{}, false
	}
	es := envStep{step: step, container: container, script: run.Value}
	at, ok := scalarLines(run, fileLines, spans)
	if !ok {
		return envStep{}, false
	}
	es.lineNos, es.file, es.prefix = at.lineNos, at.file, at.prefix
	parsed, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(es.script), "")
	if err != nil {
		return envStep{}, false
	}
	es.parsed = parsed
	return es, true
}
