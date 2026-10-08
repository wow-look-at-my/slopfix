package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// repo/json: a JSON file that does not parse, or that breaks the schema its
// $schema names.
func init() {
	RegisterRule(RuleSpec{
		ID:       IDJSON,
		Category: RuleRepo,
		Detect:   detectJSON,
		Autofix:  autofixJSON,
		Cases: []RuleCase{
			{Name: IDJSON, Files: map[string]string{"bad.json": "{\"a\": 1\n"}},
			{Name: IDJSON + "/missing-schema", Files: map[string]string{"cfg.json": "{\"$schema\": \"./missing.schema.json\", \"a\": 1}\n"}},
		},
	})
}

// detectJSON answers every JSON document the rule reports under root.
func detectJSON(c RuleCase) []ste.Finding { return treeFindings(c, IDJSON) }

// autofixJSON closes the brackets the document leaves open, drops a comma before a closing bracket.
func autofixJSON(c RuleCase) RuleCase { return treeAutofix(c, IDJSON) }
