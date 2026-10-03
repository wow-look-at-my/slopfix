package slopfix

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// script is a scripts entry, in the order the manifest lists it.
type script struct {
	name, command string
}

// moveScripts writes each scripts entry of the manifest at path as a recipe in
// the justfile beside it, and deletes the scripts key. It answers the files it wrote.
func moveScripts(path string, content []byte) ([]string, error) {
	start, end, scripts, err := scriptsSpan(content)
	if err != nil {
		return nil, err
	}
	justfile := filepath.Join(filepath.Dir(path), "justfile")
	existing, err := os.ReadFile(justfile)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.WriteFile(justfile, []byte(justRecipes(string(existing), scripts)), 0o644); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	manifest := append(append([]byte{}, content[:start]...), content[end:]...)
	if err := os.WriteFile(path, manifest, info.Mode().Perm()); err != nil {
		return nil, err
	}
	return []string{path, justfile}, nil
}

// scriptsSpan answers the bytes the scripts member covers, with the comma that
// joins it to a neighbour, and the scripts it holds.
func scriptsSpan(content []byte) (start, end int, scripts []script, err error) {
	dec := json.NewDecoder(bytes.NewReader(content))
	if _, err := dec.Token(); err != nil {
		return 0, 0, nil, err
	}
	first := true
	for dec.More() {
		before := int(dec.InputOffset())
		key, err := dec.Token()
		if err != nil {
			return 0, 0, nil, err
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return 0, 0, nil, err
		}
		after := int(dec.InputOffset())
		if key != "scripts" {
			first = false
			continue
		}
		if scripts, err = scriptsOf(raw); err != nil {
			return 0, 0, nil, err
		}
		if !first || !dec.More() {
			// The comma before the member goes with it. A sole member has none.
			return before, after, scripts, nil
		}
		comma := bytes.IndexByte(content[after:], ',')
		if comma < 0 {
			return 0, 0, nil, fmt.Errorf("package.json: no comma after the scripts member")
		}
		// The first member takes the comma after it instead.
		return before, after + comma + 1, scripts, nil
	}
	return 0, 0, nil, fmt.Errorf("package.json: no scripts member")
}

// scriptsOf reads the scripts object in order. A value that is not a string
// is written as its JSON text.
func scriptsOf(raw json.RawMessage) ([]script, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("package.json: scripts is not an object")
	}
	var out []script
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, err
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		command := string(value)
		var text string
		if json.Unmarshal(value, &text) == nil {
			command = text
		}
		out = append(out, script{name: fmt.Sprint(key), command: command})
	}
	return out, nil
}

// recipeChar is a character a recipe name cannot hold.
var recipeChar = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

// npmRun is a call to another script, which becomes a call to its recipe.
var npmRun = regexp.MustCompile(`\bnpm run(?:-script)? ([A-Za-z0-9_:.-]+)`)

// recipeName turns a script name into a recipe name.
func recipeName(name string) string {
	out := strings.Trim(recipeChar.ReplaceAllString(name, "-"), "-")
	if out == "" || out[0] >= '0' && out[0] <= '9' {
		out = "script-" + out
	}
	return out
}

// pathLine puts the package's own binaries first, as npm run does.
const pathLine = `export PATH := justfile_directory() + "/node_modules/.bin:" + env_var("PATH")`

// justRecipes answers the justfile with a recipe for each script appended. A
// pre or post script runs around its own, as npm runs it. A recipe name the
// justfile holds already is not written twice.
func justRecipes(existing string, scripts []script) string {
	taken := set.New[string]()
	for _, line := range strings.Split(existing, "\n") {
		if name, _, ok := strings.Cut(line, ":"); ok && name != "" && recipeChar.FindString(name) == "" {
			taken.Add(name)
		}
	}
	names := set.New[string]()
	for _, s := range scripts {
		names.Add(s.name)
	}
	var b strings.Builder
	b.WriteString(existing)
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		b.WriteString("\n")
	}
	if !strings.Contains(existing, "node_modules/.bin") {
		if existing != "" {
			b.WriteString("\n")
		}
		b.WriteString("# The package's own binaries come first on PATH, as npm run puts them.\n")
		b.WriteString(pathLine + "\n")
	}
	for _, s := range scripts {
		name := recipeName(s.name)
		if taken.Contains(name) {
			continue
		}
		taken.Add(name)
		command := npmRun.ReplaceAllStringFunc(s.command, func(call string) string {
			return "just " + recipeName(npmRun.FindStringSubmatch(call)[1])
		})
		command = strings.ReplaceAll(command, "{{", `{{"{{"}}`)
		head := name + ":"
		if names.Contains("pre" + s.name) {
			head += " " + recipeName("pre"+s.name)
		}
		fmt.Fprintf(&b, "\n# The npm script %s.\n%s\n\t%s\n", s.name, head, command)
		if names.Contains("post" + s.name) {
			fmt.Fprintf(&b, "\tjust %s\n", recipeName("post"+s.name))
		}
	}
	return b.String()
}
