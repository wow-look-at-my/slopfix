package bashclean

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const dataJSON = `{
  "name": "demo",
  "version": "1.2.0",
  "scripts": {"build": "go build", "test": "go test"},
  "tags": ["alpha", "beta"],
  "weird key": "x.y",
  "quote": "say \"hi\"",
  "note": "it's here",
  "enabled": false,
  "nothing": null
}
`

const logsJSONL = `{"level":"info","msg":"started","n":1}
{"level":"error","msg":"say \"hi\" failed","n":2}
{"level":"ERROR","msg":"disk full","n":3}
`

func fixtureDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"data.json":     dataJSON,
		"logs.jsonl":    logsJSONL,
		"more.jsonl":    `{"level":"error","msg":"second file"}` + "\n",
		"notes.txt":     "error: plain text\n",
		"bracketed.log": "[INFO] started\n[ERROR] failed\n",
		"misnamed.txt":  dataJSON,
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
	}
	return dir
}

// runRewrite rewrites the command and runs the result through bash and the
// real jq, so each case proves the output and not the spelling.
func runRewrite(t *testing.T, dir, command string) (Result, string) {
	t.Helper()
	_, err := exec.LookPath("jq")
	require.NoError(t, err, "these tests run the rewritten command, and it needs jq on PATH")
	res := TransformIn(command, dir)
	run := exec.Command("bash", "-c", res.Command)
	run.Dir = dir
	out, err := run.CombinedOutput()
	require.NoError(t, err, "the rewrite failed to run: %s\n%s", res.Command, out)
	return res, string(out)
}

func TestAGrepOverJSONLinesPrintsTheMatchingRecords(t *testing.T) {
	dir := fixtureDir(t)
	rec2 := `{"level":"error","msg":"say \"hi\" failed","n":2}`
	rec3 := `{"level":"ERROR","msg":"disk full","n":3}`
	rec1 := `{"level":"info","msg":"started","n":1}`
	more := `{"level":"error","msg":"second file"}`
	for command, want := range map[string]string{
		"grep error logs.jsonl":               rec2,
		"grep -i error logs.jsonl":            rec2 + "\n" + rec3,
		"grep --ignore-case error logs.jsonl": rec2 + "\n" + rec3,
		"grep -n ERROR logs.jsonl":            "3:" + rec3,
		"grep -ci error logs.jsonl":           "2",
		"grep -vi error logs.jsonl":           rec1,
		`grep 'say "hi"' logs.jsonl`:          rec2,
		"grep -e full -e started logs.jsonl":  rec1 + "\n" + rec3,
		"grep -h error logs.jsonl more.jsonl": rec2 + "\n" + more,
		"grep error logs.jsonl more.jsonl":    "logs.jsonl:" + rec2 + "\nmore.jsonl:" + more,
		"grep error *.jsonl":                  "logs.jsonl:" + rec2 + "\nmore.jsonl:" + more,
		"grep -x 3 logs.jsonl":                rec3,
		"grep -w dis logs.jsonl":              "",
		"rg -i error logs.jsonl":              rec2 + "\n" + rec3,
		"rg -S ERROR logs.jsonl":              rec3,
		"rg -S error logs.jsonl":              rec2 + "\n" + rec3,
		"true && grep error logs.jsonl":       rec2,
	} {
		res, out := runRewrite(t, dir, command)
		assert.Contains(t, res.Rules, "grep_json", "not rewritten: %s", command)
		assert.Equal(t, strings.TrimSpace(want), strings.TrimSpace(out), "wrong output: %s\nran: %s", command, res.Command)
	}
}

func TestAGrepOverAJSONDocumentPrintsThePathOfEachMatchingLeaf(t *testing.T) {
	dir := fixtureDir(t)
	for command, want := range map[string]string{
		"grep version data.json":             `.version = "1.2.0"`,
		"grep scripts data.json":             ".scripts.build = \"go build\"\n.scripts.test = \"go test\"",
		"grep -w go data.json":               ".scripts.build = \"go build\"\n.scripts.test = \"go test\"",
		"grep -F x.y data.json":              `.["weird key"] = "x.y"`,
		"grep alpha data.json":               `.tags[0] = "alpha"`,
		"grep -x false data.json":            `.enabled = false`,
		`grep 'alp\|bet' data.json`:          ".tags[0] = \"alpha\"\n.tags[1] = \"beta\"",
		"egrep 'alp|bet' data.json":          ".tags[0] = \"alpha\"\n.tags[1] = \"beta\"",
		"grep -E '^(alpha|beta)$' data.json": ".tags[0] = \"alpha\"\n.tags[1] = \"beta\"",
		`grep 'say "hi"' data.json`:          `.quote = "say \"hi\""`,
		`grep "it's" data.json`:              `.note = "it's here"`,
		"grep null data.json":                `.nothing = null`,
		"grep -c go data.json":               "2",
		"grep version < data.json":           `.version = "1.2.0"`,
		"grep -H version data.json":          `data.json:.version = "1.2.0"`,
		"grep version misnamed.txt":          `.version = "1.2.0"`,
		"grep -v o data.json":                ".tags[0] = \"alpha\"\n.tags[1] = \"beta\"\n.[\"weird key\"] = \"x.y\"\n.enabled = false",
	} {
		res, out := runRewrite(t, dir, command)
		assert.Contains(t, res.Rules, "grep_json", "not rewritten: %s", command)
		assert.Equal(t, want, strings.TrimSpace(out), "wrong output: %s\nran: %s", command, res.Command)
	}
}

// Each of these is not a grep over JSON, or asks for something the jq query
// does not reproduce, so the command stays as written.
func TestAGrepTheRuleCannotTranslateStaysAsWritten(t *testing.T) {
	dir := fixtureDir(t)
	for _, command := range []string{
		"grep error notes.txt",
		"grep ERROR bracketed.log",
		"grep error data.json notes.txt",
		"grep error logs.jsonl data.json",
		"grep -A2 version data.json",
		"grep -l version data.json",
		"grep -o version data.json",
		"grep -r version .",
		"grep -c error logs.jsonl more.jsonl",
		`grep "$pattern" data.json`,
		"grep error missing.json",
		"grep error *.nothing",
		"xargs grep error logs.jsonl",
		"some-command | grep error",
		"rg -h error logs.jsonl",
		"rg -q error logs.jsonl",
	} {
		res := TransformIn(command, dir)
		assert.NotContains(t, res.Rules, "grep_json", "wrongly rewritten: %s -> %s", command, res.Command)
	}
}

func TestAWrapperBeforeGrepSurvivesTheRewrite(t *testing.T) {
	dir := fixtureDir(t)
	res := TransformIn("timeout 5 grep version data.json", dir)
	assert.True(t, strings.HasPrefix(res.Command, "set -o pipefail\ntimeout 5 jq -n -r"), res.Command)
}

func TestTheHookResolvesARelativePathAgainstThePayloadCwd(t *testing.T) {
	dir := fixtureDir(t)
	payload, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse", "tool_name": "Bash", "cwd": dir,
		"tool_input": map[string]string{"command": "grep version data.json"},
	})
	require.NoError(t, err)
	out := Run(strings.NewReader(string(payload)))
	assert.Contains(t, out.Stdout, `jq -n -r --arg re`)
	assert.Contains(t, out.Stdout, `data.json`)
}

func TestPosixPatternsTranslateForOniguruma(t *testing.T) {
	for in, want := range map[string]string{
		`a\|b`:        `a|b`,
		`\(ab\)\{2\}`: `(ab){2}`,
		`a+b?`:        `a\+b\?`,
		`(x)`:         `\(x\)`,
		`*star`:       `\*star`,
		`a^b$c`:       `a\^b\$c`,
		`^anchored$`:  `^anchored$`,
		`\<word\>`:    `\bword\b`,
		`[]a\[]`:      `[\]a\\\[]`,
		`[[:alpha:]]`: `[[:alpha:]]`,
		`a\.b`:        `a\.b`,
	} {
		got, ok := posixToOnig(in, true)
		assert.True(t, ok, in)
		assert.Equal(t, want, got, "basic: %s", in)
	}
	got, ok := posixToOnig(`(a|b)+`, false)
	assert.True(t, ok)
	assert.Equal(t, `(a|b)+`, got)
	_, ok = posixToOnig(`[unclosed`, false)
	assert.False(t, ok)
}

func TestTheSnifferReadsContentNotTheExtension(t *testing.T) {
	dir := fixtureDir(t)
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
		return p
	}
	for path, want := range map[string]jsonKind{
		filepath.Join(dir, "data.json"):         jsonDoc,
		filepath.Join(dir, "misnamed.txt"):      jsonDoc,
		filepath.Join(dir, "logs.jsonl"):        jsonLines,
		filepath.Join(dir, "notes.txt"):         notJSON,
		filepath.Join(dir, "bracketed.log"):     notJSON,
		write("bom.json", "\xEF\xBB\xBF[1, 2]"): jsonDoc,
		write("empty.json", ""):                 notJSON,
		write("scalar.json", `"just a string"`): notJSON,
		write("unclosed.json", `{"a": [1, 2`):   notJSON,
		write("trailing.json", `{"a": 1} junk`): notJSON,
		dir:                                     notJSON,
		filepath.Join(dir, "absent.json"):       notJSON,
	} {
		assert.Equal(t, want, sniffJSON(path), path)
	}
}

// A FIFO blocks an open until a writer arrives, so the sniffer must never
// open one.
func TestTheSnifferNeverOpensAFIFO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pipe.json")
	require.NoError(t, syscall.Mkfifo(path, 0o644))
	done := make(chan jsonKind, 1)
	go func() { done <- sniffJSON(path) }()
	select {
	case k := <-done:
		assert.Equal(t, notJSON, k)
	case <-time.After(2 * time.Second):
		t.Fatal("the sniffer blocked on a FIFO")
	}
}

// The sniff reads a fixed head and tail.
func TestTheSnifferTakesUnderFiftyMillisecondsAtAnySize(t *testing.T) {
	dir := t.TempDir()
	huge := filepath.Join(dir, "huge.json")
	f, err := os.Create(huge)
	require.NoError(t, err)
	_, err = f.WriteString(`{"items": [`)
	require.NoError(t, err)
	_, err = f.WriteString(strings.Repeat(`{"id": 1, "name": "item"}, `, 4000))
	require.NoError(t, err)
	_, err = f.WriteAt([]byte("{\"id\": 2}]}\n"), 8<<30)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	lines := filepath.Join(dir, "big.jsonl")
	require.NoError(t, os.WriteFile(lines,
		[]byte(strings.Repeat(`{"level":"info","msg":"a line of a log","n":12345}`+"\n", 200000)), 0o644))

	for path, want := range map[string]jsonKind{huge: jsonDoc, lines: jsonLines} {
		start := time.Now()
		got := sniffJSON(path)
		elapsed := time.Since(start)
		assert.Equal(t, want, got, path)
		assert.Less(t, elapsed, 50*time.Millisecond, "sniffing %s took %s", path, elapsed)
	}
}
