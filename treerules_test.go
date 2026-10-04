package slopfix_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// Git LFS stores a pointer in the index and writes the binary into the
// checkout. Git itself holds no executable, so fix must keep the file.
func TestAGitLFSExecutableIsKept(t *testing.T) {
	pointer := "version https://git-lfs.github.com/spec/v1\noid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize 31\n"
	root := gitRepo(t, map[string]string{
		".gitattributes":      "bin/tool filter=lfs diff=lfs merge=lfs -text\n",
		"bin/tool":            pointer,
		"testdata/fixture.so": elf,
	})
	require.NoError(t, os.WriteFile(filepath.Join(root, "bin", "tool"), []byte(elf), 0o755))
	assert.Equal(t, []string{"testdata/fixture.so repo/binary"}, pathsOf(checkOnly(root, slopfix.IDBinary)))

	slopfix.FixTreeWith(root, slopfix.Request{Rules: []slopfix.Rule{slopfix.RuleRepo}, IDs: []string{slopfix.IDBinary}})
	assert.FileExists(t, filepath.Join(root, "bin", "tool"))
	assert.NoFileExists(t, filepath.Join(root, "testdata", "fixture.so"))
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

// A symlink is the file it names, so it is the answer to a copy and never a copy itself.
func TestASymlinkIsNotACopy(t *testing.T) {
	root := gitRepo(t, map[string]string{"a/run.ts": script, "b/.keep": ""})
	require.NoError(t, os.Symlink("../a/run.ts", filepath.Join(root, "b", "run.ts")))
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Empty(t, checkOnly(root, slopfix.IDNearDuplicate))
}

func TestFilesEachDirectoryNeedsAreNotCopies(t *testing.T) {
	manifest := "{\n  \"name\": \"x\",\n  \"private\": true\n}\n"
	root := gitRepo(t, map[string]string{"a/package.json": manifest, "b/package.json": manifest, "a/.keep": "", "b/.keep": ""})
	assert.Empty(t, checkOnly(root, slopfix.IDNearDuplicate))
}

// ts0, pnpm and each read the file in the directory they build. The
// repository-scripts repair writes a justfile beside each manifest.
func TestBuildToolFilesInEachActionAreNotCopies(t *testing.T) {
	ts0 := "{\n\t\"entry\": \"src/index.ts\",\n\t\"outfile\": \"dist/index.js\",\n\t\"target\": \"node\",\n\t\"format\": \"cjs\"\n}\n"
	workspace := "allowBuilds:\n  esbuild: true\n"
	recipes := "[private]\nhelp:\n\t@just --list\n\nbuild:\n\tpnpm install\n\tts0 build\n"
	files := map[string]string{}
	for _, dir := range []string{"cache-cleanup", "cache-upload"} {
		files[dir+"/ts0.json"] = ts0
		files[dir+"/pnpm-workspace.yaml"] = workspace
		files[dir+"/justfile"] = recipes
	}
	assert.Empty(t, checkOnly(gitRepo(t, files), slopfix.IDNearDuplicate))
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

func schemaServer(t *testing.T) string {
	t.Helper()
	files := map[string]string{"/thing.schema.json": objectSchema, "/rule.xsd": xsd}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func jsonDoc(t *testing.T, fields map[string]string) string {
	t.Helper()
	data, err := json.Marshal(fields)
	require.NoError(t, err)
	return string(data)
}

// A remote schema is fetched and enforced. One that does not load is a finding.
func TestARemoteSchemaIsFetched(t *testing.T) {
	base := schemaServer(t)
	located := func(file string) string {
		return `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="` + base + file + `"`
	}
	root := gitRepo(t, map[string]string{
		"good.json": jsonDoc(t, map[string]string{"$schema": base + "/thing.schema.json", "name": "x"}),
		"bad.json":  jsonDoc(t, map[string]string{"$schema": base + "/thing.schema.json", "nmae": "x"}),
		"lost.json": jsonDoc(t, map[string]string{"$schema": base + "/missing.schema.json", "name": "x"}),
		"good.xml":  xmlDoc(`<rule ` + located("/rule.xsd") + ` id="a"/>`),
		"bad.xml":   xmlDoc(`<rule ` + located("/rule.xsd") + `/>`),
		"lost.xml":  xmlDoc(`<rule ` + located("/missing.xsd") + ` id="a"/>`),
	})
	assert.ElementsMatch(t, []string{"bad.json repo/json", "lost.json repo/json"}, pathsOf(checkOnly(root, slopfix.IDJSON)))
	assert.ElementsMatch(t, []string{"bad.xml repo/xml", "lost.xml repo/xml"}, pathsOf(checkOnly(root, slopfix.IDXML)))
}

// A file named *.invalid.xml is a negative fixture. Its schema must reject it.
func TestANegativeFixtureMustBreakItsSchema(t *testing.T) {
	located := `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="rule.xsd"`
	root := gitRepo(t, map[string]string{
		"rule.xsd":               xsd,
		"missing-id.invalid.xml": xmlDoc(`<rule ` + located + `/>`),
		"passes.invalid.xml":     xmlDoc(`<rule ` + located + ` id="a"/>`),
		"unnamed.invalid.xml":    xmlDoc(`<rule/>`),
		"lost.invalid.xml":       xmlDoc(`<rule xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="missing.xsd"/>`),
	})
	findings := checkOnly(root, slopfix.IDXML)
	assert.ElementsMatch(t, []string{"passes.invalid.xml repo/xml", "unnamed.invalid.xml repo/xml", "lost.invalid.xml repo/xml"}, pathsOf(findings))
	for _, f := range findings {
		if f.Path == "passes.invalid.xml" {
			assert.Contains(t, f.Rule, "passes the schema it names")
		}
	}
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

const namespacedXSD = `<?xml version="1.1" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:rule" elementFormDefault="qualified">
  <xs:element name="rule">
    <xs:complexType>
      <xs:attribute name="id" type="xs:string" use="required"/>
    </xs:complexType>
  </xs:element>
</xs:schema>
`

// The hint is read from the parsed root, as XSD defines it: the
// xsi:schemaLocation pair for the root's own namespace, with xsi matched by
// URI. Text that only looks like a hint is not one.
func TestANamespacedSchemaHintIsReadFromTheParsedRoot(t *testing.T) {
	const xsi = `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`
	located := `xmlns="urn:rule" ` + xsi + ` xsi:schemaLocation="urn:rule rule.xsd"`
	root := gitRepo(t, map[string]string{
		"rule.xsd":   namespacedXSD,
		"good.xml":   xmlDoc(`<rule ` + located + ` id="a"/>`),
		"split.xml":  xmlDoc("<rule xmlns=\"urn:rule\"\n      " + xsi + "\n      xsi:schemaLocation=\"urn:rule rule.xsd\"\n      id=\"a\">\n</rule>"),
		"last.xml":   xmlDoc(`<rule id="a" ` + located + `/>`),
		"second.xml": xmlDoc(`<rule xmlns="urn:rule" ` + xsi + ` xsi:schemaLocation="urn:other missing.xsd  urn:rule rule.xsd" id="a"/>`),
		"prefix.xml": xmlDoc(`<rule xmlns="urn:rule" xmlns:i="http://www.w3.org/2001/XMLSchema-instance" i:schemaLocation="urn:rule rule.xsd" id="a"/>`),
		"bad.xml":    xmlDoc(`<rule ` + located + `/>`),
		"comment.xml": xmlDoc(`<!-- xsi:schemaLocation="urn:rule rule.xsd" -->` +
			`<rule xmlns="urn:rule" id="a"/>`),
		"foreign.xml":   xmlDoc(`<rule xmlns="urn:rule" xmlns:o="urn:other" o:schemaLocation="urn:rule rule.xsd" id="a"/>`),
		"odd.xml":       xmlDoc(`<rule xmlns="urn:rule" ` + xsi + ` xsi:schemaLocation="urn:rule" id="a"/>`),
		"elsewhere.xml": xmlDoc(`<rule xmlns="urn:rule" ` + xsi + ` xsi:schemaLocation="urn:other rule.xsd" id="a"/>`),
	})
	assert.ElementsMatch(t, []string{
		"bad.xml repo/xml", "comment.xml repo/xml", "foreign.xml repo/xml", "odd.xml repo/xml", "elsewhere.xml repo/xml",
	}, pathsOf(checkOnly(root, slopfix.IDXML)))
}

func TestXMLIsHeldToTheSchemaItNames(t *testing.T) {
	located := `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="rule.xsd"`
	root := gitRepo(t, map[string]string{
		"rule.xsd":      xsd,
		"good.xml":      xmlDoc(`<rule ` + located + ` id="a"/>`),
		"bad.xml":       xmlDoc(`<rule ` + located + `/>`),
		"unnamed.xml":   xmlDoc(`<rule id="a"/>`),
		"v10.xml":       `<?xml version="1.0"?><rule ` + located + ` id="a"/>`,
		"nodecl.xml":    `<rule ` + located + ` id="a"/>`,
		"version.xml":   `<?xml version="2.0"?><rule ` + located + ` id="a"/>`,
		"malformed.xml": xmlDoc(`<rule ` + located + ` id="a">`),
	})
	assert.ElementsMatch(t, []string{
		"bad.xml repo/xml", "unnamed.xml repo/xml", "version.xml repo/xml", "malformed.xml repo/xml",
	}, pathsOf(checkOnly(root, slopfix.IDXML)))
}

// A *.invalid.xml file is a negative fixture: it must name its schema and break it.
func TestANegativeFixtureMustBreakTheSchemaItNames(t *testing.T) {
	located := `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="rule.xsd"`
	root := gitRepo(t, map[string]string{
		"rule.xsd":               xsd,
		"rejected.invalid.xml":   xmlDoc(`<rule ` + located + `/>`),
		"accepted.invalid.xml":   xmlDoc(`<rule ` + located + ` id="a"/>`),
		"unnamed.invalid.xml":    xmlDoc(`<rule/>`),
		"lostschema.invalid.xml": xmlDoc(`<rule xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="missing.xsd"/>`),
		"malformed.invalid.xml":  xmlDoc(`<rule ` + located + `>`),
	})
	assert.ElementsMatch(t, []string{
		"accepted.invalid.xml repo/xml", "unnamed.invalid.xml repo/xml",
		"lostschema.invalid.xml repo/xml", "malformed.invalid.xml repo/xml",
	}, pathsOf(checkOnly(root, slopfix.IDXML)), "a schema that rejects the fixture is the only pass")
}

// The rule fails a document on its own: naming another rule leaves it out.
func TestNamingAnotherRuleLeavesTheDocumentRulesOut(t *testing.T) {
	root := gitRepo(t, map[string]string{"broken.json": "{", "tool": elf})
	assert.Equal(t, []string{"tool repo/binary"}, pathsOf(checkOnly(root, slopfix.IDBinary)))
}
