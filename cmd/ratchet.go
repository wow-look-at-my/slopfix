package cmd

import (
	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/slopfix/ratchet"
)

// ratchet.go holds the command. A repository's .github/ratchet names, so a
// run that judges a branch reuses the ratchet package rather than a copy of
// it.
func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "ratchet [branch-checkout]",
		Short: "Judge a branch by the default branch's ratchet",
		Long: "ratchet runs the command the default branch's .github/ratchet names,\n" +
			"from a checkout of that branch, with the branch's checkout as its last\n" +
			"argument. A non-zero exit fails. The command and everything it reads\n" +
			"come from the default branch, so the branch cannot change what judges\n" +
			"it. Outside CI, or outside a repository, it does nothing. The ci entry\n" +
			"point calls it after a build.",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return ratchet.Check()
		},
	})
}
