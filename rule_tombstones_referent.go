package slopfix

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// tombstones/name-nothing-in-the-repository-defines: a comment that names a
// symbol no file of the repository holds. The repair strips the comment line,
// or cuts the sentence that holds the name.
func init() {
	RegisterRule(RuleSpec{
		ID:       tombstones.IDDeadReferent,
		Category: RuleTombstones,
		Detect:   detectDeadReferent,
		Autofix:  autofixDeadReferent,
		Cases: []RuleCase{{
			Name:  tombstones.IDDeadReferent,
			Path:  "main.go",
			Text:  "package main\n\n// parseLegacyFlag reads the input.\nfunc main() {}\n",
			Files: map[string]string{"util.go": "package main\n\nfunc readInput() {}\n"},
		}, {
			// The inline comment of a call names a dead parameter, and the call around it names members of an imported module.
			Name: "inline block comment in a TypeScript call",
			Path: "src/comments.ts",
			Text: "import * as ts from 'typescript';\n\n" +
				"export function commentOnlyLines(script: string): number {\n" +
				"\tconst scanner = ts.createScanner(ts.ScriptTarget.Latest, /* skipTrivia */ false, ts.LanguageVariant.Standard, script);\n" +
				"\treturn scanner.getTokenStart();\n" +
				"}\n",
			Files: map[string]string{"src/other.ts": "export const other = 1;\n"},
		}, {
			// A string literal before a trailing comment is code, and only the comment's own words name the dead symbol.
			Name: "trailing comment after a TypeScript string",
			Path: "src/highlight.ts",
			Text: "export const PALETTE = new Map<string, string>([\n" +
				"\t['built_in', fg('#ffa657')], // built-in types (string, number) and globals (console, Math). See paletteFor.\n" +
				"]);\n",
			Files: map[string]string{"src/other.ts": "export function fg(hex: string): string {\n\treturn hex;\n}\n"},
		}},
	})
}

// detectDeadReferent answers every name a comment of the repository carries
// that no other file holds.
func detectDeadReferent(c RuleCase) []ste.Finding {
	var out []ste.Finding
	for _, path := range fixtureFiles(c) {
		for _, hit := range tombstones.DeadReferentHits(path, readFixture(path)) {
			out = append(out, ste.Finding{
				Line:   hit.LineNo,
				ID:     tombstones.IDDeadReferent,
				Rule:   hit.Tell,
				Detail: hit.Phrase,
				Fix:    "Cut the sentence that names it. `slopfix fix` does this.",
			})
		}
	}
	return out
}

// autofixDeadReferent strips the comment line or cuts the sentence that holds
// the name.
func autofixDeadReferent(c RuleCase) RuleCase {
	return caseAutofix(c, tombstones.IDDeadReferent)
}
