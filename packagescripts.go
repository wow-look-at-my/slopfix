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

// packageScripts reports each package.json under root that carries a scripts key.
func packageScripts(root string) ([]TreeFinding, error) {
	var out []TreeFinding
	isManifest := func(path string) bool { return filepath.Base(path) == "package.json" }
	for _, path := range commentfix.TreeFilesMatching(root, isManifest) {
		content, err := os.ReadFile(path)
		if err != nil {
			return out, err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return out, err
		}
		var manifest map[string]json.RawMessage
		if err := json.Unmarshal(content, &manifest); err != nil {
			out = append(out, repoFinding(rel, IDPackageScripts, "this package.json does not parse, so no rule can read it", err.Error()))
			continue
		}
		if _, ok := manifest["scripts"]; !ok {
			continue
		}
		finding := repoFinding(rel, IDPackageScripts, "a package.json holds a scripts section",
			"Move each command into a justfile recipe, and delete the scripts key.")
		finding.Line = scriptsLine(content)
		out = append(out, finding)
	}
	return out, nil
}

func scriptsLine(content []byte) int {
	for i, line := range bytes.Split(content, []byte("\n")) {
		if bytes.HasPrefix(bytes.TrimSpace(line), []byte(`"scripts"`)) {
			return i + 1
		}
	}
	return 1
}
