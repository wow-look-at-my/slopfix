package slopfix_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

func TestDiagTmp(t *testing.T) {
	path := deadReferentRepo(t, "os_cosmo_nt_msg.go")
	a := "package runtime\n\n" +
		"// ntSCMParse walks a Linux cmsg buffer. NtSCMParse collects the SCM_RIGHTS\n" +
		"// payload fds, with the kernel's exact acceptance rules.\n" +
		"func ntSCMParse() {}\n"
	b := "package demo\n\n" +
		"// foreignJobs answers each job of content that runs on a foreign hosted label\n" +
		"// in a private repository, and that the repair can keep to a public one.\n" +
		"func foreignJobs() {\n" + strings.Repeat("\tstep()\n", 20) + "}\n\nfunc step() {}\n"
	var msg []string
	for _, c := range []struct{ path, src string }{{path, a}, {"demo.go", b}} {
		r := slopfix.Fix(slopfix.Request{Path: c.path, Content: c.src, MaxCommentLines: tombstones.DefaultMaxCommentLines})
		msg = append(msg, fmt.Sprintf("TEXT=%q REMOVED=%q KEPT=%v FINDINGS=%v REFUSED=%q", r.Text, r.Removed, r.Kept, r.Findings, r.Refused))
		r2 := slopfix.Fix(slopfix.Request{Path: c.path, Content: c.src, Rules: []slopfix.Rule{slopfix.RuleSTE, slopfix.RuleComments}})
		msg = append(msg, fmt.Sprintf("STEONLY=%q", r2.Text))
	}
	assert.Fail(t, "DIAG", strings.Join(msg, "\n"))
}
