// hostjob.go answers which jobs need the host machine itself. Such a job
// cannot run in a container of the org runner fleet, so it keeps its
// GitHub-hosted label.
package workflow

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/rules"
	"github.com/wow-look-at-my/slopfix/shellwalk"
	"github.com/wow-look-at-my/slopfix/table"
	"mvdan.cc/sh/v3/syntax"
	yaml "go.yaml.in/yaml/v3"
)

// hostTable is what rules/ says for="host-runner".
var hostTable = table.MustLoad(rules.FS, "host-runner")

// HostedMarker opens the runs-on comment that keeps a job on its GitHub-hosted label. A reason must follow it.
const HostedMarker = "slopfix: hosted"

// hostedOptOut answers the reason a runs-on comment gives for a GitHub-hosted
// label, and whether the comment carries the marker at all.
func hostedOptOut(comment string) (string, bool) {
	text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(comment), "#"))
	rest, found := strings.CutPrefix(text, HostedMarker)
	if !found {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

// needsHost answers the first sign that a job needs the host, or an empty string.
func needsHost(job *yaml.Node) string {
	steps := mappingValue(job, "steps")
	if steps == nil || steps.Kind != yaml.SequenceNode {
		return ""
	}
	for _, step := range steps.Content {
		if uses := mappingValue(step, "uses"); uses != nil {
			if why := hostUses(strings.ToLower(strings.TrimSpace(uses.Value))); why != "" {
				return why
			}
		}
		if run := mappingValue(step, "run"); run != nil && run.Kind == yaml.ScalarNode {
			if why := hostScript(run.Value); why != "" {
				return why
			}
		}
	}
	return ""
}

// hostUses answers why the action that uses names needs the host, or an empty string.
func hostUses(uses string) string {
	for _, prefix := range hostTable.List("vm-action") {
		if strings.HasPrefix(uses, prefix) {
			return "uses: " + uses
		}
	}
	name, _, _ := strings.Cut(uses, "@")
	name = name[strings.LastIndexByte(name, '/')+1:]
	for _, action := range hostTable.List("host-action") {
		if name == action {
			return "uses: " + uses
		}
	}
	return ""
}

// hostScript answers the first command of a script that needs the host, or an
// empty string. A script the shell parser cannot read yields nothing.
func hostScript(script string) string {
	marked := expression.ReplaceAllString(script, "slopfix_expression")
	parsed, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(marked), "")
	if err != nil {
		return ""
	}
	var why string
	syntax.Walk(parsed, func(node syntax.Node) bool {
		call, ok := node.(*syntax.CallExpr)
		if !ok || why != "" {
			return why == ""
		}
		program, args := shellwalk.ResolveProgram(shellwalk.Words(call.Args))
		if hostCommand(program, args) {
			why = "run: " + program
			for _, arg := range args {
				why += " " + arg.Text
			}
		}
		return why == ""
	})
	return why
}

// hostCommand reports whether a program with its arguments needs the host.
func hostCommand(program string, args []shellwalk.Word) bool {
	if program == "" {
		return false
	}
	for _, name := range hostTable.List("host-program") {
		if program == name {
			return true
		}
	}
	for _, prefix := range hostTable.List("host-program-prefix") {
		if strings.HasPrefix(program, prefix) {
			return true
		}
	}
	installs := false
	for _, name := range hostTable.List("package-program") {
		installs = installs || program == name
	}
	for _, arg := range args {
		for _, prefix := range hostTable.List("host-argument") {
			if strings.HasPrefix(arg.Text, prefix) {
				return true
			}
		}
		if !installs {
			continue
		}
		for _, prefix := range hostTable.List("kernel-package") {
			if strings.HasPrefix(arg.Text, prefix) {
				return true
			}
		}
	}
	return false
}
