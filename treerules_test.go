package slopfix_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix"
)

// gitRepo builds a real repository and stages every file, because the binary
// rule reads what git tracks and the copy rule reads gitattributes.
func gitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	return root
}

// checkOnly runs the repository rule a test is about.
func checkOnly(root, id string) []slopfix.TreeFinding {
	return slopfix.CheckTreeWith(root, slopfix.Request{Rules: []slopfix.Rule{slopfix.RuleRepo}, IDs: []string{id}}).Findings
}

func pathsOf(findings []slopfix.TreeFinding) []string {
	var out []string
	for _, f := range findings {
		out = append(out, filepath.ToSlash(f.Path)+" "+f.ID)
	}
	return out
}

const elf = "\x7fELF\x02\x01\x01\x00rest of the binary"

func TestATrackedExecutableIsReportedAndFixDeletesIt(t *testing.T) {
	root := gitRepo(t, map[string]string{"bin/tool": elf, "main.go": "package main\n", "app.exe": "MZ\x90\x00"})
	assert.ElementsMatch(t, []string{"app.exe repo/binary", "bin/tool repo/binary"}, pathsOf(checkOnly(root, slopfix.IDBinary)))

	slopfix.FixTreeWith(root, slopfix.Request{Rules: []slopfix.Rule{slopfix.RuleRepo}, IDs: []string{slopfix.IDBinary}})
	assert.NoFileExists(t, filepath.Join(root, "bin", "tool"))
	assert.NoFileExists(t, filepath.Join(root, "app.exe"))
	assert.FileExists(t, filepath.Join(root, "main.go"))
	assert.Empty(t, checkOnly(root, slopfix.IDBinary))
}

func TestAnUntrackedExecutableIsNotTheRepositorys(t *testing.T) {
	root := gitRepo(t, map[string]string{".gitignore": "build/\n"})
	require.NoError(t, os.MkdirAll(filepath.Join(root, "build"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "build", "tool"), []byte(elf), 0o755))
	assert.Empty(t, checkOnly(root, slopfix.IDBinary))
}

func TestATreeGitCannotListIsReadFromDisk(t *testing.T) {
	root := gitRoot(t, map[string]string{"tool": elf})
	assert.Equal(t, []string{"tool repo/binary"}, pathsOf(checkOnly(root, slopfix.IDBinary)))
}

// script is long enough that a single added comment line keeps it over the share.
var script = func() string {
	lines := []string{"import { run } from './run';"}
	for _, step := range []string{"read", "parse", "check", "build", "write", "send", "log", "close", "sync", "wait"} {
		lines = append(lines, "export function "+step+"(n: number) {", "  return run('"+step+"', n);", "}")
	}
	return strings.Join(lines, "\n") + "\n"
}()

func TestTwoFilesOfOneNameThatMatchAreReported(t *testing.T) {
	root := gitRepo(t, map[string]string{
		"a/run.ts":   script,
		"b/run.ts":   "// The copy differs in a comment alone.\n" + script,
		"c/run.ts":   "export const other = true;\n",
		"d/other.ts": script,
	})
	findings := checkOnly(root, slopfix.IDNearDuplicate)
	require.Len(t, findings, 1)
	assert.Equal(t, "b/run.ts", filepath.ToSlash(findings[0].Path))
	assert.Contains(t, findings[0].Rule, "a/run.ts")
}

// Each copy is reported once, so the report grows with the copies and not with the pairs.
func TestEachCopyIsReportedOnce(t *testing.T) {
	root := gitRepo(t, map[string]string{"a/run.ts": script, "b/run.ts": script, "c/run.ts": script})
	assert.Equal(t, []string{"b/run.ts repo/near-duplicate", "c/run.ts repo/near-duplicate"}, pathsOf(checkOnly(root, slopfix.IDNearDuplicate)))
}

// No attribute exempts a copy. The only answer to a copy is one file.
func TestNoAttributeExemptsACopy(t *testing.T) {
	root := gitRepo(t, map[string]string{
		".gitattributes":  "copies/** slopfix-copy\n",
		"copies/a/run.ts": script,
		"copies/b/run.ts": script,
	})
	assert.Equal(t, []string{"copies/b/run.ts repo/near-duplicate"}, pathsOf(checkOnly(root, slopfix.IDNearDuplicate)))
}

func TestFilesEachDirectoryNeedsAreNotCopies(t *testing.T) {
	manifest := "{\n  \"name\": \"x\",\n  \"private\": true\n}\n"
	root := gitRepo(t, map[string]string{"a/package.json": manifest, "b/package.json": manifest, "a/.keep": "", "b/.keep": ""})
	assert.Empty(t, checkOnly(root, slopfix.IDNearDuplicate))
}

func TestJSONThatDoesNotParseIsReportedAtItsLine(t *testing.T) {
	root := gitRepo(t, map[string]string{
		"ok.json":     "{\n  // A comment is allowed.\n  \"a\": 1,\n}\n",
		"broken.json": "{\n  \"a\": 1\n  \"b\": 2\n}\n",
	})
	findings := checkOnly(root, slopfix.IDJSON)
	require.Len(t, findings, 1)
	assert.Equal(t, "broken.json", findings[0].Path)
	assert.Equal(t, 3, findings[0].Line)
}

const objectSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["name"],
  "properties": {"$schema": {"type": "string"}, "name": {"type": "string"}},
  "additionalProperties": false
}
`

func TestJSONIsHeldToTheSchemaItNames(t *testing.T) {
	root := gitRepo(t, map[string]string{
		"schema/thing.schema.json": objectSchema,
		"good/thing.json":          `{"$schema": "../schema/thing.schema.json", "name": "x"}`,
		"bad/thing.json":           `{"$schema": "../schema/thing.schema.json", "nmae": "x"}`,
		"lost/thing.json":          `{"$schema": "../nowhere.schema.json", "name": "x"}`,
	})
	findings := checkOnly(root, slopfix.IDJSON)
	assert.ElementsMatch(t, []string{"bad/thing.json repo/json", "lost/thing.json repo/json"}, pathsOf(findings))
	for _, f := range findings {
		if strings.HasPrefix(filepath.ToSlash(f.Path), "bad/") {
			assert.Contains(t, f.Fix, "name")
		}
	}
}

// A remote schema is never fetched, so a document that names one gets the parse check alone.
func TestARemoteSchemaIsNotFetched(t *testing.T) {
	root := gitRepo(t, map[string]string{
		"thing.json":  `{"$schema": "https://schemas.invalid/thing.schema.json", "anything": true}`,
		"broken.json": `{"$schema": "https://schemas.invalid/thing.schema.json",}}`,
		"thing.xml":   xmlDoc(`<rule xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="https://schemas.invalid/rule.xsd"/>`),
	})
	assert.Equal(t, []string{"broken.json repo/json"}, pathsOf(checkOnly(root, slopfix.IDJSON)))
	assert.Empty(t, checkOnly(root, slopfix.IDXML))
}

func TestASchemaIsHeldToTheMetaSchema(t *testing.T) {
	root := gitRepo(t, map[string]string{"broken.schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": 7}`})
	assert.Equal(t, []string{"broken.schema.json repo/json"}, pathsOf(checkOnly(root, slopfix.IDJSON)))
}

const xsd = `<?xml version="1.1" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="rule">
    <xs:complexType>
      <xs:attribute name="id" type="xs:string" use="required"/>
    </xs:complexType>
  </xs:element>
</xs:schema>
`

func xmlDoc(body string) string {
	return `<?xml version="1.1" encoding="UTF-8"?>` + "\n" + body + "\n"
}

func TestXMLIsHeldToTheSchemaItNames(t *testing.T) {
	located := `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="rule.xsd"`
	root := gitRepo(t, map[string]string{
		"rule.xsd":      xsd,
		"good.xml":      xmlDoc(`<rule ` + located + ` id="a"/>`),
		"bad.xml":       xmlDoc(`<rule ` + located + `/>`),
		"unnamed.xml":   xmlDoc(`<rule id="a"/>`),
		"version.xml":   `<?xml version="1.0"?><rule ` + located + ` id="a"/>`,
		"malformed.xml": xmlDoc(`<rule ` + located + ` id="a">`),
	})
	assert.ElementsMatch(t, []string{
		"bad.xml repo/xml", "unnamed.xml repo/xml", "version.xml repo/xml", "malformed.xml repo/xml",
	}, pathsOf(checkOnly(root, slopfix.IDXML)))
}

// The rule fails a document on its own: naming another rule leaves it out.
func TestNamingAnotherRuleLeavesTheDocumentRulesOut(t *testing.T) {
	root := gitRepo(t, map[string]string{"broken.json": "{", "tool": elf})
	assert.Equal(t, []string{"tool repo/binary"}, pathsOf(checkOnly(root, slopfix.IDBinary)))
}
