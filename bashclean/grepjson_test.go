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

var kindNames = map[string]jsonKind{
	"none": notJSON, "document": jsonDoc, "lines": jsonLines, "one-line": jsonOneLine,
}

// lines is the output a test compares: each line trimmed, blank lines dropped.
func lines(s string) []string {
	out := []string{}
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// grepFixtures writes the fixtures grepjson.xml declares, and checks the
// sniff verdict each states.
func grepFixtures(t *testing.T, doc xmlGrepJSON) string {
	t.Helper()
	dir := t.TempDir()
	require.NotEmpty(t, doc.Fixtures, "grepjson.xml declares no fixtures")
	for _, f := range doc.Fixtures {
		path := filepath.Join(dir, f.Name)
		require.NoError(t, os.WriteFile(path, []byte(strings.TrimSpace(f.Body)+"\n"), 0o644))
		want, ok := kindNames[f.Kind]
		require.True(t, ok, "fixture %s: unknown kind %q", f.Name, f.Kind)
		assert.Equal(t, want, sniffJSON(path), "the sniff verdict for fixture %s", f.Name)
	}
	return dir
}

// runGrepTest drives a command through the rewrite, then runs the result
// through bash and the real jq.
func runGrepTest(t *testing.T, dir string, tc xmlGrepTest) {
	t.Helper()
	res := TransformIn(tc.Cmd, dir)
	if tc.Keep {
		assert.NotContains(t, res.Rules, "grep_json", "wrongly rewritten: %s", res.Command)
		assert.Empty(t, lines(tc.Output), "a keep test states no output")
		return
	}
	require.Contains(t, res.Rules, "grep_json", "not rewritten: %s", res.Command)
	_, err := exec.LookPath("jq")
	require.NoError(t, err, "the tests run the rewritten command, and it needs jq on PATH")
	run := exec.Command("bash", "-c", res.Command)
	run.Dir = dir
	out, err := run.CombinedOutput()
	require.NoError(t, err, "the rewrite failed to run: %s\n%s", res.Command, out)
	assert.Equal(t, lines(tc.Output), lines(string(out)), "ran: %s", res.Command)
}

// The cases live in grepjson.xml, beside the flag or program each covers.
func TestTheGrepJSONTableTests(t *testing.T) {
	doc, err := parseGrepJSONXML(grepJSONXML)
	require.NoError(t, err)
	dir := grepFixtures(t, doc)

	tests := append([]xmlGrepTest{}, doc.Tests...)
	for _, set := range doc.Flags {
		for _, f := range set.Flags {
			assert.NotEmpty(t, f.Tests, "flag set %s: flag %s%s has no test", set.Name, f.Short, f.Long)
			tests = append(tests, f.Tests...)
		}
	}
	for _, p := range doc.Programs {
		assert.NotEmpty(t, p.Tests, "program %s has no test", p.Name)
		tests = append(tests, p.Tests...)
	}
	for _, tc := range tests {
		t.Run(tc.Cmd, func(t *testing.T) { runGrepTest(t, dir, tc) })
	}

	require.NotEmpty(t, doc.Translations, "grepjson.xml declares no translations")
	for _, tr := range doc.Translations {
		got, ok := translatePattern(syntaxNames[tr.Syntax], tr.From)
		if tr.To == nil {
			assert.False(t, ok, "%s %q must be refused, got %q", tr.Syntax, tr.From, got)
			continue
		}
		assert.True(t, ok, "%s %q was refused", tr.Syntax, tr.From)
		assert.Equal(t, *tr.To, got, "%s %q", tr.Syntax, tr.From)
	}
}

func TestAMalformedGrepTableFailsToLoad(t *testing.T) {
	for name, doc := range map[string]string{
		"unknown effect": `<grepJSON><flags name="a"><flag short="i" effect="shout"/></flags></grepJSON>`,
		"no spelling":    `<grepJSON><flags name="a"><flag effect="none"/></flags></grepJSON>`,
		"missing set":    `<grepJSON><program name="grep" flags="nope" syntax="basic"/></grepJSON>`,
		"unknown syntax": `<grepJSON><flags name="a"><flag short="i" effect="none"/></flags><program name="grep" flags="a" syntax="glob"/></grepJSON>`,
	} {
		_, err := loadGrepPrograms([]byte(doc))
		assert.Error(t, err, name)
	}
}

func TestTheHookResolvesARelativePathAgainstThePayloadCwd(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.json"), []byte("{\"version\": \"1\"}\n"), 0o644))
	payload, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse", "tool_name": "Bash", "cwd": dir,
		"tool_input": map[string]string{"command": "grep version data.json"},
	})
	require.NoError(t, err)
	out := Run(strings.NewReader(string(payload)))
	assert.Contains(t, out.Stdout, `jq -n -r --arg re`)
}

// A fixture in grepjson.xml is a file with content.
func TestTheSnifferAnswersNoneForWhatIsNotAFileOfJSON(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.json")
	require.NoError(t, os.WriteFile(empty, nil, 0o644))
	for _, path := range []string{dir, empty, filepath.Join(dir, "absent.json")} {
		assert.Equal(t, notJSON, sniffJSON(path), path)
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

	big := filepath.Join(dir, "big.jsonl")
	require.NoError(t, os.WriteFile(big,
		[]byte(strings.Repeat(`{"level":"info","msg":"a line of a log","n":12345}`+"\n", 200000)), 0o644))

	for path, want := range map[string]jsonKind{huge: jsonDoc, big: jsonLines} {
		start := time.Now()
		got := sniffJSON(path)
		elapsed := time.Since(start)
		assert.Equal(t, want, got, path)
		assert.Less(t, elapsed, 50*time.Millisecond, "sniffing %s took %s", path, elapsed)
	}
}
