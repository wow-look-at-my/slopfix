package slopfix_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	yaml "go.yaml.in/yaml/v3"
)

// scriptActionUses matches a step that runs a TypeScript or github-script script.
var scriptActionUses = regexp.MustCompile(`(^|[/@])typescript([@#]|$)|github-script@`)

// templateExpr matches one ${{ }} expression.
var templateExpr = regexp.MustCompile(`\$\{\{.*?\}\}`)

// scriptGlobals declares what the script actions put in scope, as far as the
// cases use it.
const scriptGlobals = "declare const core: any;\ndeclare const github: any;\ndeclare const context: any;\n" +
	"declare const process: { env: Record<string, string | undefined> };\n"

// scriptConfig type-checks script.ts the way the typescript action does: strict, ES2022, no ambient types.
const scriptConfig = `{"compilerOptions":{"strict":true,"noEmit":true,"target":"ES2022","lib":["ES2022"],"types":[],"skipLibCheck":true},"files":["script.ts"]}`

// actionScripts answers the with.script of every script-action step in a YAML text.
func actionScripts(text string) []string {
	var doc yaml.Node
	if yaml.Unmarshal([]byte(text), &doc) != nil {
		return nil
	}
	var out []string
	var visit func(n *yaml.Node)
	visit = func(n *yaml.Node) {
		if n.Kind == yaml.MappingNode {
			var uses, with *yaml.Node
			for i := 0; i+1 < len(n.Content); i += 2 {
				switch n.Content[i].Value {
				case "uses":
					uses = n.Content[i+1]
				case "with":
					with = n.Content[i+1]
				}
			}
			if uses != nil && with != nil && scriptActionUses.MatchString(uses.Value) && with.Kind == yaml.MappingNode {
				for i := 0; i+1 < len(with.Content); i += 2 {
					if with.Content[i].Value == "script" {
						out = append(out, with.Content[i+1].Value)
					}
				}
			}
		}
		for _, c := range n.Content {
			visit(c)
		}
	}
	visit(&doc)
	return out
}

// renderScript writes a script as the runner hands it over: each toJSON
// expression becomes the string literal value, and any other expression its text.
func renderScript(script, value string) string {
	return templateExpr.ReplaceAllStringFunc(script, func(expr string) string {
		if strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(expr, "${{")), "toJSON(") {
			return value
		}
		return "v"
	})
}

// typeCheck runs tsc on a rendered script inside an async function, which
// admits the top-level await and return the script actions allow.
func typeCheck(t *testing.T, tsc, script string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	body := scriptGlobals + "async function main() {\n" + script + "\n}\nvoid main;\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "script.ts"), []byte(body), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(scriptConfig), 0o644))
	out, err := exec.Command(tsc, "-p", dir).CombinedOutput()
	return body + "\n" + string(out), err
}

// Every script a rule's autofix writes into a script-action step passes tsc,
// with the literal empty and with it set. A repair that leaves a string
// literal where tsc rejects one breaks the step it repaired.
func TestRepairedActionScriptsTypeCheck(t *testing.T) {
	tsc, err := exec.LookPath("tsc")
	require.NoError(t, err, "this test runs the real TypeScript compiler; install it with `npm install -g typescript`")
	checked := 0
	for _, rule := range slopfix.AllRuleSpecs() {
		for _, raw := range rule.Cases {
			if !strings.Contains(raw.Text, "script:") {
				continue
			}
			fixture, err := slopfix.Materialize(t.TempDir(), raw)
			require.NoError(t, err)
			fixed := rule.Autofix(fixture)
			for path, text := range caseTexts(t, fixed) {
				for _, script := range actionScripts(text) {
					for _, value := range []string{`"v"`, `""`} {
						out, err := typeCheck(t, tsc, renderScript(script, value))
						assert.NoError(t, err, "%s: case %q: %s: tsc rejects the repaired script with %s:\n%s", rule.ID, raw.Name, path, value, out)
					}
					checked++
				}
			}
		}
	}
	assert.NotZero(t, checked, "no case carries a script-action step to type-check")
}
