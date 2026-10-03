package slopfix

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/wow-look-at-my/slopfix/commentfix"
)

// IDPackageScripts is a package.json with a scripts section. A justfile holds the commands instead.
const IDPackageScripts = "repo/package-scripts"

// packageScripts reports each package.json under root that carries a scripts
// key. Writing, it moves the scripts into a justfile first, and names each file
// it changed. A package.json that does not parse stays a finding, because no
// rewrite can read it.
func packageScripts(root string, writing bool) ([]TreeFinding, []string, error) {
	var out []TreeFinding
	var changed []string
	isManifest := func(path string) bool { return filepath.Base(path) == "package.json" }
	for _, path := range commentfix.TreeFilesMatching(root, isManifest) {
		content, err := os.ReadFile(path)
		if err != nil {
			return out, changed, err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return out, changed, err
		}
		var manifest map[string]json.RawMessage
		if err := json.Unmarshal(content, &manifest); err != nil {
			out = append(out, repoFinding(rel, IDPackageScripts, "this package.json does not parse, so no rule can read it", err.Error()))
			continue
		}
		if _, ok := manifest["scripts"]; !ok {
			continue
		}
		if writing {
			wrote, err := moveScripts(path, content)
			if err != nil {
				return out, changed, err
			}
			changed = append(changed, wrote...)
			continue
		}
		finding := repoFinding(rel, IDPackageScripts, "a package.json holds a scripts section",
			"Move each command into a justfile recipe, and delete the scripts key. `slopfix fix` does this.")
		finding.Line = scriptsLine(content)
		out = append(out, finding)
	}
	return out, changed, nil
}

func scriptsLine(content []byte) int {
	for i, line := range bytes.Split(content, []byte("\n")) {
		if bytes.HasPrefix(bytes.TrimSpace(line), []byte(`"scripts"`)) {
			return i + 1
		}
	}
	return 1
}
