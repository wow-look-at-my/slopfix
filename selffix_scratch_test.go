package slopfix

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/goembed"
)

func TestSelfFixScratch(t *testing.T) {
	root := os.Getenv("SELF_FIX_ROOT")
	if root == "" {
		t.Skip()
	}
	out, err := exec.Command("git", "-C", root, "ls-files", "*.go").Output()
	require.NoError(t, err)
	var log strings.Builder
	for _, rel := range strings.Fields(string(out)) {
		path := filepath.Join(root, rel)
		src, err := os.ReadFile(path)
		if err != nil || len(goembed.CheckPath(path, string(src))) == 0 {
			continue
		}
		// A rule case is test data, so it goes under testdata as a test's string does.
		as := path
		if !strings.HasSuffix(path, "_test.go") && strings.HasPrefix(filepath.Base(path), "rule_") {
			as = strings.TrimSuffix(path, ".go") + "_test.go"
		}
		rep := Fix(Request{Content: string(src), Path: as, Rules: []Rule{RuleGo}, IDs: []string{goembed.ID}})
		if !rep.Changed {
			log.WriteString("UNCHANGED " + rel + "\n")
			continue
		}
		for _, c := range rep.Created {
			os.MkdirAll(filepath.Dir(c.Path), 0o755)
			os.WriteFile(c.Path, []byte(c.Text), 0o644)
		}
		os.WriteFile(path, []byte(rep.Text), 0o644)
		log.WriteString("FIXED " + rel + "\n")
	}
	os.WriteFile(os.Getenv("SELF_FIX_LOG"), []byte(log.String()), 0o644)
}
