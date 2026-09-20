// command.go serves the guards that judge a command rather than a file. They
// share a single subcommand because they share a single payload: each reads
// the same bytes and most of them have nothing to say about any given call.
package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfix/autoallow"
	"github.com/wow-look-at-my/slopfix/bashclean"
	"github.com/wow-look-at-my/slopfix/busypoll"
	"github.com/wow-look-at-my/slopfix/noworkloss"
)

// guard names a single command rule for the error a caller reads when it fires.
type guard struct {
	name string
	run  func(io.Reader) hookResult
}

// guards run in this order, and the earliest with anything to say answers. A
// refusal or a rewrite therefore reaches the caller ahead of an approval, so
// auto-allow can only approve a command every other guard passed on.
var guards = []guard{
	{"busy-poll", func(r io.Reader) hookResult { return hookResult(busypoll.Run(r)) }},
	{"no-work-loss", func(r io.Reader) hookResult { return hookResult(noworkloss.Run(r)) }},
	{"clean-bash", func(r io.Reader) hookResult { return hookResult(bashclean.Run(r)) }},
	{"auto-allow", func(r io.Reader) hookResult { return hookResult(autoallow.Run(r)) }},
}

// judgeCommand answers the earliest guard that produced a response or a
// refusal, and names it on stderr so a fired rule is traceable to its package.
func judgeCommand(payload []byte) hookResult {
	for _, g := range guards {
		res := g.run(bytes.NewReader(payload))
		if res.Stdout == "" && res.Stderr == "" && res.Code == 0 {
			continue
		}
		if res.Stderr != "" {
			res.Stderr = fmt.Sprintf("slopfix command %s: %s", g.name, res.Stderr)
		}
		return res
	}
	return hookResult{}
}

func init() {
	parent := &cobra.Command{
		Use:   "command",
		Short: "Judge a command a hook is about to run",
	}

	check := &cobra.Command{
		Use:   "check",
		Short: "Judge the command on stdin against every command rule",
		Long: "check reads a hook payload and runs the busy-poll, no-work-loss,\n" +
			"clean-bash and auto-allow rules over it. Each reads the event off the\n" +
			"payload, so one invocation serves PreToolUse, PermissionRequest and Stop.\n\n" +
			"The rules run in order and the first with a response answers, which puts\n" +
			"every refusal and every rewrite ahead of an approval.",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			payload, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("reading the hook payload: %w", err)
			}
			res := judgeCommand(payload)
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

	parent.AddCommand(check)
	rootCmd.AddCommand(parent)
}
