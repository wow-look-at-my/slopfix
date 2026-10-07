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
