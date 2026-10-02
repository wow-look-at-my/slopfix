package workflow

import (
	"fmt"
	"slices"
	"sort"

	yaml "go.yaml.in/yaml/v3"
)

// Levels are the permission levels in rising order. write grants read.
var Levels = []string{"none", "read", "write"}

// Grant is the level a job holds for one permission, and the block it came from.
type Grant struct {
	Level string `json:"level"`
	// Source is "job", "workflow" or "default".
	Source string `json:"source"`
}

// Covers reports whether the grant is at least the wanted level.
func (g Grant) Covers(want string) bool {
	return slices.Index(Levels, g.Level) >= slices.Index(Levels, want)
}

// Permission resolves what a job in the workflow holds for one permission. The
// job's own block wins. With none, the workflow block applies. With neither,
// the level is the repository default, which the file cannot state, so it
// reads as none.
func Permission(content, job, permission string) (Grant, error) {
	var doc struct {
		Permissions any `yaml:"permissions"`
		Jobs        map[string]struct {
			Permissions any `yaml:"permissions"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return Grant{}, fmt.Errorf("the workflow does not parse: %w", err)
	}
	spec, ok := doc.Jobs[job]
	if !ok {
		names := make([]string, 0, len(doc.Jobs))
		for name := range doc.Jobs {
			names = append(names, name)
		}
		sort.Strings(names)
		return Grant{}, fmt.Errorf("the workflow names no job %q. It has: %v", job, names)
	}
	for _, block := range []struct {
		source string
		value  any
	}{{"job", spec.Permissions}, {"workflow", doc.Permissions}} {
		level, set, err := levelIn(block.value, permission)
		if err != nil {
			return Grant{}, fmt.Errorf("the %s permissions block: %w", block.source, err)
		}
		if set {
			return Grant{Level: level, Source: block.source}, nil
		}
	}
	return Grant{Level: "none", Source: "default"}, nil
}

// levelIn reads one permissions block. An absent block is unset. A mapping
// that omits the permission sets it to none, which is what GitHub does.
func levelIn(block any, permission string) (string, bool, error) {
	switch b := block.(type) {
	case nil:
		return "", false, nil
	case string:
		switch b {
		case "write-all":
			return "write", true, nil
		case "read-all":
			return "read", true, nil
		}
		return "", false, fmt.Errorf("%q is neither read-all nor write-all", b)
	case map[string]any:
		value, ok := b[permission]
		if !ok {
			return "none", true, nil
		}
		level, isString := value.(string)
		if !isString || !slices.Contains(Levels, level) {
			return "", false, fmt.Errorf("%s is %v, not read, write or none", permission, value)
		}
		return level, true, nil
	}
	return "", false, fmt.Errorf("it is not a mapping")
}
