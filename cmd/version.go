// buildSettings is the build info go-toolchain stamped the release into.

package cmd

import (
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	errNoRelease  = errors.New("this slopfix binary carries no release number: go-toolchain stamps the build setting release into every CI build")
	errNoRevision = errors.New("this slopfix binary carries no commit stamp: go-toolchain stamps the build setting release.revision beside the release")
)

// buildSettings is the build info go-toolchain stamped the release into.
var buildSettings = func() []debug.BuildSetting {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	return bi.Settings
}

// stampedRelease answers the buildhost release number and its commit.
func stampedRelease() (release, revision string, err error) {
	for _, s := range buildSettings() {
		switch s.Key {
		case "release":
			release = s.Value
		case "release.revision":
			revision = s.Value
		}
	}
	if release == "" {
		return "", "", errNoRelease
	}
	if revision == "" {
		return "", "", errNoRevision
	}
	return release, revision, nil
}

func printVersion(cmd *cobra.Command) error {
	release, revision, err := stampedRelease()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "slopfix %s (%s)\n", release, revision)
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
