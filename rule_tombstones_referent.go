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
			// The session id in a URL is a name no file holds, so the line that carries it goes.
			Name: "an identifier inside a URL", Path: "msg.txt",
			Text:  "The gate reads the file.\n\nClaude-Session: https://claude.ai/code/session_013ERSrxHkx1TwBeoQVMojfm\n",
			Files: map[string]string{"util.go": "package main\n\nfunc readInput() {}\n"},
		}, {
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
		}, {
			Name: tombstones.IDDeadReferent + " on the closer line of a JSDoc block",
			Path: "plain.js",
			Text: "/** Report whether the value is plain.\n * A plain value never equals `value === thisOperand` at runtime. */\nexport function isPlain(value) {\n\treturn value === null;\n}\n",
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
