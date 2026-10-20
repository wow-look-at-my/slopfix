package slopfix

import (
	"github.com/wow-look-at-my/slopfix/goembed"
	"github.com/wow-look-at-my/slopfix/ste"
)

// go/foreign-source: a Go string that holds the source of another language.
// The repair moves the text into a file of its own type and reads it with
// //go:embed. A string in a test moves under testdata.
func init() {
	RegisterRule(RuleSpec{
		ID:       goembed.ID,
		Category: RuleGo,
		Detect:   detectForeignSource,
		Autofix:  autofixForeignSource,
		Cases: []RuleCase{
			{
				Name: "shell-stub-in-a-test",
				Path: "stub_test.go",
				Text: "package x\n\nconst slopfixStub = `#!/bin/sh\n" +
					"if [ -n \"$SLOPFIX_STUB_MISSING\" ]; then echo \"sh: slopfix: not found\" >&2; exit 127; fi\n" +
					"shift\nif [ \"$1\" = --only ]; then\n  if grep -n SLOP \"$3\"; then exit 1; fi\n  exit 0\nfi\n`\n",
				Want: "package x\n\nimport _ \"embed\"\n\n//go:embed testdata/slopfix-stub.sh\nvar slopfixStub string\n",
			},
			{
				Name: "sql-in-a-function",
				Path: "store.go",
				Text: "package store\n\nimport \"database/sql\"\n\nfunc migrate(db *sql.DB) error {\n" +
					"\t_, err := db.Exec(\"CREATE TABLE runs (\\n\\tid INTEGER PRIMARY KEY,\\n\\tname TEXT NOT NULL\\n);\\n\")\n" +
					"\treturn err\n}\n",
				Want: "package store\n\nimport _ \"embed\"\n\nimport \"database/sql\"\n\nfunc migrate(db *sql.DB) error {\n" +
					"\t_, err := db.Exec(migrateSql)\n\treturn err\n}\n\n//go:embed migrate.sql\nvar migrateSql string\n",
			},
			{
				Name:      "help-text-is-prose",
				Path:      "help.go",
				Text:      "package help\n\nconst usage = `Usage: tool [flags]\n\nRun the tool.\nIt reads the tree.\n`\n",
				Unchanged: true,
			},
		},
	})
}

// detectForeignSource answers every Go string that holds another language.
func detectForeignSource(c RuleCase) []ste.Finding {
	return caseFindings(c, goembed.ID)
}

// autofixForeignSource moves each such string into an embedded file, as
// `slopfix fix` does on the tree.
func autofixForeignSource(c RuleCase) RuleCase {
	return treeAutofix(c, goembed.ID)
}
