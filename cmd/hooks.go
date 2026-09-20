// hooks.go registers the subcommands a Claude Code hook launcher execs. Each
// subcommand reads its payload on stdin and writes the hook's own response on
// stdout.
//
// They share a shape, so they share a runner. A hook that refuses does it with
// an exit code, and the launcher passes that through: the marketplace plugin is
// then a manifest and a launcher, with no rule of its own.
package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfix/askproperly"
	"github.com/wow-look-at-my/slopfix/blamelanguage"
	"github.com/wow-look-at-my/slopfix/laziness"
	"github.com/wow-look-at-my/slopfix/linkrefs"
	"github.com/wow-look-at-my/slopfix/mdbudget"
)

// hookResult is what every hook package answers with.
type hookResult struct {
	Stdout string
	Stderr string
	Code   int
}

// register adds a hook subcommand that pipes stdin through run.
func register(use, short, long string, run func(io.Reader) hookResult) {
	c := &cobra.Command{
		Use:           use,
		Short:         short,
		Long:          long,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res := run(cmd.InOrStdin())
			if res.Stdout != "" {
				fmt.Fprint(cmd.OutOrStdout(), res.Stdout)
			}
			if res.Stderr != "" {
				fmt.Fprint(cmd.ErrOrStderr(), res.Stderr)
			}
			if res.Code != 0 {
				os.Exit(res.Code)
			}
			return nil
		},
	}
	rootCmd.AddCommand(c)
}

func init() {
	register("ask-properly",
		"Add a line to a message that hands the reader a decision in prose",
		"ask-properly reads a MessageDisplay payload and appends one line to what\n"+
			"the reader sees, when the message closes by putting a decision to them in\n"+
			"prose. It refuses nothing: a refusal cannot unsend a message that has\n"+
			"already streamed, and the retype carries the same question again.",
		func(r io.Reader) hookResult {
			res := askproperly.Run(r)
			return hookResult(res)
		})

	register("md-budget",
		"Report an instruction file that is over its character budget",
		"md-budget reports at session start, again the moment such a file is\n"+
			"written, and blocks a Stop that leaves one broken. Every CLAUDE.md and\n"+
			"every imported snippet is inlined into the prompt on every request, and\n"+
			"nothing truncates them.",
		func(r io.Reader) hookResult {
			res := mdbudget.Run(r)
			return hookResult(res)
		})

	register("link-refs",
		"Render a pull request, a commit or a branch as a markdown link",
		"link-refs rewrites the message as it streams and sends nothing back to the\n"+
			"model. A reference whose target it cannot show to exist is left as plain\n"+
			"text: a link is a demand to move your hand, and the reader pays it before\n"+
			"knowing whether it was worth paying.",
		func(r io.Reader) hookResult {
			res := linkrefs.Run(r)
			return hookResult(res)
		})

	register("laziness",
		"Refuse a turn that reports a defect it left alone",
		"laziness judges the turn's closing message on Stop and sends the model\n"+
			"back to the defect it named. Stop is the event because this rule is for\n"+
			"the model: a MessageDisplay annotation reaches only the reader, who is\n"+
			"not the one who can go and fix it.\n\n"+
			"A payload that carries no message is read out of the transcript, so a\n"+
			"turn is never left unjudged by a field that is simply absent.",
		func(r io.Reader) hookResult {
			res := laziness.Run(r)
			return hookResult(res)
		})

	register("blame-language",
		"Add a line to a message that hands its own defect to another author",
		"blame-language annotates the finished message and sends nothing back to\n"+
			"the model. MessageDisplay is the event because a Stop refusal on wording\n"+
			"only buys a retype of the same claim.\n\n"+
			"The message arrives in flushes and a phrase can straddle two, so the\n"+
			"text is accumulated and judged whole.",
		func(r io.Reader) hookResult {
			res := blamelanguage.Run(r)
			return hookResult(res)
		})
}
