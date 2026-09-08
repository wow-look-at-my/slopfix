// hooks.go registers the subcommands a Claude Code hook launcher execs. Each
// one reads its payload on stdin and writes the hook's own response on stdout.
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
	"github.com/wow-look-at-my/slopfix/busypoll"
	"github.com/wow-look-at-my/slopfix/noworkloss"
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
	register("busy-poll",
		"Refuse a repeated call that cannot learn anything",
		"busy-poll serves Stop and PreToolUse from one command. On Stop it refuses\n"+
			"a turn that is the latest of several making the same call, seconds apart.\n"+
			"On PreToolUse it refuses a status read whose subject this session already\n"+
			"watched settle. The event is on the payload.",
		func(r io.Reader) hookResult {
			res := busypoll.Run(r)
			return hookResult(res)
		})

	register("no-work-loss",
		"Refuse a command that loses content or authors a file outside the edit tools",
		"no-work-loss asks two questions of one parsed command. Destruction: would\n"+
			"this destroy content that exists only in the working tree? Provenance:\n"+
			"does this change file content without Write, Edit or NotebookEdit?\n\n"+
			"Where it can, it commits the content at risk before allowing the command,\n"+
			"and says where the commit went.",
		func(r io.Reader) hookResult {
			res := noworkloss.Run(r)
			return hookResult(res)
		})

	register("ask-properly",
		"Mark a message that hands the reader a decision in prose",
		"ask-properly reads a MessageDisplay payload and appends one line to what\n"+
			"the reader sees, when the message closes by putting a decision to them in\n"+
			"prose. It refuses nothing: a refusal cannot unsend a message that has\n"+
			"already streamed, and the retype carries the same question again.",
		func(r io.Reader) hookResult {
			res := askproperly.Run(r)
			return hookResult(res)
		})
}
