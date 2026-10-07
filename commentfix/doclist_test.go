package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// forkArgsDoc is a Rust doc from a real tree: paragraphs, then a list of errors.
const forkArgsDoc = "/// Parse the raw argument string after `/fork`.\n" +
	"///\n" +
	"/// Recognised flags appear at the start; everything after the last flag\n" +
	"/// is the directive. Unknown flags are deliberately treated as the\n" +
	"/// start of the directive (so `/fork --foo bar` becomes a directive\n" +
	"/// `--foo bar`) -- the parser is conservative because the args are\n" +
	"/// user-typed text and we do not want to reject directives that happen\n" +
	"/// to begin with `--`.\n" +
	"///\n" +
	"/// Errors:\n" +
	"/// - `--worktree` and `--no-worktree` cannot both appear.\n" +
	"/// - a flag cannot be repeated.\n" +
	"/// - `--at <turn>` returns a friendly \"not supported in this version\"\n" +
	"///   message: the shell already supports the underlying parameter (see\n" +
	"///   `xai_grok_shell::session::fork::ForkSessionRequest::target_prompt_index`)\n" +
	"///   and a turn-picker UI is planned; this version deliberately rejects\n" +
	"///   the flag so users discover the deferral cleanly.\n" +
	"pub fn parse_fork_args(args: &str) -> Result<ForkArgs, String> {\n" +
	"    let rest = args.trim_start();\n" +
	"    Ok(ForkArgs::new(rest))\n" +
	"}\n"

// stepComment is a numbered step that wraps, above the line it documents.
const stepComment = "fn config() {\n" +
	"    // 2) Every axis overridden -> the struct round-ends and the emitted rules\n" +
	"    //    on this platform reflect a read-only grok home / writable system base.\n" +
	"    let over = \"[jail]\\ncwd = \\\"ro\\\"\\ngrok_home = \\\"ro\\\"\\ntmp = \\\"rw\\\"\\nsystem = \\\"rw\\\"\\n\";\n" +
	"}\n"

// A step that opens on "2)" and wraps is prose, not a list, so the length repair still fixes it.
func TestALengthRepairFixesAWrappedNumberedStep(t *testing.T) {
	fixed, _ := FixLength("x.rs", stepComment)
	assert.Empty(t, CheckLength("x.rs", fixed), "the step comment is still too long:\n%s", fixed)
}

// A repair keeps each list item on a line of its own. Joined into the prose
// around it, the doc only looks shorter.
func TestALengthRepairKeepsListItems(t *testing.T) {
	fixed, _ := FixLength("x.rs", forkArgsDoc)
	for _, line := range strings.Split(fixed, "\n") {
		prose := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "///"))
		if strings.HasPrefix(prose, "- ") {
			continue
		}
		assert.NotContains(t, prose, " - `", "a list item was joined onto other prose: %q", line)
		assert.NotContains(t, prose, ". - ", "a list item was joined onto other prose: %q", line)
	}
}
