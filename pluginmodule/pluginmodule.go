// Package pluginmodule carries the marketplace plugin's hooks module. The
// module calls this binary, so both ship from one build and cannot drift.
package pluginmodule

import (
	"embed"
	"os"
	"path/filepath"
)

//go:embed register.ts register.test.ts
var files embed.FS

// Names are the files Write puts in the plugin's hooks directory.
var Names = []string{"register.ts", "register.test.ts"}

// Write puts the module and its tests in dir, which it creates.
func Write(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, name := range Names {
		data, err := files.ReadFile(name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Source answers the module's own text.
func Source() string {
	data, _ := files.ReadFile("register.ts")
	return string(data)
}
