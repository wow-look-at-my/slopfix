package tombstones

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/treecomments"
)

func TestZZDebugCaseTypo(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package p\n\nfunc readInput() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "os_cosmo_nt_msg.go")
	content := "package runtime\n\n" +
		"// ntSCMParse walks a Linux cmsg buffer. NtSCMParse collects the SCM_RIGHTS\n" +
		"// payload fds, with the kernel's exact acceptance rules.\n" +
		"func ntSCMParse() {}\n"
	blocks := AddedBlocks(path, content)
	t.Logf("blocks: %d", len(blocks))
	for i, b := range blocks {
		t.Logf("block %d: LineNos=%v Pure=%v Prose=%q", i, b.LineNos, b.Pure, b.Prose)
	}
	dead := DeadReferents(path, content, blocks)
	t.Logf("dead: %q", dead)
	for _, name := range dead {
		h := HitForName(blocks, name)
		t.Logf("hit %q: LineNo=%d Strippable=%v Line=%q", name, h.LineNo, h.Strippable, h.Line)
	}
	gone := map[string]bool{}
	for _, d := range dead {
		gone[d] = true
	}
	for _, w := range identifierWords(content) {
		if strings.EqualFold(w, "NtSCMParse") {
			t.Logf("identifierWord %q dead=%v", w, gone[w])
		}
	}
	edits := renameEdits(path, content, blocks, dead)
	t.Logf("renameEdits: %d", len(edits))
	for _, e := range edits {
		t.Logf("edit start=%d end=%d text=%q cut=%q", e.Start, e.End, e.Text, e.Cut)
	}
	res := treecomments.Apply(path, content, edits, edit.Scope{})
	t.Logf("gate applied=%d", len(res.Applied))
	for _, r := range res.Refused {
		t.Logf("gate refused: %q", r.Reason)
	}
	t.Logf("gate text=%q", res.Text)
}
