package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// release and revision are what go-toolchain stamped into cmd/slopfix's releaseNumber and gitHash.
var release, revision string

var (
	errNoRelease  = errors.New("this slopfix binary carries no release number: go-toolchain stamps it into every CI build through cmd/slopfix's releaseNumber")
	errNoRevision = errors.New("this slopfix binary carries no commit stamp: go-toolchain stamps it through cmd/slopfix's gitHash")
)

// SetBuild records the stamped release number and commit before Execute runs.
func SetBuild(releaseNumber, gitHash string) { release, revision = releaseNumber, gitHash }

func printVersion(cmd *cobra.Command) error {
	if release == "" {
		return errNoRelease
	}
	if revision == "" {
		return errNoRevision
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "slopfix %s (%s)\n", release, revision)
	return err
}

func init() {
	rootCmd.Flags().Bool("version", false, "print the buildhost release number and the commit")
	rootCmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if on, _ := cmd.Flags().GetBool("version"); on {
			return printVersion(cmd)
		}
		return cmd.Help()
	}
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the buildhost release number and the commit",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return printVersion(cmd)
		},
	})
}
